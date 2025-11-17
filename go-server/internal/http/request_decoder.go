package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"rag-go-server/internal/model"
)

const maxRequestBodyBytes int64 = 1 << 20 // 1 MiB

func decodeRagRequest(body io.Reader) (model.RagRequest, error) {
	limited := io.LimitReader(body, maxRequestBodyBytes+1)
	raw, err := io.ReadAll(limited)
	if err != nil {
		return model.RagRequest{}, fmt.Errorf("读取请求数据失败: %w", err)
	}
	if int64(len(raw)) > maxRequestBodyBytes {
		return model.RagRequest{}, model.NewValidationError("body", fmt.Sprintf("请求体大小不得超过 %d 字节", maxRequestBodyBytes))
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return model.RagRequest{}, model.NewValidationError("body", "请求体为空")
	}

	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()

	var req model.RagRequest
	if err := decoder.Decode(&req); err != nil {
		return model.RagRequest{}, model.NewValidationError("body", fmt.Sprintf("请求体解析失败: %v", err))
	}

	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return model.RagRequest{}, model.NewValidationError("body", "请求体包含多余数据")
		}
		return model.RagRequest{}, fmt.Errorf("解析请求失败: %w", err)
	}

	req.Normalize()
	if err := req.Validate(); err != nil {
		return model.RagRequest{}, err
	}
	return req, nil
}
