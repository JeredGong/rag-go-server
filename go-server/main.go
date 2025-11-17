// go-server/main.go
//
// 这是 RAG Go Server 的主入口文件，负责整个应用的启动和初始化流程。
//
// 主要职责：
// 1. 加载环境变量配置（从 .env 文件或系统环境变量）
// 2. 初始化各个外部依赖：Qdrant（向量数据库）、Redis（限流存储）
// 3. 构造核心业务模块：向量嵌入服务、向量存储、大语言模型客户端、限流器
// 4. 组装完整的 RAG 服务并注册 HTTP 路由
// 5. 启动 Gin Web 服务器监听请求
//
// 整体架构采用依赖注入模式，各模块通过接口解耦，便于测试和替换实现。
package main

import (
	"context"
	"log"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/qdrant/go-client/qdrant"
	"github.com/redis/go-redis/v9"

	"rag-go-server/internal/config"
	"rag-go-server/internal/embedding"
	httpapi "rag-go-server/internal/http"
	"rag-go-server/internal/limit"
	"rag-go-server/internal/llm"
	"rag-go-server/internal/rag"
	"rag-go-server/internal/vectorstore"
)

func main() {
	// ========================================
	// 阶段1: 加载配置
	// ========================================
	
	// 尝试从当前目录加载 .env 文件
	// 如果文件不存在也不会报错，后续会从系统环境变量中读取配置
	_ = godotenv.Load()

	// 从环境变量中读取所有必需的配置项
	// 包括：API密钥、数据库地址、限流参数等
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// ========================================
	// 阶段2: 初始化 Qdrant 向量数据库客户端
	// ========================================
	
	// Qdrant 是一个高性能的向量搜索引擎，用于存储和检索课程向量
	// 配置说明：
	// - Host: Qdrant 服务器地址（Cloud 或自建）
	// - Port: gRPC 端口，Cloud 默认为 6334
	// - APIKey: 认证密钥
	// - UseTLS: Cloud 服务必须启用 TLS
	qClient, err := qdrant.NewClient(&qdrant.Config{
		Host:   cfg.QdrantHost,
		Port:   6334,
		APIKey: cfg.QdrantAPIKey,
		UseTLS: true,
	})
	if err != nil {
		log.Fatalf("❌ Qdrant 初始化失败: %v", err)
	}
	log.Println("✅ Qdrant 客户端初始化成功")

	// ========================================
	// 阶段3: 初始化 Redis 客户端
	// ========================================
	
	// Redis 用于实现分布式限流功能
	// 记录每个设备指纹的访问次数，并自动在每周四凌晨重置配额
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,     // Redis 服务器地址，格式：host:port
		Password: cfg.RedisPassword,  // Redis 密码，本地测试环境可为空
		DB:       0,                  // 使用默认数据库（DB 0）
	})
	
	// 通过 Ping 命令验证 Redis 连接是否正常
	if _, err := rdb.Ping(context.Background()).Result(); err != nil {
		log.Fatalf("❌ Redis 初始化失败: %v", err)
	}
	log.Println("✅ Redis 初始化成功")

	// ========================================
	// 阶段4: 构造各个业务模块
	// ========================================
	
	// embedder: 负责将用户的自然语言问题转换为向量表示
	// 使用 Cloudflare Worker 提供的 BGE-M3 模型服务
	embedder := embedding.NewCloudflareClient(cfg.EmbedEndpoint)
	
	// store: 封装 Qdrant 的向量检索操作
	// 提供统一的接口在指定集合中搜索最相似的课程
	store := vectorstore.NewQdrantStore(qClient, cfg.QdrantCollection)
	
	// llmClient: 大语言模型客户端，用于生成课程推荐和解释
	// 使用 DeepSeek Chat API（兼容 OpenAI 接口格式）
	llmClient := llm.NewDeepSeekClient(cfg.OpenAIAPIKey)
	
	// limiter: 限流器，基于 Redis 实现设备级访问频率控制
	// "limit:" 是 Redis key 的前缀，用于隔离限流相关的数据
	limiter := limit.NewRedisRateLimiter(rdb, cfg.LimitPerDevice, "limit:")

	// ========================================
	// 阶段5: 组装 RAG 服务
	// ========================================
	
	// 将上述四个模块注入到 RAG 服务中
	// RAG 服务会协调它们完成完整的检索增强生成流程：
	// 用户问题 → 向量化 → 向量检索 → LLM 生成 → 结构化输出
	ragService := rag.NewService(
		embedder,
		store,
		llmClient,
		limiter,
		rag.WithCandidateLimit(cfg.CandidateLimit),
		rag.WithRequestTimeout(cfg.RequestTimeout),
	)
	startedAt := time.Now()

	// ========================================
	// 阶段6: 配置 HTTP 服务器
	// ========================================
	
	r := gin.New()
	r.Use(gin.Recovery(), httpapi.RequestLogger())
	httpapi.RegisterRoutes(r, ragService, startedAt)

	// ========================================
	// 阶段7: 启动服务器
	// ========================================
	
	log.Printf("🚀 RAG 服务启动，监听地址: %s", cfg.ListenAddr)
	
	// 启动 HTTP 服务器，开始接受请求
	// Run 方法会阻塞，直到服务器关闭或发生错误
	if err := r.Run(cfg.ListenAddr); err != nil {
		log.Fatalf("Gin 启动失败: %v", err)
	}
}
