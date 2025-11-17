// go-server/internal/config/config.go
//
// 配置管理模块
//
// 本模块负责从环境变量中读取应用配置，并进行校验和默认值处理。
// 采用环境变量的方式便于在不同环境（开发、测试、生产）中灵活配置，
// 也符合 12-Factor App 的最佳实践。
//
// 配置加载优先级：
// 1. 环境变量（由 .env 文件或系统环境变量提供）
// 2. 代码中定义的默认值
//
// 配置验证策略：
// - 必填项缺失时会返回错误，阻止服务启动
// - 可选项使用合理的默认值，并在必要时打印警告信息
package config

import (
	"fmt"
	"net"
	"os"
)

// Config 封装 RAG 服务运行所需的全部配置项
type Config struct {
	// OpenAIAPIKey DeepSeek API 的密钥（兼容 OpenAI 接口格式）
	// 用于调用大语言模型生成课程推荐
	OpenAIAPIKey string
	
	// QdrantHost Qdrant 向量数据库的主机地址
	// 格式示例：xxx.us-west-1-0.aws.cloud.qdrant.io
	QdrantHost string
	
	// QdrantAPIKey Qdrant 的 API 密钥，用于身份认证
	QdrantAPIKey string
	
	// RedisAddr Redis 服务器地址
	// 格式：host:port，例如 127.0.0.1:6379
	RedisAddr string
	
	// RedisPassword Redis 的访问密码
	// 本地开发环境通常为空，生产环境应设置强密码
	RedisPassword string
	
	// EmbedEndpoint 向量嵌入服务的 HTTP 端点
	// 用于将文本转换为向量表示
	EmbedEndpoint string
	
	// ListenAddr HTTP 服务器监听的地址
	// 格式：host:port，例如 127.0.0.1:8091
	ListenAddr string
	
	// LimitPerDevice 每个设备指纹在一个周期内的访问配额
	// 用于限流，防止滥用
	LimitPerDevice int
}

// Load 从环境变量中加载配置，并进行校验和默认值填充
//
// 返回值：
//   - *Config: 加载并验证后的配置对象
//   - error: 如果必填配置缺失或格式无效，返回错误信息
//
// 环境变量列表：
//   - OPENAI_API_KEY: DeepSeek API 密钥（必填）
//   - QDRANT_HOST: Qdrant 主机地址（必填）
//   - QDRANT_API_KEY: Qdrant API 密钥（必填）
//   - REDIS_HOST: Redis 地址（可选，默认 127.0.0.1:6379）
//   - REDIS_PASSWORD: Redis 密码（可选，默认为空）
//   - EMBED_ENDPOINT: 向量化服务地址（可选，有默认值）
//   - LISTEN_ADDR: 服务监听地址（可选，默认 127.0.0.1:8091）
func Load() (*Config, error) {
	// 从环境变量中读取所有配置项
	cfg := &Config{
		OpenAIAPIKey:   os.Getenv("OPENAI_API_KEY"),
		QdrantHost:     os.Getenv("QDRANT_HOST"),
		QdrantAPIKey:   os.Getenv("QDRANT_API_KEY"),
		RedisAddr:      os.Getenv("REDIS_HOST"),
		RedisPassword:  os.Getenv("REDIS_PASSWORD"),
		EmbedEndpoint:  os.Getenv("EMBED_ENDPOINT"),
		ListenAddr:     os.Getenv("LISTEN_ADDR"),
		LimitPerDevice: 10, // 默认每设备每周 10 次访问配额
	}

	// ========================================
	// 阶段1: 处理向量嵌入服务配置
	// ========================================
	
	// 如果未设置，使用默认的 Cloudflare Worker 端点
	// 该端点封装了 BGE-M3 模型，提供文本向量化服务
	if cfg.EmbedEndpoint == "" {
		cfg.EmbedEndpoint = "https://whuworkers.jeredgong.workers.dev"
	}

	// ========================================
	// 阶段2: 处理 HTTP 服务器监听地址
	// ========================================
	
	if cfg.ListenAddr == "" {
		// 默认监听本地 8091 端口
		// 生产环境建议使用 0.0.0.0:8091 以接受外部请求
		cfg.ListenAddr = "127.0.0.1:8091"
	} else if !isValidAddr(cfg.ListenAddr) {
		// 验证地址格式，避免因配置错误导致启动失败
		return nil, fmt.Errorf("无效的 LISTEN_ADDR 格式: %s", cfg.ListenAddr)
	}

	// ========================================
	// 阶段3: 处理 Redis 配置
	// ========================================
	
	if cfg.RedisAddr == "" {
		// 默认连接本地 Redis 实例
		cfg.RedisAddr = "127.0.0.1:6379"
	}

	if cfg.RedisPassword == "" {
		// Redis 密码为空时给出警告
		// 本地开发环境这是正常的，但生产环境应该配置密码
		fmt.Println("[Config] 警告：REDIS_PASSWORD 未设置，将使用空密码")
	}

	// ========================================
	// 阶段4: 验证必填配置项
	// ========================================
	
	// DeepSeek API 密钥是必须的，否则无法调用大模型
	if cfg.OpenAIAPIKey == "" {
		return nil, fmt.Errorf("环境变量 OPENAI_API_KEY 未设置")
	}

	// Qdrant 配置是必须的，否则无法进行向量检索
	if cfg.QdrantHost == "" || cfg.QdrantAPIKey == "" {
		return nil, fmt.Errorf("环境变量 QDRANT_HOST 或 QDRANT_API_KEY 未设置")
	}

	return cfg, nil
}

// isValidAddr 验证网络地址格式是否有效
//
// 参数：
//   - addr: 要验证的地址字符串，格式应为 host:port
//
// 返回值：
//   - bool: 地址格式是否有效
//
// 验证规则：
//   - 必须能够成功解析为 host 和 port 两部分
//   - host 和 port 都不能为空
func isValidAddr(addr string) bool {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	if host == "" || port == "" {
		return false
	}
	return true
}
