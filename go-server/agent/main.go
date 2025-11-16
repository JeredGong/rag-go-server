// agent/main.go
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/tmc/langchaingo/agents"
	"github.com/tmc/langchaingo/llms/openai"
	"github.com/tmc/langchaingo/tools"

	"rag-go-server/internal/model"
	"rag-go-server/internal/rag"
)

// RagTool 封装现有 /rag 接口调用的工具
type RagTool struct {
	// 课程分类 ID（0 表示不过滤）
	Category     int
	RagServerURL string
}

// Name 返回工具名称（Agent 提示中会用到）
func (t RagTool) Name() string {
	return "CourseSearch"
}

// Description 返回工具描述，指导 LLM 何时使用该工具
func (t RagTool) Description() string {
	desc := "一个用于检索课程推荐的工具。" +
		"给定用户的选课问题，它返回相关课程列表及推荐理由。" +
		"当需要根据用户问题查找课程信息时应调用此工具。"
	if t.Category != 0 {
		desc += fmt.Sprintf("（当前工具限定课程分类ID=%d）", t.Category)
	}
	return desc
}

// Call 方法封装对 /rag 接口的实际调用
func (t RagTool) Call(ctx context.Context, input string) (string, error) {
	// 构造 RagRequest 请求体
	reqBody := model.RagRequest{
		UserQuestion: input,
		Catagory:     t.Category,
	}
	data, _ := json.Marshal(reqBody)

	// /rag URL：允许从 RagServerURL 配置，未配置时走默认
	url := t.RagServerURL
	if url == "" {
		url = "http://127.0.0.1:8089/rag"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	// 传递设备指纹用于复用 /rag 的限流机制
	if fp := ctx.Value("fingerprint"); fp != nil {
		if s, ok := fp.(string); ok && s != "" {
			req.Header.Set("X-Device-Fingerprint", s)
		}
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("调用 /rag 接口失败: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("RAG 接口错误（HTTP %d）: %s", resp.StatusCode, string(body))
	}

	// 解析 /rag 返回的 JSON，复用统一的 CourseRecommendation 类型
	var ragResp struct {
		Status string `json:"status"`
		Data   struct {
			Recommendations []model.CourseRecommendation `json:"recommendations"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &ragResp); err != nil {
		return "", fmt.Errorf("解析 /rag 响应失败: %w", err)
	}

	// 将推荐列表序列化为 JSON 字符串，作为工具的输出
	recsJSON, _ := json.Marshal(ragResp.Data.Recommendations)
	return string(recsJSON), nil
}

func main() {
	// 1. 加载 .env（如果存在）
	_ = godotenv.Load()

	// 2. 读取 LLM 相关配置
	openaiAPIKey := os.Getenv("OPENAI_API_KEY")
	if openaiAPIKey == "" {
		log.Fatal("未在环境中找到 OPENAI_API_KEY")
	}
	// DeepSeek 的 OpenAI 兼容 Base URL
	baseURL := os.Getenv("OPENAI_BASE_URL")
	if baseURL == "" {
		baseURL = os.Getenv("OPENAI_API_BASE")
	}

	// 3. 初始化 LLM（DeepSeek 模型，OpenAI 兼容接口）
	llmOpts := []openai.Option{
		openai.WithModel("deepseek-chat"),
		openai.WithToken(openaiAPIKey),
	}
	if baseURL != "" {
		llmOpts = append(llmOpts, openai.WithBaseURL(baseURL))
	}
	llm, err := openai.New(llmOpts...)
	if err != nil {
		log.Fatalf("初始化 LLM 失败: %v", err)
	}

	// 4. 构建 ReAct Agent
	agentPrefix := `你是一个课程推荐智能助手。
你可以访问一个名为 CourseSearch 的工具来帮助查询课程信息。该工具会根据用户的问题检索相关课程列表及理由供你参考。
请按照以下要求与格式提供回答：
1. 先输出你的分析过程。
2. 然后输出特别标志 <|Result|>。
3. 在该标志后面输出 JSON 格式的课程推荐列表，每个元素包含 "course" 和 "reason" 字段。`

	agent := agents.NewOneShotAgent(
		llm,
		nil, // 工具每次请求时动态设置
		agents.WithPromptPrefix(agentPrefix),
		agents.WithMaxIterations(3),
	)
	executor := agents.NewExecutor(agent)

	// /rag 服务地址可以通过环境变量配置，方便部署
	ragServerURL := os.Getenv("RAG_SERVER_URL")
	if ragServerURL == "" {
		ragServerURL = "http://127.0.0.1:8091/rag"
	}

	// 5. 启动 HTTP 服务（Gin）
	r := gin.Default()
	r.POST("/agent", func(c *gin.Context) {
		// 请求体：结构与 /rag 保持一致，方便前端复用
		var req model.RagRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, model.RagResponse{
				Status: "error",
				Data:   map[string]interface{}{"message": "请求格式错误: " + err.Error()},
			})
			return
		}

		// 获取设备指纹并检查（保持和 /rag 相同的约束）
		fingerprint := c.GetHeader("X-Device-Fingerprint")
		if fingerprint == "" {
			log.Println("缺少设备指纹")
			c.JSON(http.StatusBadRequest, model.RagResponse{
				Status: "error",
				Data:   map[string]interface{}{"message": "缺少设备指纹"},
			})
			return
		}

		// 每次请求创建工具实例，注入分类与 /rag URL
		tool := RagTool{
			Category:     req.Catagory,
			RagServerURL: ragServerURL,
		}
		agent.Tools = []tools.Tool{tool}

		// 上下文中附带 fingerprint，以便 RagTool 中透传
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		ctx = context.WithValue(ctx, "fingerprint", fingerprint)

		// 调用 Agent 执行
		outputMap, err := executor.Call(ctx, map[string]any{"input": req.UserQuestion})
		if err != nil {
			log.Println("Agent 执行失败:", err)
			c.JSON(http.StatusInternalServerError, model.RagResponse{
				Status: "error",
				Data:   map[string]interface{}{"message": "Agent 调用失败: " + err.Error()},
			})
			return
		}

		// 默认输出键为 "output"
		resultStr, _ := outputMap["output"].(string)
		log.Println("Agent 原始输出:", resultStr)

		// ✅ 使用内部的 rag.ParseLLMOutput 解析 <|Result|> 后的 JSON
		recs, err := rag.ParseLLMOutput(resultStr)
		if err != nil {
			log.Println("解析 Agent 输出失败:", err)
			c.JSON(http.StatusInternalServerError, model.RagResponse{
				Status: "error",
				Data:   map[string]interface{}{"message": "解析 Agent 输出失败: " + err.Error()},
			})
			return
		}

		// 对外响应结构与 /rag 保持一致（recommendations 为数组）
		recommendations := make([]map[string]interface{}, 0, len(recs))
		for _, rrec := range recs {
			recommendations = append(recommendations, map[string]interface{}{
				"course": rrec.Course,
				"reason": rrec.Reason,
			})
		}

		c.JSON(http.StatusOK, model.RagResponse{
			Status: "success",
			Data: map[string]interface{}{
				"recommendations": recommendations,
			},
		})
	})

	// Agent 服务建议监听不同端口，例如 8089
	addr := "127.0.0.1:8089"
	if v := os.Getenv("AGENT_LISTEN_ADDR"); v != "" {
		addr = v
	}
	log.Println("🚀 Agent 服务启动，监听地址:", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Agent Gin 启动失败: %v", err)
	}
}
