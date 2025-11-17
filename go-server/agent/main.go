// go-server/main.go
package main

import (
	"context"
	"log"

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

const collectionName = "WHUCoursesDB"

func main() {
	// ============================================================
	// 1. 加载环境变量文件（可选）
	// ============================================================
	if err := godotenv.Load(); err != nil {
		log.Println("⚠️  未找到 .env 文件，继续使用系统环境变量")
	}

	// ============================================================
	// 2. 加载系统配置
	// ============================================================
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("❌ 配置加载失败: %v", err)
	}
	log.Println("✅ 配置加载成功")

	// ============================================================
	// 3. 初始化 Qdrant 客户端
	// ============================================================
	qClient, err := qdrant.NewClient(&qdrant.Config{
		Host:   cfg.QdrantHost,
		Port:   6334,
		APIKey: cfg.QdrantAPIKey,
		UseTLS: true,
	})
	if err != nil {
		log.Fatalf("❌ Qdrant 客户端初始化失败: %v", err)
	}
	log.Printf("✅ 已连接 Qdrant 向量库: %s\n", cfg.QdrantHost)

	// ============================================================
	// 4. 初始化 Redis
	// ============================================================
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       0,
	})
	if _, err := rdb.Ping(context.Background()).Result(); err != nil {
		log.Fatalf("❌ Redis 连接失败: %v", err)
	}
	log.Printf("✅ 已连接 Redis: %s\n", cfg.RedisAddr)

	// ============================================================
	// 5. 创建业务模块（Embedder / VectorStore / LLM / Limiter）
	// ============================================================
	embedder := embedding.NewCloudflareClient(cfg.EmbedEndpoint)
	store := vectorstore.NewQdrantStore(qClient, collectionName)
	llmClient := llm.NewDeepSeekClient(cfg.OpenAIAPIKey)
	limiter := limit.NewRedisRateLimiter(rdb, cfg.LimitPerDevice, "limit:")

	// ============================================================
	// 6. 组合成 RAG 服务
	// ============================================================
	ragService := rag.NewService(embedder, store, llmClient, limiter)
	log.Println("✨ RAG 服务初始化完成")

	// ============================================================
	// 7. 启动 Gin HTTP 服务
	// ============================================================
	router := gin.Default()
	router.POST("/rag", httpapi.MakeRagHandler(ragService))

	log.Printf("🚀 服务器启动中，监听地址: %s\n", cfg.ListenAddr)
	if err := router.Run(cfg.ListenAddr); err != nil {
		log.Fatalf("❌ Gin 启动失败: %v", err)
	}
}
