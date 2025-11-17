// go-server/internal/rag/service.go
// 该文件负责 orchestrate RAG 服务逻辑，是最常被业务层调用的入口。
package rag

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"rag-go-server/internal/embedding"
	"rag-go-server/internal/limit"
	"rag-go-server/internal/llm"
	"rag-go-server/internal/model"
	"rag-go-server/internal/vectorstore"
)

// Service 封装完整 RAG 处理链条
type Service struct {
	// Embedder 用于将自然语言转化为稠密向量表示。
	Embedder embedding.Client
	// VectorStore 负责检索与用户问题语义相关的课程条目。
	VectorStore vectorstore.Store
	// LLM 负责在检索结果基础上生成结构化推荐。
	LLM llm.Client
	// Limiter 控制请求速率，保护后端资源。
	Limiter limit.RateLimiter
}

// NewService 创建一个 RAG 服务实例
func NewService(
	e embedding.Client,
	vs vectorstore.Store,
	l llm.Client,
	limiter limit.RateLimiter,
) *Service {
	// 以依赖注入方式组装服务，方便在测试环境替换组件。
	return &Service{
		Embedder:    e,
		VectorStore: vs,
		LLM:         l,
		Limiter:     limiter,
	}
}

// HandleRag 运行完整的 RAG 流程：限流 → 向量化 → 检索 → LLM → 解析
func (s *Service) HandleRag(
	ctx context.Context,
	req model.RagRequest,
	fingerprint string,
) ([]model.CourseRecommendation, error) {
	// 此方法贯穿限流、向量化、检索、生成、解析五个阶段。

	// --------------------------
	// 1. 限流检查
	// --------------------------
	// 根据 fingerprint 判断是否仍有配额，若失败立即返回。
	allowed, err := s.Limiter.Allow(ctx, fingerprint)
	if err != nil {
		return nil, fmt.Errorf("访问限制检查失败: %w", err)
	}
	if !allowed {
		return nil, fmt.Errorf("访问次数已用完，请稍后再试")
	}

	// --------------------------
	// 2. 用户查询 → embedding
	// --------------------------
	// 将用户问题转换为向量以便向量数据库进行语义匹配。
	vec, err := s.Embedder.Embed(ctx, req.UserQuestion)
	if err != nil {
		return nil, fmt.Errorf("生成嵌入失败: %w", err)
	}
	// 记录嵌入完成信息，有助于排查延迟瓶颈。
	log.Println("🔹 用户问题嵌入向量生成完毕")

	// --------------------------
	// 3. 向量检索（Qdrant）
	// --------------------------
	// 从向量数据库中检索 topK 课程，形成候选集合。
	courses, err := s.VectorStore.Search(ctx, vec, req.Catagory, 100)
	if err != nil {
		return nil, fmt.Errorf("Qdrant 搜索失败: %w", err)
	}
	// 记录命中数量，方便监控召回效果。
	log.Printf("🔹 Qdrant 检索完成，共找到 %d 条候选课程", len(courses))

	// --------------------------
	// 4. 使用 LLM 生成推荐内容
	// --------------------------
	// 将用户问题与候选课程传入 LLM 以构造最终推荐。
	llmResp, err := s.LLM.RecommendCourses(ctx, req.UserQuestion, courses)
	if err != nil {
		return nil, fmt.Errorf("LLM 调用失败: %w", err)
	}
	// LLM 走通表示生成环节已完成。
	log.Println("🔹 LLM 已成功返回推荐结果")

	// --------------------------
	// 5. 解析 LLM JSON 输出
	// --------------------------
	// LLM 输出通常含有提示语，需要提取分隔符后的 JSON 段落。
	recommendations, err := ParseLLMOutput(llmResp)
	if err != nil {
		return nil, fmt.Errorf("解析 LLM 输出失败: %w", err)
	}

	// 走到此处说明所有环节已成功完成，可以安全返回推荐结果给调用方。
	return recommendations, nil
}

// ParseLLMOutput 从 LLM 文本输出中截取 JSON 数组并解析
func ParseLLMOutput(llmOutput string) ([]model.CourseRecommendation, error) {
	// 解析策略：定位分隔符 → 提取 JSON 片段 → 反序列化。
	// --------------------------
	// 1. 查找分隔符 <|Result|>
	// --------------------------
	pos := strings.Index(llmOutput, model.SepToken)
	if pos == -1 {
		return nil, fmt.Errorf("LLM 输出中未找到分隔符 %s", model.SepToken)
	}

	fragment := llmOutput[pos+len(model.SepToken):]
	// fragment 仅保留分隔符之后的内容，避免被系统提示词干扰。

	// --------------------------
	// 2. 搜索 JSON 起点 '[' 或 '{'
	// --------------------------
	// LLM 可能在分隔符后仍带有解释文字，因此需要截取首次出现的 JSON 起点。
	start := strings.IndexAny(fragment, "[{")
	if start == -1 {
		log.Printf("⛔ JSON 起始符号未找到，输出片段：%s", fragment)
		return nil, fmt.Errorf("未找到 JSON 起始符号")
	}
	fragment = fragment[start:]
	// 经过截断后，fragment 应该以 '[' 或 '{' 开头，更利于后续定位。

	// --------------------------
	// 3. 查找 JSON 结束符
	// --------------------------
	// 为避免尾部提示词影响解析，尝试找到 JSON 的最后一个闭合符。
	end := strings.LastIndexAny(fragment, "]}")
	if end != -1 {
		fragment = fragment[:end+1]
	}

	// 去掉前后空白字符，降低 JSON 解析失败的概率。
	fragment = strings.TrimSpace(fragment)

	// --------------------------
	// 4. 尝试反序列化
	// --------------------------
	// items 定义为业务层所需的课程推荐结构。
	var items []model.CourseRecommendation
	if err := json.Unmarshal([]byte(fragment), &items); err != nil {
		log.Printf("⛔ JSON 解析失败，片段：%s | 错误：%v", fragment, err)
		return nil, fmt.Errorf("JSON 解析失败: %v", err)
	}

	// 解析成功后返回结构化课程推荐数组供上层使用。
	return items, nil
}
