// go-server/internal/vectorstore/qdrant_store.go
package vectorstore

import (
	"context"
	"log"

	"github.com/qdrant/go-client/qdrant"
)

// Store 抽象：向量检索
type Store interface {
	Search(ctx context.Context, vector []float32, catagory int, limit int) ([]map[string]interface{}, error)
}

// QdrantStore 使用 Qdrant 作为向量数据库
type QdrantStore struct {
	Client         *qdrant.Client
	CollectionName string
}

func NewQdrantStore(client *qdrant.Client, collection string) *QdrantStore {
	return &QdrantStore{
		Client:         client,
		CollectionName: collection,
	}
}

func (s *QdrantStore) Search(ctx context.Context, vector []float32, catagory int, limit int) ([]map[string]interface{}, error) {
	log.Println("开始构造 Qdrant 查询请求...")

	lim := uint64(limit)
	query := qdrant.NewQuery(vector...)
	req := &qdrant.QueryPoints{
		CollectionName: s.CollectionName,
		Query:          query,
		Limit:          &lim,
		WithPayload:    qdrant.NewWithPayload(true),
	}

	log.Printf("查询请求: %+v\n", req)
	resp, err := s.Client.Query(ctx, req)
	if err != nil {
		log.Println("Qdrant 查询失败:", err)
		return nil, err
	}
	log.Println("Qdrant 查询成功，返回结果数量:", len(resp))

	var matches []map[string]interface{}
	for _, pt := range resp {
		payload := make(map[string]interface{})
		for k, v := range pt.Payload {
			switch val := v.Kind.(type) {
			case *qdrant.Value_StringValue:
				payload[k] = val.StringValue
			case *qdrant.Value_IntegerValue:
				payload[k] = int(val.IntegerValue)
			case *qdrant.Value_BoolValue:
				payload[k] = val.BoolValue
			case *qdrant.Value_DoubleValue:
				payload[k] = val.DoubleValue
			}
		}

		// 按 catagory 过滤（兼容 float64 / int）
		if catagory == 0 {
			matches = append(matches, payload)
		} else if val, ok := payload["catagory"]; ok {
			switch v := val.(type) {
			case float64:
				if int(v) == catagory {
					matches = append(matches, payload)
				}
			case int:
				if v == catagory {
					matches = append(matches, payload)
				}
			}
		}
	}

	log.Printf("找到 %d 个匹配的课程", len(matches))
	return matches, nil
}
