// main.go
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
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/qdrant/go-client/qdrant"
	"github.com/redis/go-redis/v9"
)

type RagRequest struct {
	UserQuestion string `json:"userQuestion"`
	Catagory     int    `json:"catagory"`
}

type RagResponse struct {
	Status string                 `json:"status"`
	Data   map[string]interface{} `json:"data"`
}

const SEP_TOKEN = "<|Result|>"

var (
	rpcClient      *qdrant.Client
	collectionName = "WHUCoursesDB"
	openaiAPIKey   = "sk-27bf4f76e0004738ba601aa7e1b8744e"
	embedEndpoint  = "https://whuworkers.jeredgong.workers.dev"
)
var (
	redisClient *redis.Client
	ctx         = context.Background()
)

func main() {
	// 加载 .env 文件
	errenv := godotenv.Load()
	if errenv != nil {
		log.Fatalf("加载 .env 文件失败: %v", errenv)
	}
	// ✅ 建立 Qdrant 客户端（推荐方式）
	var err error
	rpcClient, err = qdrant.NewClient(&qdrant.Config{
		Host:   os.Getenv("QDRANT_HOST"), // 例如：a7dcca84-xxx.us-west-1-0.aws.cloud.qdrant.io
		Port:   6334,
		APIKey: os.Getenv("QDRANT_API_KEY"), // 从 Qdrant Cloud 控制台获取
		UseTLS: true,                        // 强烈建议在生产中开启 TLS
	})
	if err != nil {
		log.Fatalf("❌ Qdrant 初始化失败: %v", err)
	}
	initRedis()
	log.Println("✅ Qdrant 客户端初始化成功")
	r := gin.Default()
	r.POST("/rag", ragHandler)
	r.Run("127.0.0.1:8089") // 启动服务，监听 8089 端口
}
func initRedis() {
	redisClient = redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDIS_HOST"),     // 例：localhost:6379
		Password: os.Getenv("REDIS_PASSWORD"), // 如果无密码则留空
		DB:       0,
	})

	_, err := redisClient.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("❌ Redis 初始化失败: %v", err)
	}
	log.Println("✅ Redis 初始化成功")
}
func ragHandler(c *gin.Context) {
	// 读取请求体
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, RagResponse{Status: "error", Data: map[string]interface{}{"message": "读取请求数据失败"}})
		return
	}

	// 解析请求体
	var req RagRequest
	if err := json.Unmarshal(bodyBytes, &req); err != nil {
		c.JSON(http.StatusBadRequest, RagResponse{Status: "error", Data: map[string]interface{}{"message": err.Error()}})
		return
	}
	// 获取用户指纹，这是一个唯一标识符，用于限制访问频率
	fingerprint := c.GetHeader("X-Device-Fingerprint")
	// 如果这个头不存在,若不存在则返回错误
	if fingerprint == "" {
		log.Println("缺少设备指纹")
		c.JSON(http.StatusBadRequest, RagResponse{Status: "error", Data: map[string]interface{}{"message": "缺少设备指纹"}})
		return
	}
	// 检查指纹是否在Redis中，若不在则设置
	if _, err := CheckFingerprintExists(fingerprint); err != nil {
		// 设置指纹访问限制为 10 次
		if err := SetFingerprintLimit(fingerprint, 10); err != nil {
			log.Println("设置访问限制失败:", err)
			c.JSON(http.StatusInternalServerError, RagResponse{Status: "error", Data: map[string]interface{}{"message": "设置访问限制失败"}})
			return
		}
		log.Printf("新设备指纹 %s 已设置访问限制", fingerprint)
	} else {
		log.Printf("设备指纹 %s 已存在，检查访问限制", fingerprint)
	}
	// 检查指纹访问限制
	if _, err := DecrementFingerprintLimit(fingerprint); err != nil {
		log.Println("访问已达上限", err)
		c.JSON(http.StatusInternalServerError, RagResponse{Status: "error", Data: map[string]interface{}{"message": "访问限制检查失败"}})
		return
	}

	embedding, err := getEmbeddingFromCloudflare(req.UserQuestion)
	if err != nil {
		c.JSON(http.StatusInternalServerError, RagResponse{Status: "error", Data: map[string]interface{}{"message": err.Error()}})
		return
	}

	// 搜索 Qdrant 数据库
	similarCourses, err := searchQdrant(embedding, req.Catagory)
	if err != nil {
		log.Println("搜索 Qdrant 失败:", err)
		c.JSON(http.StatusInternalServerError, RagResponse{Status: "error", Data: map[string]interface{}{"message": err.Error()}})
		return
	}

	// 调用 OpenAI 或 DeepSeek 生成回答
	llmOutput, err := callOpenAI(req.UserQuestion, similarCourses)
	if err != nil {
		log.Println("调用 OpenAI 或 DeepSeek 失败:", err) // 新增日志：LLM 调用失败
		c.JSON(http.StatusInternalServerError, RagResponse{Status: "error", Data: map[string]interface{}{"message": err.Error()}})
		return
	}

	// 返回最终的响应
	c.JSON(http.StatusOK, RagResponse{
		Status: "success",
		Data: map[string]interface{}{
			"rag_results": similarCourses,
			"llm_output":  llmOutput,
		},
	})
}

