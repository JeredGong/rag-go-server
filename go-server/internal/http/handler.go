// go-server/internal/http/handler.go
package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"rag-go-server/internal/model"
	"rag-go-server/internal/rag"
)

// MakeRagHandler 返回 Gin 的 HandlerFunc
func MakeRagHandler(service *rag.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. 读取请求体
		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusInternalServerError, model.RagResponse{
				Status: "error",
				Data:   map[string]interface{}{"message": "读取请求数据失败"},
			})
			return
		}

		// 2. JSON 反序列化
		var req model.RagRequest
		if err := json.Unmarshal(bodyBytes, &req); err != nil {
			c.JSON(http.StatusBadRequest, model.RagResponse{
				Status: "error",
				Data:   map[string]interface{}{"message": err.Error()},
			})
			return
		}

		// 3. 获取设备指纹
		fingerprint := c.GetHeader("X-Device-Fingerprint")
		if fingerprint == "" {
			c.JSON(http.StatusBadRequest, model.RagResponse{
				Status: "error",
				Data:   map[string]interface{}{"message": "缺少设备指纹"},
			})
			return
		}

		// 4. 调用 RagService
		recs, err := service.HandleRag(c.Request.Context(), req, fingerprint)
		if err != nil {
			status := http.StatusInternalServerError
			if strings.Contains(err.Error(), "访问次数已用完") {
				status = http.StatusTooManyRequests
			}
			c.JSON(status, model.RagResponse{
				Status: "error",
				Data:   map[string]interface{}{"message": err.Error()},
			})
			return
		}

		// 5. 组装响应
		responseData := make([]map[string]interface{}, 0, len(recs))
		for _, r := range recs {
			responseData = append(responseData, map[string]interface{}{
				"course": r.Course,
				"reason": r.Reason,
			})
		}

		c.JSON(http.StatusOK, model.RagResponse{
			Status: "success",
			Data: map[string]interface{}{
				"recommendations": responseData,
			},
		})
	}
}
