// go-server/internal/llm/deepseek.go
//
// 大语言模型客户端模块
//
// 本模块封装了与 DeepSeek 大模型的交互逻辑。
// DeepSeek 是一个高性能的中文大模型，提供与 OpenAI 兼容的 API 接口。
//
// 模块职责：
// 1. 将检索到的课程列表和用户问题组装成 prompt
// 2. 调用 DeepSeek Chat API 生成推荐结果
// 3. 处理 API 响应并提取生成的文本内容
// 4. 支持通过接口抽象，便于替换其他大模型
//
// RAG 流程中的位置：
// 向量检索 → LLM 生成（本模块）→ 结构化解析
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
	"time"

	"rag-go-server/internal/model"
)

// Client 定义大语言模型客户端的通用接口
//
// 通过接口抽象，可以方便地切换不同的大模型：
// - DeepSeek（当前实现）
// - OpenAI GPT-4
// - Claude
// - 本地部署的开源模型
type Client interface {
	// RecommendCourses 根据用户问题和检索到的课程列表，生成推荐结果
	//
	// 参数：
	//   - ctx: 上下文对象，用于超时控制
	//   - question: 用户的自然语言问题
	//   - courses: 向量检索得到的候选课程列表，每个元素包含课程的详细信息
	//
	// 返回值：
	//   - string: 大模型生成的原始文本，包含分析过程和 JSON 格式的推荐结果
	//   - error: 如果 API 调用失败，返回错误信息
	RecommendCourses(ctx context.Context, question string, courses []map[string]interface{}) (string, error)
}

// DeepSeekClient DeepSeek 大模型的客户端实现
//
// DeepSeek 提供与 OpenAI 兼容的 Chat Completions API，
// 支持多轮对话和自定义系统提示词。
type DeepSeekClient struct {
	// APIKey DeepSeek API 的认证密钥
	APIKey string
	
	// URL DeepSeek API 的端点地址
	URL string
	
	// HTTPClient 用于发送 HTTP 请求的客户端
	HTTPClient *http.Client
}

const (
	defaultLLMTimeout      = 60 * time.Second
	maxCourseContextRunes  = 512
)

// NewDeepSeekClient 创建一个新的 DeepSeek 客户端
//
// 参数：
//   - apiKey: DeepSeek API 密钥
//
// 返回值：
//   - *DeepSeekClient: 客户端实例
func NewDeepSeekClient(apiKey string) *DeepSeekClient {
	return &DeepSeekClient{
		APIKey:     apiKey,
		URL:        "https://api.deepseek.com/chat/completions",
		HTTPClient: &http.Client{
			Timeout: defaultLLMTimeout,
		},
	}
}

// RecommendCourses 实现 Client 接口，生成课程推荐
//
// 工作流程：
// 1. 从课程列表中提取文本描述
// 2. 组装用户消息：课程列表 + 用户问题
// 3. 构造 Chat Completions 请求（包含系统提示词和用户消息）
// 4. 调用 DeepSeek API
// 5. 解析响应并提取生成的内容
//
// 参数：
//   - ctx: 上下文对象
//   - question: 用户问题
//   - courses: 候选课程列表
//
// 返回值：
//   - string: 生成的推荐文本
//   - error: 错误信息
func (d *DeepSeekClient) RecommendCourses(ctx context.Context, question string, courses []map[string]interface{}) (string, error) {
	// ========================================
	// 阶段1: 提取课程文本描述
	// ========================================
	
	// 从每个课程的 payload 中提取 "text" 字段
	// 这些文本包含课程名称、教师、评价等完整信息
	textList := make([]string, 0, len(courses))
	for _, course := range courses {
		if text, ok := course["text"].(string); ok {
			textList = append(textList, sanitizeCourseText(text))
		}
	}

	// ========================================
	// 阶段2: 组装用户消息
	// ========================================
	
	// 将课程列表和用户问题拼接成一条消息
	// 格式：
	//   课程列表: ["课程1描述", "课程2描述", ...]
	//   用户提问: 我想选没有期末考试的课
	joined := fmt.Sprintf("课程列表: [\"%s\"]\n用户提问: %s",
		strings.Join(textList, "\", \""), question)

	// ========================================
	// 阶段3: 构造 API 请求
	// ========================================
	
	// 按照 OpenAI Chat Completions 格式构造请求
	// messages 包含两条消息：
	// 1. system: 定义 AI 的角色和输出格式
	// 2. user: 提供具体的任务数据
	requestBody := map[string]interface{}{
		"model": "deepseek-chat", // 使用 DeepSeek 的 Chat 模型
		"messages": []map[string]string{
			{"role": "system", "content": model.SystemPrompt}, // 系统提示词
			{"role": "user", "content": joined},               // 用户输入
		},
		"stream": false, // 不使用流式响应，等待完整结果
	}
	
	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("序列化 LLM 请求失败: %w", err)
	}

	// ========================================
	// 阶段4: 发送 HTTP 请求
	// ========================================
	
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.URL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("创建 LLM 请求失败: %w", err)
	}
	
	// 设置必需的请求头
	req.Header.Set("Authorization", "Bearer "+d.APIKey) // 认证
	req.Header.Set("Content-Type", "application/json")  // 指定内容类型

	client := d.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("LLM API 调用失败: %w", err)
	}
	defer resp.Body.Close()

	// ========================================
	// 阶段5: 读取并验证响应
	// ========================================
	
	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取 LLM 响应体失败: %w", err)
	}
	log.Printf("DeepSeek 响应状态码: %d", resp.StatusCode)

	// 检查 HTTP 状态码
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("非成功响应内容: %s", string(respData))
		return "", fmt.Errorf("API 错误，状态码: %d，响应内容: %s", resp.StatusCode, string(respData))
	}

	// ========================================
	// 阶段6: 解析 JSON 响应
	// ========================================
	
	// DeepSeek 响应格式（与 OpenAI 兼容）：
	// {
	//   "choices": [
	//     {
	//       "message": {
	//         "content": "生成的文本内容"
	//       }
	//     }
	//   ]
	// }
	var out map[string]interface{}
	if err := json.Unmarshal(respData, &out); err != nil {
		log.Printf("无法解析 JSON 响应: %s", string(respData))
		return "", err
	}

	// 逐层提取 content 字段
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

func sanitizeCourseText(text string) string {
	trimmed := strings.TrimSpace(text)
	runes := []rune(trimmed)
	if len(runes) > maxCourseContextRunes {
		return string(runes[:maxCourseContextRunes]) + "..."
	}
	return trimmed
}
	
	// 返回生成的文本内容
	// 该内容包含 LLM 的分析过程和 <|Result|> 标记后的 JSON 推荐列表
	return content, nil
}
