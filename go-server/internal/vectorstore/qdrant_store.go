// go-server/internal/vectorstore/qdrant_store.go
// 该文件封装了对 Qdrant 的访问逻辑，提供统一的向量检索接口。
package vectorstore

import (
	"context"
	"log"

	"github.com/qdrant/go-client/qdrant"
)

// Store 抽象：向量检索
// interface 方便在测试中注入 mock，或未来接入不同的向量数据库实现。
type Store interface {
	Search(ctx context.Context, vector []float32, catagory int, limit int) ([]map[string]interface{}, error)
}

// QdrantStore 使用 Qdrant 作为向量数据库
type QdrantStore struct {
	// Client 是与 Qdrant 通信的 gRPC 客户端。
	Client         *qdrant.Client
	// CollectionName 指定查询的集合，通常在初始化时配置。
	CollectionName string
}

func NewQdrantStore(client *qdrant.Client, collection string) *QdrantStore {
	// 通过依赖注入客户端与集合名，提高复用性与可测试性。
	return &QdrantStore{
		Client:         client,
		CollectionName: collection,
	}
}

func (s *QdrantStore) Search(ctx context.Context, vector []float32, catagory int, limit int) ([]map[string]interface{}, error) {
	// Search 根据输入向量执行最近邻检索，并按分类进行过滤。
	log.Println("开始构造 Qdrant 查询请求...")

	lim := uint64(limit)
	// Qdrant SDK 的 QueryPoints 使用 uint64 表示 limit，需要转换。
	query := qdrant.NewQuery(vector...)
	req := &qdrant.QueryPoints{
		CollectionName: s.CollectionName,
		Query:          query,
		Limit:          &lim,
		WithPayload:    qdrant.NewWithPayload(true),
	}

	log.Printf("查询请求: %+v\n", req)
	// 发起请求前打印调试信息，便于观察 limit、collection 等参数。
	resp, err := s.Client.Query(ctx, req)
	if err != nil {
		log.Println("Qdrant 查询失败:", err)
		return nil, err
	}
	log.Println("Qdrant 查询成功，返回结果数量:", len(resp))

	var matches []map[string]interface{}
	// 逐条遍历返回的 points，将 payload 转成通用 map。
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
		// catagory=0 表示不限制分类，直接收集所有候选。
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

	// 记录最终匹配数，方便与 limit、原始召回量对照。
	log.Printf("找到 %d 个匹配的课程", len(matches))
	return matches, nil
}
