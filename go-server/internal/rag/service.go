// go-server/internal/rag/service.go
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
	Embedder    embedding.Client
	VectorStore vectorstore.Store
	LLM         llm.Client
	Limiter     limit.RateLimiter
}

// NewService 创建一个 RAG 服务实例
func NewService(
	e embedding.Client,
	vs vectorstore.Store,
	l llm.Client,
	limiter limit.RateLimiter,
) *Service {
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

	// --------------------------
	// 1. 限流检查
	// --------------------------
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
	vec, err := s.Embedder.Embed(ctx, req.UserQuestion)
	if err != nil {
		return nil, fmt.Errorf("生成嵌入失败: %w", err)
	}
	log.Println("🔹 用户问题嵌入向量生成完毕")

	// --------------------------
	// 3. 向量检索（Qdrant）
	// --------------------------
	courses, err := s.VectorStore.Search(ctx, vec, req.Catagory, 100)
	if err != nil {
		return nil, fmt.Errorf("Qdrant 搜索失败: %w", err)
	}
	log.Printf("🔹 Qdrant 检索完成，共找到 %d 条候选课程", len(courses))

	// --------------------------
	// 4. 使用 LLM 生成推荐内容
	// --------------------------
	llmResp, err := s.LLM.RecommendCourses(ctx, req.UserQuestion, courses)
	if err != nil {
		return nil, fmt.Errorf("LLM 调用失败: %w", err)
	}
	log.Println("🔹 LLM 已成功返回推荐结果")

	// --------------------------
	// 5. 解析 LLM JSON 输出
	// --------------------------
	recommendations, err := ParseLLMOutput(llmResp)
	if err != nil {
		return nil, fmt.Errorf("解析 LLM 输出失败: %w", err)
	}

	return recommendations, nil
}

// ParseLLMOutput 从 LLM 文本输出中截取 JSON 数组并解析
func ParseLLMOutput(llmOutput string) ([]model.CourseRecommendation, error) {
	// --------------------------
	// 1. 查找分隔符 <|Result|>
	// --------------------------
	pos := strings.Index(llmOutput, model.SepToken)
	if pos == -1 {
		return nil, fmt.Errorf("LLM 输出中未找到分隔符 %s", model.SepToken)
	}

	fragment := llmOutput[pos+len(model.SepToken):]

	// --------------------------
	// 2. 搜索 JSON 起点 '[' 或 '{'
	// --------------------------
	start := strings.IndexAny(fragment, "[{")
	if start == -1 {
		log.Printf("⛔ JSON 起始符号未找到，输出片段：%s", fragment)
		return nil, fmt.Errorf("未找到 JSON 起始符号")
	}
	fragment = fragment[start:]

	// --------------------------
	// 3. 查找 JSON 结束符
	// --------------------------
	end := strings.LastIndexAny(fragment, "]}")
	if end != -1 {
		fragment = fragment[:end+1]
	}

	fragment = strings.TrimSpace(fragment)

	// --------------------------
	// 4. 尝试反序列化
	// --------------------------
	var items []model.CourseRecommendation
	if err := json.Unmarshal([]byte(fragment), &items); err != nil {
		log.Printf("⛔ JSON 解析失败，片段：%s | 错误：%v", fragment, err)
		return nil, fmt.Errorf("JSON 解析失败: %v", err)
	}

	return items, nil
}
