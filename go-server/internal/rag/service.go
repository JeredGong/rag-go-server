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
	index := strings.Index(llmOutput, model.SepToken)
	if index == -1 {
		return nil, fmt.Errorf("未找到 %s 分隔符", model.SepToken)
	}

	jsonPart := strings.TrimSpace(llmOutput[index+len(model.SepToken):])

	var result []model.CourseRecommendation
	if err := json.Unmarshal([]byte(jsonPart), &result); err != nil {
		return nil, fmt.Errorf("JSON 解析失败: %v", err)
	}
	return result, nil
}
