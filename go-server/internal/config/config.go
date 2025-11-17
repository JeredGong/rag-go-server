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
	defaultEmbedEndpoint     = "https://whuworkers.jeredgong.workers.dev"
	defaultListenAddr        = "127.0.0.1:8091"
	defaultRedisAddr         = "127.0.0.1:6379"
	defaultQdrantCollection  = "WHUCoursesDB"
	defaultLimitPerDevice    = 10
	defaultCandidateLimit    = 40
	defaultRequestTimeout    = 25 * time.Second
	minRequestTimeout        = 5 * time.Second
	limitPerDeviceEnv        = "LIMIT_PER_DEVICE"
	candidateLimitEnv        = "RAG_CANDIDATE_LIMIT"
	requestTimeoutEnv        = "RAG_REQUEST_TIMEOUT"
	qdrantCollectionEnv      = "QDRANT_COLLECTION"
)

// Config 封装 RAG 服务运行所需配置
type Config struct {
	OpenAIAPIKey     string
	QdrantHost       string
	QdrantAPIKey     string
	QdrantCollection string
	RedisAddr        string
	RedisPassword    string
	EmbedEndpoint    string
	ListenAddr       string
	LimitPerDevice   int
	CandidateLimit   int
	RequestTimeout   time.Duration
}

// Load 从环境变量加载配置，提供合理默认值并进行校验
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

// isValidAddr 验证地址格式是否有效
func isValidAddr(addr string) bool {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	return host != "" && port != ""
}
