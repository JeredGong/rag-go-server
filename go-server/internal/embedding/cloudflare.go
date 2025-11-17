// go-server/internal/embedding/cloudflare.go
//
// 向量嵌入服务模块
//
// 本模块封装了文本向量化的功能，将自然语言文本转换为稠密向量表示。
// 这些向量用于后续的相似度检索，是 RAG 系统的核心组件之一。
//
// 架构特点：
// 1. 通过接口抽象，支持多种向量化服务提供商
// 2. 默认实现使用 Cloudflare Worker 部署的 BGE-M3 模型
// 3. 便于在测试环境中使用 mock 对象
// 4. 支持自定义 HTTP 客户端配置（超时、重试等）
//
// 向量化原理：
// BGE-M3 是一个多语言稠密检索模型，可以将文本映射到高维向量空间。
// 语义相似的文本在向量空间中距离较近，从而支持基于相似度的检索。
package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Client 定义文本向量化的通用接口
//
// 通过接口抽象，可以灵活替换不同的向量化服务：
// - Cloudflare Worker + BGE-M3（当前实现）
// - OpenAI Embeddings API
// - 本地部署的向量化模型
// - 测试环境中的 mock 实现
type Client interface {
	// Embed 将文本转换为向量表示
	//
	// 参数：
	//   - ctx: 上下文对象，用于超时控制和请求取消
	//   - text: 待向量化的文本内容
	//
	// 返回值：
	//   - []float32: 文本的向量表示，维度由模型决定（BGE-M3 为 1024 维）
	//   - error: 如果向量化失败，返回错误信息
	Embed(ctx context.Context, text string) ([]float32, error)
}

// CloudflareClient 基于 Cloudflare Worker 的向量化客户端
//
// 该实现调用部署在 Cloudflare Worker 上的向量化服务。
// Cloudflare Worker 的优势：
// 1. 全球边缘节点部署，低延迟
// 2. 自动扩缩容，无需管理服务器
// 3. 免费额度充足，适合中小规模应用
type CloudflareClient struct {
	// Endpoint Cloudflare Worker 的完整 URL
	// 示例：https://whuworkers.jeredgong.workers.dev
	Endpoint string
	
	// HTTPClient 用于发送 HTTP 请求的客户端
	// 可自定义超时、重试等参数，默认使用 http.DefaultClient
	HTTPClient *http.Client
}

// NewCloudflareClient 创建一个新的 Cloudflare 向量化客户端
//
// 参数：
//   - endpoint: Cloudflare Worker 的 URL
//
// 返回值：
//   - *CloudflareClient: 客户端实例
func NewCloudflareClient(endpoint string) *CloudflareClient {
	return &CloudflareClient{
		Endpoint:   endpoint,
		HTTPClient: http.DefaultClient,
	}
}

// Embed 实现 Client 接口，将文本转换为向量
//
// 工作流程：
// 1. 构造 JSON 请求体，包含待向量化的文本
// 2. 向 Cloudflare Worker 发送 POST 请求
// 3. 解析响应中的向量数据
// 4. 将 float64 转换为 float32（Qdrant 使用 float32）
//
// 参数：
//   - ctx: 上下文对象
//   - text: 待向量化的文本
//
// 返回值：
//   - []float32: 向量表示
//   - error: 错误信息
func (c *CloudflareClient) Embed(ctx context.Context, text string) ([]float32, error) {
	// ========================================
	// 阶段1: 构造请求
	// ========================================
	
	// 构造请求体：{"text": "用户的问题..."}
	body := map[string]interface{}{"text": text}
	jsonData, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	// 创建 HTTP POST 请求，携带上下文以支持超时和取消
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	// ========================================
	// 阶段2: 发送请求
	// ========================================
	
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP 请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 检查 HTTP 状态码，非 2xx 视为失败
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("embedding API 返回错误状态码 %d: %s", resp.StatusCode, string(body))
	}

	// ========================================
	// 阶段3: 解析响应
	// ========================================
	
	// Cloudflare Worker 返回的 JSON 结构：
	// {
	//   "embedding": {
	//     "data": [[0.123, 0.456, ...]]
	//   }
	// }
	var result struct {
		Embedding struct {
			Data [][]float64 `json:"data"`
		} `json:"embedding"`
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应体失败: %w", err)
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("解析 embedding 响应失败: %w", err)
	}

	// 验证返回的向量数据是否有效
	if len(result.Embedding.Data) == 0 || len(result.Embedding.Data[0]) == 0 {
		return nil, fmt.Errorf("embedding 数据为空")
	}

	// ========================================
	// 阶段4: 类型转换
	// ========================================
	
	// 将 float64 向量转换为 float32
	// Qdrant 使用 float32 存储向量，可以节省一半的存储空间
	// 精度损失对检索效果影响微乎其微
	vec := make([]float32, len(result.Embedding.Data[0]))
	for i, v := range result.Embedding.Data[0] {
		vec[i] = float32(v)
	}
	
	return vec, nil
}
