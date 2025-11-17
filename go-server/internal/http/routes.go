package httpapi

import (
	"time"

	"github.com/gin-gonic/gin"

	"rag-go-server/internal/rag"
)

// RegisterRoutes 挂载所有 HTTP 路由
func RegisterRoutes(r *gin.Engine, service *rag.Service, startedAt time.Time) {
	r.POST("/rag", MakeRagHandler(service))
	r.GET("/healthz", MakeHealthHandler(startedAt))
}