func getEmbeddingFromCloudflare(text string) ([]float32, error) {
	body := map[string]interface{}{"text": text}
	jsonData, _ := json.Marshal(body)

	resp, err := http.Post(embedEndpoint, "application/json", bytes.NewBuffer(jsonData))
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
	for i, val := range result.Embedding.Data[0] {
		vec[i] = float32(val)
	}
	return vec, nil
}

func searchQdrant(vector []float32, catagory int) ([]map[string]interface{}, error) {
	log.Println("开始构造 Qdrant 查询请求...")

	// 将 100 转换为 uint64 类型
	limit := uint64(100)

	// 创建查询请求
	query := qdrant.NewQuery(vector...)
	req := &qdrant.QueryPoints{
		CollectionName: collectionName,
		Query:          query,
		Limit:          &limit, // 使用 *uint64 类型
		WithPayload:    qdrant.NewWithPayload(true),
	}

	// 打印请求数据
	log.Printf("查询请求: %+v\n", req)

	// 使用 Query 方法进行查询
	log.Println("发送查询请求到 Qdrant...")
	resp, err := rpcClient.Query(context.Background(), req)
	if err != nil {
		log.Println("Qdrant 查询失败:", err)
		return nil, err
	}
	log.Println("Qdrant 查询成功，返回结果：", resp)

	// 解析查询结果，查询结果应该在 resp.Hits 或类似的字段中
	var matches []map[string]interface{}
	for _, pt := range resp {
		payload := make(map[string]interface{})
		for k, v := range pt.Payload {
			switch val := v.Kind.(type) {
			case *qdrant.Value_StringValue:
				payload[k] = val.StringValue
			case *qdrant.Value_IntegerValue:
				payload[k] = int(val.IntegerValue)
			case *qdrant.Value_BoolValue:
				payload[k] = val.BoolValue
			case *qdrant.Value_DoubleValue:
				payload[k] = val.DoubleValue
			}
		}
		if catagory == 0 {
			matches = append(matches, payload)
		} else if val, ok := payload["catagory"]; ok {
			switch v := val.(type) {
			case float64:
				if int(v) == catagory {
					matches = append(matches, payload)
				}
			case int:
				if v == catagory {
					matches = append(matches, payload)
				}
			}
		}
	}

	log.Printf("找到 %d 个匹配的课程", len(matches))

	return matches, nil
}

