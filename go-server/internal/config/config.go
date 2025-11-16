// go-server/internal/config/config.go
package config

import (
	"fmt"
	"os"
)

// Config 描述 RAG 服务运行需要的配置
type Config struct {
	OpenAIAPIKey   string
	QdrantHost     string
	QdrantAPIKey   string
	RedisAddr      string
	RedisPassword  string
	EmbedEndpoint  string
	ListenAddr     string
	LimitPerDevice int
}

// Load 从环境变量中加载配置
func Load() (*Config, error) {
	cfg := &Config{
		OpenAIAPIKey:   os.Getenv("OPENAI_API_KEY"),
		QdrantHost:     os.Getenv("QDRANT_HOST"),
		QdrantAPIKey:   os.Getenv("QDRANT_API_KEY"),
		RedisAddr:      os.Getenv("REDIS_HOST"),
		RedisPassword:  os.Getenv("REDIS_PASSWORD"),
		EmbedEndpoint:  os.Getenv("EMBED_ENDPOINT"),
		ListenAddr:     os.Getenv("LISTEN_ADDR"),
		LimitPerDevice: 10, // 默认每设备每周 10 次
	}

	// ===== 默认值处理 =====

	if cfg.EmbedEndpoint == "" {
		cfg.EmbedEndpoint = "https://whuworkers.jeredgong.workers.dev"
	}

	if cfg.ListenAddr == "" {
		cfg.ListenAddr = "127.0.0.1:8091"
	}

	// ===== Redis 默认值增强 =====

	if cfg.RedisAddr == "" {
		cfg.RedisAddr = "127.0.0.1:6379" // 本地默认 Redis
	}

	if cfg.RedisPassword == "" {
		// 默认无密码，打印一次提醒即可（不阻断）
		fmt.Println("[Config] 警告：REDIS_PASSWORD 未设置，将使用空密码")
	}

	// ===== 必填校验：仅校验真正必须的配置 =====

	if cfg.OpenAIAPIKey == "" {
		return nil, fmt.Errorf("环境变量 OPENAI_API_KEY 未设置")
	}

	if cfg.QdrantHost == "" || cfg.QdrantAPIKey == "" {
		return nil, fmt.Errorf("环境变量 QDRANT_HOST 或 QDRANT_API_KEY 未设置")
	}

	return cfg, nil
}
