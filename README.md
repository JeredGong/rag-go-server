# 📚 WHU 课程推荐系统（RAG + Qdrant + Redis + DeepSeek）

本项目基于 Go + Gin 框架实现一个 RAG（Retrieval-Augmented Generation）系统，用于根据用户提问从向量数据库中检索课程并调用大语言模型（LLM）生成推荐。

## 🧠 项目结构

- 使用 Cloudflare Worker API 获取嵌入向量
- 向量数据存储在 Qdrant（支持相似度检索）
- Redis 用于设备访问频率限制（基于指纹）
- DeepSeek API 用于调用大语言模型生成回答

------

## ⚙️ 环境准备

### 必须安装：

- Go 1.20+
- Redis 服务
- Qdrant Cloud 实例（或本地 Qdrant）
- DeepSeek API Key（可兼容 OpenAI API 格式）

------

## 📦 安装依赖

```
bash


CopyEdit
go mod tidy
```

------

## 📁 `.env` 环境变量配置

项目根目录下新建 `.env` 文件，并填写以下内容：

```
envCopyEditOPENAI_API_KEY=sk-xxx                    # 你的 DeepSeek API Key
QDRANT_HOST=a7dcca84-xxx.cloud.qdrant.io
QDRANT_API_KEY=your_qdrant_key
REDIS_HOST=localhost:6379
REDIS_PASSWORD=                         # 如无密码则留空
```

------

## ▶️ 启动服务

```
bash


CopyEdit
go run main.go
```

默认监听地址为：

```
cpp


CopyEdit
http://127.0.0.1:8089
```

------

## 🧪 使用说明

### 请求地址

```
bash


CopyEdit
POST /rag
```

### 请求头（必需）

```
pgsqlCopyEditX-Device-Fingerprint: your-unique-device-id
Content-Type: application/json
```

### 请求体示例

```
jsonCopyEdit{
  "userQuestion": "我想选一些没有期末考试的课程",
  "catagory": 0
}
```

> `catagory = 0` 表示不限定课程分类。

------

### 成功响应示例

```
jsonCopyEdit{
  "status": "success",
  "data": {
    "recommendations": [
      {
        "course": "公共艺术赏析",
        "reason": "课程内容轻松，无期末考试"
      },
      ...
    ]
  }
}
```

### 错误响应示例

```
jsonCopyEdit{
  "status": "error",
  "data": {
    "message": "缺少设备指纹"
  }
}
```

------

## 🧠 功能亮点

- ✅ 支持课程向量检索（Qdrant）
- ✅ 支持用户访问频控（Redis）
- ✅ 支持 DeepSeek 多轮对话模型
- ✅ 支持设备指纹限制每周调用次数（默认 10 次）

------

## 📌 注意事项

1. 需要提前确保 Qdrant 数据库已填充课程向量数据。
2. DeepSeek API Key 应确保有效，并已启用对应模型。
3. 如果部署于公网，建议使用 Nginx 配置 TLS 和限速。