// go-server/internal/embedding/cloudflare.go
package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Client 抽象：文本 -> 向量
type Client interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

// CloudflareClient 使用 Cloudflare Worker 提供的 embedding 接口
type CloudflareClient struct {
	Endpoint   string
	HTTPClient *http.Client
}

func NewCloudflareClient(endpoint string) *CloudflareClient {
	return &CloudflareClient{
		Endpoint:   endpoint,
		HTTPClient: http.DefaultClient,
	}
}

func (c *CloudflareClient) Embed(ctx context.Context, text string) ([]float32, error) {
	body := map[string]interface{}{"text": text}
	jsonData, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("embedding API 返回错误状态码 %d: %s", resp.StatusCode, string(body))
	}

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

	if len(result.Embedding.Data) == 0 || len(result.Embedding.Data[0]) == 0 {
		return nil, fmt.Errorf("embedding 数据为空")
	}

	vec := make([]float32, len(result.Embedding.Data[0]))
	for i, v := range result.Embedding.Data[0] {
		vec[i] = float32(v)
	}
	return vec, nil
}
