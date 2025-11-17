package httpapi

import (
	"log"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	requestIDContextKey = "_request_id"
	requestIDHeader     = "X-Request-ID"
)

// RequestLogger 中间件：生成请求 ID 并记录基本访问日志
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader(requestIDHeader)
		if requestID == "" {
			requestID = uuid.New().String()
		}
		c.Set(requestIDContextKey, requestID)
		c.Writer.Header().Set(requestIDHeader, requestID)

		start := time.Now()
		c.Next()
		latency := time.Since(start)
		status := c.Writer.Status()
		log.Printf("[HTTP] %s %s -> %d (%s) rid=%s", c.Request.Method, c.FullPath(), status, latency, requestID)

		for _, e := range c.Errors {
			log.Printf("[HTTP] rid=%s error=%v", requestID, e.Err)
		}
	}
}

// RequestIDFromContext 返回当前请求的 Request ID
func RequestIDFromContext(c *gin.Context) string {
	if v, ok := c.Get(requestIDContextKey); ok {
		if id, ok := v.(string); ok {
			return id
		}
	}
	return ""
}