func callOpenAI(question string, courses []map[string]interface{}) (string, error) {
	textList := make([]string, 0, len(courses))
	for _, course := range courses {
		if text, ok := course["text"].(string); ok {
			textList = append(textList, text)
		}
	}
	joined := fmt.Sprintf("课程列表: [\"%s\"]\n用户提问: %s", strings.Join(textList, "\", \""), question)

	requestBody := map[string]interface{}{
		"model": "deepseek-chat", // 使用 DeepSeek 模型名
		"messages": []map[string]string{
			{"role": "system", "content": SYSTEM_PROMPT},
			{"role": "user", "content": joined},
		},
		"stream": false, // 明确设置非流式
	}
	jsonBody, _ := json.Marshal(requestBody)

	req, _ := http.NewRequest("POST", "https://api.deepseek.com/chat/completions", bytes.NewBuffer(jsonBody))
	req.Header.Set("Authorization", "Bearer "+openaiAPIKey) // 使用你的 DeepSeek API Key
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// 打印响应的状态码和返回内容，帮助调试
	log.Printf("收到响应状态码: %d\n", resp.StatusCode)
	respData, _ := io.ReadAll(resp.Body)

	// 如果返回的不是 2xx 状态码，输出响应内容并返回错误
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("非成功响应内容: %s", string(respData))
		return "", fmt.Errorf("API 错误，状态码: %d，响应内容: %s", resp.StatusCode, string(respData))
	}

	// 解析 JSON 响应
	var out map[string]interface{}
	if err := json.Unmarshal(respData, &out); err != nil {
		log.Printf("无法解析 JSON 响应: %s", string(respData))
		return "", err
	}

	choices := out["choices"].([]interface{})
	content := choices[0].(map[string]interface{})["message"].(map[string]interface{})["content"].(string)
	return content, nil
}
func nextThursdayMidnight() time.Time {
	now := time.Now()
	weekday := now.Weekday()
	// 计算距离下个周四的天数（Weekday 是 time.Sunday=0）
	daysUntilThursday := (4 - int(weekday) + 7) % 7
	if daysUntilThursday == 0 {
		daysUntilThursday = 7
	}
	thursday := now.AddDate(0, 0, daysUntilThursday)
	return time.Date(thursday.Year(), thursday.Month(), thursday.Day(), 0, 0, 0, 0, thursday.Location())
}
func SetFingerprintLimit(fingerprint string, times int) error {
	expireAt := nextThursdayMidnight()
	duration := time.Until(expireAt)

	key := "limit:" + fingerprint
	return redisClient.Set(ctx, key, times, duration).Err()
}
func CheckFingerprintExists(fingerprint string) (bool, error) {
	key := "limit:" + fingerprint
	exists, err := redisClient.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return exists == 1, nil
}
func DecrementFingerprintLimit(fingerprint string) (bool, error) {
	key := "limit:" + fingerprint

	val, err := redisClient.Get(ctx, key).Int()
	if err == redis.Nil {
		return false, fmt.Errorf("没有设置访问限制")
	} else if err != nil {
		return false, err
	}

	if val <= 0 {
		return false, nil
	}

	// 使用 pipeline 原子减并保留 TTL
	pipe := redisClient.TxPipeline()
	pipe.Decr(ctx, key)
	ttl := redisClient.TTL(ctx, key)
	_, err = pipe.Exec(ctx)
	if err != nil {
		return false, err
	}

	// 重新设置过期时间（保证延迟处理不失效）
	redisClient.Expire(ctx, key, ttl.Val())

	return true, nil
}

const SYSTEM_PROMPT = `你是一个课程选择助手。
在用户的输入部分，你会得到一个json格式的字符串，叫做课程列表，以及一段查询。
json格式的字符串是一个列表，列表中的每个元素是一个字符串，描述了一个课程。
查询就是一个朴素的字符串，是用户的选课需求。
你的任务是：从课程列表中选择一到三门课程作为用户的选课推荐，要求尽可能满足用户的选课需求。
请注意，课程列表可能混入一些随机数据，也可能是一个空列表，没有可用或满足要求课程的情况下你可以不推荐任何课程。
你可以输出任何思考过程，但是最终需要形式化的给出结果。具体地说，你可以先输出任何东西，比如解析用户的需求，
分析提供的课程列表等。然后你需要输出一个特别标志 <|Result|>，在该标志后面是一个json格式的列表。列表中的
每个元素是一个字典，包含\"课程名称\"和\"理由\"两个项目。理由应该尽可能简要。
具体地说，你的输出应该保持如下格式：

这里是你的分析过程。<|Result|>
[{\"course\": \"你推荐课程的名称1\", \"reason\": \"你推荐课程的理由1\"},
 {\"course\": \"你推荐课程的名称2\", \"reason\": \"你推荐课程的理由2\"},
 {\"course\": \"你推荐课程的名称3\", \"reason\": \"你推荐课程的理由3\"}]

以下是用户的输入`
