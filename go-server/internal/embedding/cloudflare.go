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
	jsonData, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Embedding struct {
			Data [][]float64 `json:"data"`
		} `json:"embedding"`
	}

	data, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
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
