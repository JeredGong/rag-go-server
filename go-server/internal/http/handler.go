// go-server/internal/http/handler.go
package httpapi

import (
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"rag-go-server/internal/model"
	"rag-go-server/internal/rag"
)

// MakeRagHandler 返回 Gin 的 HandlerFunc
func MakeRagHandler(service *rag.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := RequestIDFromContext(c)
		req, err := decodeRagRequest(c.Request.Body)
		if err != nil {
			status := http.StatusBadRequest
			if !model.IsValidationError(err) {
				log.Printf("⚠️ (%s) 请求体解析失败: %v", requestID, err)
			}
			writeError(c, status, err.Error())
			return
		}

		fingerprint := c.GetHeader("X-Device-Fingerprint")
		if fingerprint == "" {
			log.Printf("⚠️ (%s) 请求缺少设备指纹", requestID)
			writeError(c, http.StatusBadRequest, "缺少设备指纹")
			return
		}

		recs, err := service.HandleRag(c.Request.Context(), req, fingerprint)
		if err != nil {
			status := http.StatusInternalServerError
			switch {
			case errors.Is(err, rag.ErrRateLimitExceeded):
				status = http.StatusTooManyRequests
			case model.IsValidationError(err):
				status = http.StatusBadRequest
			}
			log.Printf("❌ (%s) RAG 处理失败 (设备: %s): %v", requestID, fingerprint, err)
			writeError(c, status, err.Error())
			return
		}

		responseData := make([]map[string]interface{}, 0, len(recs))
		for _, r := range recs {
			responseData = append(responseData, map[string]interface{}{
				"course": r.Course,
				"reason": r.Reason,
			})
		}

		writeSuccess(c, map[string]interface{}{
			"recommendations": responseData,
		})
	}
}

func writeError(c *gin.Context, status int, message string) {
	c.JSON(status, model.RagResponse{
		Status: "error",
		Data: map[string]interface{}{
			"message": message,
		},
	})
}

func writeSuccess(c *gin.Context, data map[string]interface{}) {
	c.JSON(http.StatusOK, model.RagResponse{
		Status: "success",
		Data:   data,
	})
}
