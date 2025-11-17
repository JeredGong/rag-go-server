// go-server/internal/llm/deepseek.go
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"rag-go-server/internal/model"
)

// Client 抽象：根据用户问题和课程列表，生成推荐结果（原始字符串）
type Client interface {
	RecommendCourses(ctx context.Context, question string, courses []map[string]interface{}) (string, error)
}

// DeepSeekClient 使用 DeepSeek Chat API
type DeepSeekClient struct {
	APIKey     string
	URL        string
	HTTPClient *http.Client
}

func NewDeepSeekClient(apiKey string) *DeepSeekClient {
	return &DeepSeekClient{
		APIKey:     apiKey,
		URL:        "https://api.deepseek.com/chat/completions",
		HTTPClient: http.DefaultClient,
	}
}

func (d *DeepSeekClient) RecommendCourses(ctx context.Context, question string, courses []map[string]interface{}) (string, error) {
	textList := make([]string, 0, len(courses))
	for _, course := range courses {
		if text, ok := course["text"].(string); ok {
			textList = append(textList, text)
		}
	}

	joined := fmt.Sprintf("课程列表: [\"%s\"]\n用户提问: %s",
		strings.Join(textList, "\", \""), question)

	requestBody := map[string]interface{}{
		"model": "deepseek-chat",
		"messages": []map[string]string{
			{"role": "system", "content": model.SystemPrompt},
			{"role": "user", "content": joined},
		},
		"stream": false,
	}
	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("序列化 LLM 请求失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.URL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("创建 LLM 请求失败: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+d.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("LLM API 调用失败: %w", err)
	}
	defer resp.Body.Close()

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取 LLM 响应体失败: %w", err)
	}
	log.Printf("DeepSeek 响应状态码: %d", resp.StatusCode)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("非成功响应内容: %s", string(respData))
		return "", fmt.Errorf("API 错误，状态码: %d，响应内容: %s", resp.StatusCode, string(respData))
	}

	var out map[string]interface{}
	if err := json.Unmarshal(respData, &out); err != nil {
		log.Printf("无法解析 JSON 响应: %s", string(respData))
		return "", err
	}

	choicesRaw, ok := out["choices"].([]interface{})
	if !ok || len(choicesRaw) == 0 {
		return "", fmt.Errorf("响应中缺少 choices")
	}
	first, ok := choicesRaw[0].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("choices[0] 结构不符合预期")
	}
	msg, ok := first["message"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("choices[0].message 结构不符合预期")
	}
	content, ok := msg["content"].(string)
	if !ok {
		return "", fmt.Errorf("message.content 不是字符串")
	}
	return content, nil
}
