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

type Service struct {
	Embedder    embedding.Client
	VectorStore vectorstore.Store
	LLM         llm.Client
	Limiter     limit.RateLimiter
}

func NewService(e embedding.Client, vs vectorstore.Store, l llm.Client, limiter limit.RateLimiter) *Service {
	return &Service{
		Embedder:    e,
		VectorStore: vs,
		LLM:         l,
		Limiter:     limiter,
	}
}

// HandleRag 运行完整的 RAG 流程
func (s *Service) HandleRag(ctx context.Context, req model.RagRequest, fingerprint string) ([]model.CourseRecommendation, error) {
	// 1. 限流
	allowed, err := s.Limiter.Allow(ctx, fingerprint)
	if err != nil {
		return nil, fmt.Errorf("访问限制检查失败: %w", err)
	}
	if !allowed {
		return nil, fmt.Errorf("访问次数已用完，请稍后再试")
	}

	// 2. embedding
	embeddingVec, err := s.Embedder.Embed(ctx, req.UserQuestion)
	if err != nil {
		return nil, fmt.Errorf("获取嵌入失败: %w", err)
	}
	log.Println("获取用户问题的嵌入向量成功")

	// 3. Qdrant 检索
	courses, err := s.VectorStore.Search(ctx, embeddingVec, req.Catagory, 100)
	if err != nil {
		return nil, fmt.Errorf("搜索 Qdrant 失败: %w", err)
	}
	log.Printf("找到 %d 个相似课程", len(courses))

	// 4. 调用 LLM
	llmOutput, err := s.LLM.RecommendCourses(ctx, req.UserQuestion, courses)
	if err != nil {
		return nil, fmt.Errorf("调用 LLM 失败: %w", err)
	}
	log.Println("LLM 调用成功，生成回答:", llmOutput)

	// 5. 解析 LLM 输出
	recs, err := ParseLLMOutput(llmOutput)
	if err != nil {
		return nil, fmt.Errorf("解析 LLM 输出失败: %w", err)
	}
	return recs, nil
}

// ParseLLMOutput 从 LLM 输出中截取 <|Result|> 后的 JSON
func ParseLLMOutput(llmOutput string) ([]model.CourseRecommendation, error) {
	// 1. 先找到 <|Result|>
	idx := strings.Index(llmOutput, model.SepToken)
	if idx == -1 {
		return nil, fmt.Errorf("未找到 %s 分隔符", model.SepToken)
	}

	// 2. 取出 <|Result|> 之后的内容
	s := llmOutput[idx+len(model.SepToken):]

	// 3. 找到第一个 '[' 或 '{'（JSON 开头）
	start := strings.IndexAny(s, "[{")
	if start == -1 {
		// 打印一下方便你在日志里看原始输出
		log.Printf("LLM 输出中未找到 JSON 起始符号：[ 或 {，原始输出片段: %s", s)
		return nil, fmt.Errorf("未找到 JSON 起始符号")
	}
	s = s[start:]

	// 4. 可选：裁到最后一个 ']' 或 '}'（防止后面模型再啰嗦）
	end := strings.LastIndexAny(s, "]}")
	if end != -1 {
		s = s[:end+1]
	}

	s = strings.TrimSpace(s)

	// 5. 解析 JSON
	var result []model.CourseRecommendation
	if err := json.Unmarshal([]byte(s), &result); err != nil {
		log.Printf("解析 LLM JSON 片段失败，片段为: %s, 错误: %v", s, err)
		return nil, fmt.Errorf("JSON 解析失败: %v", err)
	}
	return result, nil
}
