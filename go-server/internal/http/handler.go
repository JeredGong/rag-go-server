// go-server/internal/http/handler.go
// 提供对外的 Gin HTTP handler，负责验证请求、调用 RAG 服务并返回结构化响应。
package httpapi

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"rag-go-server/internal/model"
	"rag-go-server/internal/rag"
)

// MakeRagHandler 返回 Gin 的 HandlerFunc
// 通过闭包注入 rag.Service，便于在路由层统一注册。
func MakeRagHandler(service *rag.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. 读取请求体
		// 读取到的原始字节随后用于 JSON 解析，若失败直接返回 500。
		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusInternalServerError, model.RagResponse{
				Status: "error",
				Data:   map[string]interface{}{"message": "读取请求数据失败"},
			})
			return
		}

		// 2. JSON 反序列化
		// 将 body 解析为 RagRequest，校验必要字段。
		var req model.RagRequest
		if err := json.Unmarshal(bodyBytes, &req); err != nil {
			log.Printf("⚠️ 请求体解析失败: %v", err)
			c.JSON(http.StatusBadRequest, model.RagResponse{
				Status: "error",
				Data:   map[string]interface{}{"message": err.Error()},
			})
			return
		}

		// 3. 获取设备指纹
		// fingerprint 用于限流，缺失直接判为 Bad Request。
		fingerprint := c.GetHeader("X-Device-Fingerprint")
		if fingerprint == "" {
			log.Println("⚠️ 请求缺少设备指纹")
			c.JSON(http.StatusBadRequest, model.RagResponse{
				Status: "error",
				Data:   map[string]interface{}{"message": "缺少设备指纹"},
			})
			return
		}

		// 4. 调用 RagService
		// 将请求上下文、参数和指纹交给 RAG 处理链，任何错误都映射为 HTTP 错误。
		recs, err := service.HandleRag(c.Request.Context(), req, fingerprint)
		if err != nil {
			status := http.StatusInternalServerError
			if strings.Contains(err.Error(), "访问次数已用完") {
				status = http.StatusTooManyRequests
			}
			log.Printf("❌ RAG 处理失败 (设备: %s): %v", fingerprint, err)
			c.JSON(status, model.RagResponse{
				Status: "error",
				Data:   map[string]interface{}{"message": err.Error()},
			})
			return
		}

		// 5. 组装响应
		// 将内部结构转换为轻量 map，方便前端展示。
		responseData := make([]map[string]interface{}, 0, len(recs))
		for _, r := range recs {
			responseData = append(responseData, map[string]interface{}{
				"course": r.Course,
				"reason": r.Reason,
			})
		}

		// 返回统一的 RagResponse，status 字段标记成功。
		c.JSON(http.StatusOK, model.RagResponse{
			Status: "success",
			Data: map[string]interface{}{
				"recommendations": responseData,
			},
		})
	}
}
