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
	httpapi "rag-go-server/internal/http" // 注意别名
	"rag-go-server/internal/limit"
	"rag-go-server/internal/llm"
	"rag-go-server/internal/rag"
	"rag-go-server/internal/vectorstore"
)

const collectionName = "WHUCoursesDB"

func main() {
	// 1. 加载 .env（如果文件不存在也没关系，后面用系统 env）
	_ = godotenv.Load()

	// 2. 读取配置
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 3. 初始化 Qdrant 客户端
	qClient, err := qdrant.NewClient(&qdrant.Config{
		Host:   cfg.QdrantHost, // 例如：xxx.us-west-1-0.aws.cloud.qdrant.io
		Port:   6334,
		APIKey: cfg.QdrantAPIKey,
		UseTLS: true,
	})
	if err != nil {
		log.Fatalf("❌ Qdrant 初始化失败: %v", err)
	}
	log.Println("✅ Qdrant 客户端初始化成功")

	// 4. 初始化 Redis
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       0,
	})
	if _, err := rdb.Ping(context.Background()).Result(); err != nil {
		log.Fatalf("❌ Redis 初始化失败: %v", err)
	}
	log.Println("✅ Redis 初始化成功")

	// 5. 构造各模块实现
	embedder := embedding.NewCloudflareClient(cfg.EmbedEndpoint)
	store := vectorstore.NewQdrantStore(qClient, collectionName)
	llmClient := llm.NewDeepSeekClient(cfg.OpenAIAPIKey)
	limiter := limit.NewRedisRateLimiter(rdb, cfg.LimitPerDevice, "limit:")

	// 6. 组合成 RAG 服务
	ragService := rag.NewService(embedder, store, llmClient, limiter)

	// 7. 启动 Gin HTTP 服务
	r := gin.Default()
	r.POST("/rag", httpapi.MakeRagHandler(ragService))

	log.Printf("🚀 RAG 服务启动，监听地址: %s", cfg.ListenAddr)
	if err := r.Run(cfg.ListenAddr); err != nil {
		log.Fatalf("Gin 启动失败: %v", err)
	}
}
