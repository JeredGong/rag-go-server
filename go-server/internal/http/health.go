package httpapi

import (
	"time"

	"github.com/gin-gonic/gin"
)

// MakeHealthHandler 返回健康检查接口
func MakeHealthHandler(startedAt time.Time) gin.HandlerFunc {
	return func(c *gin.Context) {
		writeSuccess(c, map[string]interface{}{
			"uptime":    time.Since(startedAt).String(),
			"startedAt": startedAt.Format(time.RFC3339),
			"status":    "ok",
		})
	}
}
