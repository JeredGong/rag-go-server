// go-server/internal/config/config.go
//
// 配置管理模块：负责从环境变量读取 RAG 服务运行所需的所有参数，
// 并进行默认值填充与基础校验。
package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"time"
)

const (
	// 默认嵌入向量服务端点地址
	defaultEmbedEndpoint = "https://whuworkers.jeredgong.workers.dev"
	// 默认服务监听地址
	defaultListenAddr = "127.0.0.1:8091"
	// 默认 Redis 服务器地址
	defaultRedisAddr = "127.0.0.1:6379"
	// 默认 Qdrant 集合名称
	defaultQdrantCollection = "WHUCoursesDB"
	// 默认每个设备的请求限制次数
	defaultLimitPerDevice = 10
	// 默认 RAG 检索候选结果数量上限
	defaultCandidateLimit = 40
	// 默认请求超时时间
	defaultRequestTimeout = 25 * time.Second
	// 最小请求超时时间（用于校验）
	minRequestTimeout = 5 * time.Second
	// 环境变量名：每个设备的请求限制
	limitPerDeviceEnv = "LIMIT_PER_DEVICE"
	// 环境变量名：RAG 检索候选结果数量上限
	candidateLimitEnv = "RAG_CANDIDATE_LIMIT"
	// 环境变量名：请求超时时间
	requestTimeoutEnv = "RAG_REQUEST_TIMEOUT"
	// 环境变量名：Qdrant 集合名称
	qdrantCollectionEnv = "QDRANT_COLLECTION"
)

// Config 封装 RAG 服务运行所需配置
type Config struct {
	// OpenAIAPIKey OpenAI API 密钥，用于调用 LLM 服务（必填）
	OpenAIAPIKey string
	// QdrantHost Qdrant 向量数据库主机地址（必填）
	QdrantHost string
	// QdrantAPIKey Qdrant 向量数据库 API 密钥（必填）
	QdrantAPIKey string
	// QdrantCollection Qdrant 集合名称，用于存储和检索向量数据
	QdrantCollection string
	// RedisAddr Redis 服务器地址，用于限流和缓存
	RedisAddr string
	// RedisPassword Redis 服务器密码，为空时使用空密码
	RedisPassword string
	// EmbedEndpoint 嵌入向量服务端点地址，用于将文本转换为向量
	EmbedEndpoint string
	// ListenAddr HTTP 服务监听地址，格式为 "host:port"
	ListenAddr string
	// LimitPerDevice 每个设备的请求限制次数（每分钟）
	LimitPerDevice int
	// CandidateLimit RAG 检索时返回的候选结果数量上限
	CandidateLimit int
	// RequestTimeout 单个请求的最大超时时间
	RequestTimeout time.Duration
}

// Load 从环境变量加载配置，提供合理默认值并进行校验。
// 如果必需的环境变量未设置，将返回错误。
// 对于可选配置项，将使用默认值或显示警告信息。
//
// 返回:
//   - *Config: 加载成功的配置对象
//   - error: 如果必需配置缺失或格式错误，返回相应错误
func Load() (*Config, error) {
	cfg := &Config{
		OpenAIAPIKey:     os.Getenv("OPENAI_API_KEY"),
		QdrantHost:       os.Getenv("QDRANT_HOST"),
		QdrantAPIKey:     os.Getenv("QDRANT_API_KEY"),
		QdrantCollection: os.Getenv(qdrantCollectionEnv),
		RedisAddr:        os.Getenv("REDIS_HOST"),
		RedisPassword:    os.Getenv("REDIS_PASSWORD"),
		EmbedEndpoint:    os.Getenv("EMBED_ENDPOINT"),
		ListenAddr:       os.Getenv("LISTEN_ADDR"),
		LimitPerDevice:   defaultLimitPerDevice,
		CandidateLimit:   defaultCandidateLimit,
		RequestTimeout:   defaultRequestTimeout,
	}

	if cfg.EmbedEndpoint == "" {
		cfg.EmbedEndpoint = defaultEmbedEndpoint
	}

	if cfg.ListenAddr == "" {
		cfg.ListenAddr = defaultListenAddr
	} else if !isValidAddr(cfg.ListenAddr) {
		return nil, fmt.Errorf("无效的 LISTEN_ADDR 格式: %s", cfg.ListenAddr)
	}

	if cfg.QdrantCollection == "" {
		cfg.QdrantCollection = defaultQdrantCollection
	}

	if cfg.RedisAddr == "" {
		cfg.RedisAddr = defaultRedisAddr
	}

	if cfg.RedisPassword == "" {
		fmt.Println("[Config] 警告：REDIS_PASSWORD 未设置，将使用空密码")
	}

	if limitStr := os.Getenv(limitPerDeviceEnv); limitStr != "" {
		if val, err := strconv.Atoi(limitStr); err == nil && val > 0 {
			cfg.LimitPerDevice = val
		} else {
			fmt.Printf("[Config] 警告：%s=%s 非法，已回退为默认值 %d\n", limitPerDeviceEnv, limitStr, cfg.LimitPerDevice)
		}
	}

	if candidateStr := os.Getenv(candidateLimitEnv); candidateStr != "" {
		if val, err := strconv.Atoi(candidateStr); err == nil && val > 0 {
			cfg.CandidateLimit = val
		} else {
			fmt.Printf("[Config] 警告：%s=%s 非法，使用默认值 %d\n", candidateLimitEnv, candidateStr, cfg.CandidateLimit)
		}
	}

	if timeoutStr := os.Getenv(requestTimeoutEnv); timeoutStr != "" {
		if dur, err := time.ParseDuration(timeoutStr); err == nil && dur >= minRequestTimeout {
			cfg.RequestTimeout = dur
		} else {
			fmt.Printf("[Config] 警告：%s=%s 非法，使用默认值 %s\n", requestTimeoutEnv, timeoutStr, cfg.RequestTimeout)
		}
	}

	if cfg.OpenAIAPIKey == "" {
		return nil, fmt.Errorf("环境变量 OPENAI_API_KEY 未设置")
	}

	if cfg.QdrantHost == "" || cfg.QdrantAPIKey == "" {
		return nil, fmt.Errorf("环境变量 QDRANT_HOST 或 QDRANT_API_KEY 未设置")
	}

	return cfg, nil
}

// isValidAddr 验证地址格式是否有效。
// 检查地址是否符合 "host:port" 格式，且主机和端口均不为空。
//
// 参数:
//   - addr: 待验证的地址字符串
//
// 返回:
//   - bool: 地址格式有效返回 true，否则返回 false
func isValidAddr(addr string) bool {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	return host != "" && port != ""
}
