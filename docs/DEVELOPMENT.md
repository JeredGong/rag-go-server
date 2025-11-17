# 开发者指南

本文档为开发者提供项目的开发环境搭建、代码规范、测试方法和贡献指南。

## 目录

- [开发环境搭建](#开发环境搭建)
- [项目结构](#项目结构)
- [代码规范](#代码规范)
- [开发工作流](#开发工作流)
- [测试指南](#测试指南)
- [调试技巧](#调试技巧)
- [性能分析](#性能分析)
- [贡献指南](#贡献指南)

---

## 开发环境搭建

### 前置要求

| 工具          | 版本要求   | 用途                     |
| ------------- | ---------- | ------------------------ |
| Go            | 1.20+      | 后端开发                 |
| Python        | 3.8+       | 索引构建脚本             |
| Git           | 2.0+       | 版本控制                 |
| Redis         | 6.0+       | 本地测试                 |
| VS Code       | 最新版     | 推荐 IDE                 |

### IDE 配置

#### VS Code 推荐插件

```json
{
  "recommendations": [
    "golang.go",              // Go 语言支持
    "ms-python.python",       // Python 语言支持
    "eamodio.gitlens",        // Git 增强
    "editorconfig.editorconfig", // 编辑器配置
    "streetsidesoftware.code-spell-checker", // 拼写检查
    "ms-azuretools.vscode-docker" // Docker 支持
  ]
}
```

将上述内容保存到 `.vscode/extensions.json`。

#### VS Code 设置

创建 `.vscode/settings.json`：

```json
{
  "go.useLanguageServer": true,
  "go.lintTool": "golangci-lint",
  "go.lintOnSave": "package",
  "go.formatTool": "goimports",
  "editor.formatOnSave": true,
  "go.testOnSave": false,
  "[go]": {
    "editor.codeActionsOnSave": {
      "source.organizeImports": true
    }
  },
  "python.linting.enabled": true,
  "python.linting.pylintEnabled": true,
  "python.formatting.provider": "black"
}
```

### 安装 Go 工具链

```bash
# 安装 linter
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# 安装 goimports（自动整理导入）
go install golang.org/x/tools/cmd/goimports@latest

# 安装 delve（调试器）
go install github.com/go-delve/delve/cmd/dlv@latest
```

### 克隆和初始化项目

```bash
# 克隆仓库
git clone https://github.com/your-username/rag-go-server.git
cd rag-go-server

# 安装 Go 依赖
cd go-server
go mod download

# 安装 Python 依赖（可选，用于索引构建）
cd ../build_index
pip install -r requirements.txt
```

### 配置本地环境

复制环境变量模板：

```bash
cp .env.example .env
```

编辑 `.env`，填入你的配置：

```env
OPENAI_API_KEY=sk-test-key
QDRANT_HOST=localhost
QDRANT_API_KEY=test-key
REDIS_HOST=127.0.0.1:6379
REDIS_PASSWORD=
EMBED_ENDPOINT=http://localhost:8080
LISTEN_ADDR=127.0.0.1:8091
```

### 运行本地服务

#### 方法1：直接运行

```bash
cd go-server
go run main.go
```

#### 方法2：使用 Make（推荐）

创建 `Makefile`：

```makefile
.PHONY: run build test lint clean

# 运行服务
run:
	cd go-server && go run main.go

# 构建二进制
build:
	cd go-server && go build -o bin/rag-server main.go

# 运行测试
test:
	cd go-server && go test ./... -v -cover

# 代码检查
lint:
	cd go-server && golangci-lint run

# 清理
clean:
	cd go-server && rm -rf bin/
```

使用：

```bash
make run    # 运行服务
make build  # 构建
make test   # 测试
make lint   # 代码检查
```

---

## 项目结构

```
rag-go-server/
├── go-server/                  # Go 后端服务
│   ├── main.go                 # 服务入口
│   ├── internal/               # 内部包（不对外暴露）
│   │   ├── config/             # 配置管理
│   │   │   └── config.go
│   │   ├── embedding/          # 向量嵌入
│   │   │   └── cloudflare.go
│   │   ├── http/               # HTTP 处理
│   │   │   └── handler.go
│   │   ├── limit/              # 限流
│   │   │   └── redis_limiter.go
│   │   ├── llm/                # 大语言模型
│   │   │   └── deepseek.go
│   │   ├── model/              # 数据模型
│   │   │   └── model.go
│   │   ├── rag/                # RAG 服务
│   │   │   └── service.go
│   │   └── vectorstore/        # 向量存储
│   │       └── qdrant_store.go
│   ├── go.mod                  # Go 依赖管理
│   └── go.sum                  # 依赖校验
│
├── build_index/                # 索引构建脚本
│   ├── build_db.py             # 构建 FAISS 索引
│   ├── embedding.py            # BGE-M3 向量化
│   ├── push_cloud.py           # 上传到 Qdrant
│   ├── CouresesData.csv        # 课程数据（示例）
│   └── requirements.txt        # Python 依赖
│
├── docs/                       # 文档目录
│   ├── API.md                  # API 文档
│   ├── ARCHITECTURE.md         # 架构设计
│   ├── DEPLOYMENT.md           # 部署指南
│   └── DEVELOPMENT.md          # 本文档
│
├── .env.example                # 环境变量模板
├── .gitignore                  # Git 忽略规则
├── README.md                   # 项目说明
└── LICENSE                     # 开源协议
```

### 目录职责

| 目录/文件          | 职责                                       |
| ------------------ | ------------------------------------------ |
| `go-server/`       | Go 后端服务的所有代码                      |
| `internal/`        | 内部包，遵循 Go 的 internal 约定           |
| `build_index/`     | Python 脚本，用于离线构建向量索引          |
| `docs/`            | 项目文档                                   |
| `.env.example`     | 环境变量配置模板                           |

---

## 代码规范

### Go 代码规范

#### 1. 命名规范

```go
// ✅ 推荐
type RagRequest struct {
    UserQuestion string `json:"userQuestion"`
}

func HandleRag(ctx context.Context, req RagRequest) error {
    // ...
}

// ❌ 不推荐
type ragRequest struct {  // 导出类型应大写
    user_question string  // Go 使用驼峰命名
}

func handle_rag(ctx context.Context, req ragRequest) error {  // 函数名应驼峰
    // ...
}
```

#### 2. 错误处理

```go
// ✅ 推荐：使用 fmt.Errorf 包装错误
if err := doSomething(); err != nil {
    return nil, fmt.Errorf("执行 doSomething 失败: %w", err)
}

// ❌ 不推荐：直接返回错误
if err := doSomething(); err != nil {
    return nil, err
}
```

#### 3. 上下文传递

```go
// ✅ 推荐：第一个参数为 context.Context
func (s *Service) HandleRag(ctx context.Context, req RagRequest) error {
    // 传递 context 到下游
    vec, err := s.Embedder.Embed(ctx, req.UserQuestion)
    // ...
}

// ❌ 不推荐：不使用 context
func (s *Service) HandleRag(req RagRequest) error {
    vec, err := s.Embedder.Embed(req.UserQuestion)
    // ...
}
```

#### 4. 注释规范

```go
// Package rag 提供 RAG（Retrieval-Augmented Generation）服务。
package rag

// Service 封装完整 RAG 处理链条。
//
// 工作流程：
// 1. 限流检查
// 2. 向量化
// 3. 向量检索
// 4. LLM 生成
// 5. 结果解析
type Service struct {
    Embedder    embedding.Client
    VectorStore vectorstore.Store
    LLM         llm.Client
    Limiter     limit.RateLimiter
}

// HandleRag 运行完整的 RAG 流程。
//
// 参数：
//   - ctx: 上下文对象
//   - req: RAG 请求
//   - fingerprint: 设备指纹
//
// 返回值：
//   - []model.CourseRecommendation: 推荐结果
//   - error: 错误信息
func (s *Service) HandleRag(ctx context.Context, req model.RagRequest, fingerprint string) ([]model.CourseRecommendation, error) {
    // 实现...
}
```

#### 5. 接口设计

```go
// ✅ 推荐：小而精的接口
type Embedder interface {
    Embed(ctx context.Context, text string) ([]float32, error)
}

// ❌ 不推荐：臃肿的接口
type AIService interface {
    Embed(ctx context.Context, text string) ([]float32, error)
    Classify(ctx context.Context, text string) (string, error)
    Summarize(ctx context.Context, text string) (string, error)
    // ...（太多方法）
}
```

### Python 代码规范

遵循 [PEP 8](https://pep8.org/) 规范：

```python
# ✅ 推荐
def build_rag_database(csv_path: str, db_path: str, embedding_dim: int = 1024):
    """
    构建 RAG 向量数据库。

    参数：
        csv_path: 输入 CSV 文件路径
        db_path: 输出数据库目录
        embedding_dim: 向量维度
    """
    pass

# ❌ 不推荐
def BuildRagDatabase(csvPath, dbPath, embeddingDim=1024):  # 驼峰命名不符合 Python 规范
    pass
```

### 提交规范

遵循 [Conventional Commits](https://www.conventionalcommits.org/)：

```bash
# 格式
<type>(<scope>): <subject>

# 示例
feat(rag): 添加多轮对话支持
fix(limit): 修复限流器并发问题
docs(api): 更新 API 文档
refactor(llm): 重构 LLM 客户端代码
test(handler): 添加 HTTP 处理器测试
chore(deps): 升级依赖版本
```

**Type 类型**：
- `feat`: 新功能
- `fix`: Bug 修复
- `docs`: 文档更新
- `refactor`: 代码重构
- `test`: 测试相关
- `chore`: 构建、工具、依赖等

---

## 开发工作流

### 1. 创建功能分支

```bash
git checkout -b feature/add-caching
```

### 2. 开发和测试

```bash
# 运行服务
make run

# 在另一个终端测试
curl -X POST http://127.0.0.1:8091/rag \
  -H "X-Device-Fingerprint: test-123" \
  -H "Content-Type: application/json" \
  -d '{"userQuestion": "test", "catagory": 0}'
```

### 3. 代码检查

```bash
# Go 代码检查
make lint

# 修复自动可修复的问题
cd go-server
golangci-lint run --fix
```

### 4. 运行测试

```bash
make test
```

### 5. 提交代码

```bash
git add .
git commit -m "feat(cache): 添加 Redis 缓存层"
git push origin feature/add-caching
```

### 6. 创建 Pull Request

在 GitHub 上创建 PR，填写：
- **标题**: 简洁描述
- **描述**: 详细说明变更内容
- **关联 Issue**: 如果有相关 Issue

---

## 测试指南

### 单元测试

#### 测试文件命名

```bash
handler.go       # 源文件
handler_test.go  # 测试文件
```

#### 测试示例

```go
// go-server/internal/rag/service_test.go
package rag

import (
    "context"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
)

// Mock Embedder
type MockEmbedder struct {
    mock.Mock
}

func (m *MockEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
    args := m.Called(ctx, text)
    return args.Get(0).([]float32), args.Error(1)
}

func TestService_HandleRag(t *testing.T) {
    // 创建 mock 对象
    mockEmbedder := new(MockEmbedder)
    mockEmbedder.On("Embed", mock.Anything, "test question").Return([]float32{0.1, 0.2}, nil)

    // 创建 service
    service := NewService(mockEmbedder, nil, nil, nil)

    // 测试
    req := model.RagRequest{
        UserQuestion: "test question",
        Catagory:     0,
    }
    _, err := service.HandleRag(context.Background(), req, "test-fp")

    // 断言
    assert.NoError(t, err)
    mockEmbedder.AssertExpectations(t)
}
```

### 运行测试

```bash
# 运行所有测试
go test ./...

# 带覆盖率
go test ./... -cover

# 详细输出
go test ./... -v

# 运行特定测试
go test ./internal/rag -run TestService_HandleRag
```

### 集成测试

创建 `go-server/tests/integration_test.go`：

```go
// +build integration

package tests

import (
    "context"
    "testing"
    "time"

    "github.com/redis/go-redis/v9"
    "github.com/stretchr/testify/assert"
)

func TestRedisConnection(t *testing.T) {
    // 需要本地运行 Redis
    rdb := redis.NewClient(&redis.Options{
        Addr: "127.0.0.1:6379",
    })
    defer rdb.Close()

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    // 测试连接
    err := rdb.Ping(ctx).Err()
    assert.NoError(t, err)
}
```

运行集成测试：

```bash
go test ./tests -tags=integration -v
```

### E2E 测试

使用 `httptest` 测试完整请求流程：

```go
func TestRAGEndpoint(t *testing.T) {
    // 创建测试服务器
    router := gin.Default()
    router.POST("/rag", httpapi.MakeRagHandler(ragService))

    // 构造请求
    body := `{"userQuestion": "test", "catagory": 0}`
    req, _ := http.NewRequest("POST", "/rag", strings.NewReader(body))
    req.Header.Set("X-Device-Fingerprint", "test-123")
    req.Header.Set("Content-Type", "application/json")

    // 发送请求
    w := httptest.NewRecorder()
    router.ServeHTTP(w, req)

    // 验证响应
    assert.Equal(t, 200, w.Code)
    assert.Contains(t, w.Body.String(), "success")
}
```

---

## 调试技巧

### 使用 Delve 调试器

#### 安装 Delve

```bash
go install github.com/go-delve/delve/cmd/dlv@latest
```

#### VS Code 调试配置

创建 `.vscode/launch.json`：

```json
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "Debug Go Server",
      "type": "go",
      "request": "launch",
      "mode": "auto",
      "program": "${workspaceFolder}/go-server/main.go",
      "env": {
        "OPENAI_API_KEY": "sk-test",
        "QDRANT_HOST": "localhost",
        "REDIS_HOST": "127.0.0.1:6379"
      },
      "args": []
    }
  ]
}
```

按 `F5` 启动调试。

#### 命令行调试

```bash
cd go-server
dlv debug main.go

# 在 dlv 提示符中
(dlv) break main.main
(dlv) continue
(dlv) next
(dlv) print cfg
```

### 日志调试

#### 添加调试日志

```go
log.Printf("🐛 [DEBUG] 用户问题: %s", req.UserQuestion)
log.Printf("🐛 [DEBUG] 向量维度: %d", len(vec))
log.Printf("🐛 [DEBUG] 检索到 %d 门课程", len(courses))
```

#### 条件日志

```go
const DEBUG = true

if DEBUG {
    log.Printf("🐛 [DEBUG] 详细信息: %+v", obj)
}
```

### 网络调试

#### 使用 curl 测试

```bash
curl -v -X POST http://127.0.0.1:8091/rag \
  -H "X-Device-Fingerprint: test-123" \
  -H "Content-Type: application/json" \
  -d '{"userQuestion": "test", "catagory": 0}' \
  | jq '.'
```

#### 使用 Postman

1. 导入 API 集合（如有）
2. 设置环境变量
3. 发送请求并查看响应

---

## 性能分析

### Go pprof

#### 启用 pprof

```go
import _ "net/http/pprof"

func main() {
    go func() {
        log.Println(http.ListenAndServe("localhost:6060", nil))
    }()
    
    // ... 启动主服务
}
```

#### 分析 CPU

```bash
# 采集 30 秒 CPU profile
curl http://localhost:6060/debug/pprof/profile?seconds=30 > cpu.prof

# 分析
go tool pprof cpu.prof

# 在 pprof 提示符中
(pprof) top10
(pprof) web  # 生成可视化图表（需要安装 graphviz）
```

#### 分析内存

```bash
curl http://localhost:6060/debug/pprof/heap > mem.prof
go tool pprof mem.prof
```

### 基准测试

```go
func BenchmarkParseDict(b *testing.B) {
    row := map[string]interface{}{
        "课程名称": "计算机网络",
        "授课老师": "张三",
        // ...
    }

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        parseDict(row)
    }
}
```

运行基准测试：

```bash
go test -bench=. -benchmem
```

---

## 贡献指南

### 报告 Bug

在 GitHub Issues 中创建新 Issue，包含：

1. **Bug 描述**：清晰描述问题
2. **复现步骤**：详细的复现方法
3. **预期行为**：应该发生什么
4. **实际行为**：实际发生了什么
5. **环境信息**：
   - 操作系统
   - Go 版本
   - 相关配置

### 提交 Pull Request

1. **Fork 仓库**
2. **创建分支**：`git checkout -b feature/my-feature`
3. **开发和测试**
4. **提交代码**：遵循提交规范
5. **推送分支**：`git push origin feature/my-feature`
6. **创建 PR**：在 GitHub 上创建 Pull Request

### PR 检查清单

- [ ] 代码通过 linter 检查
- [ ] 所有测试通过
- [ ] 添加了必要的测试
- [ ] 更新了相关文档
- [ ] 提交信息清晰规范
- [ ] 代码有适当的注释

### 代码审查

PR 会由维护者审查，可能会收到反馈：
- **Approve**: 可以合并
- **Request Changes**: 需要修改
- **Comment**: 一般性意见

根据反馈修改后，推送新的提交即可。

---

## 常用工具

### Go 工具

| 工具             | 用途                 | 安装命令                                           |
| ---------------- | -------------------- | -------------------------------------------------- |
| golangci-lint    | 代码检查             | `go install github.com/golangci/golangci-lint/...` |
| goimports        | 自动整理导入         | `go install golang.org/x/tools/cmd/goimports@...`  |
| gotests          | 生成测试骨架         | `go install github.com/cweill/gotests/...`         |
| dlv              | 调试器               | `go install github.com/go-delve/delve/cmd/dlv@...` |

### Python 工具

| 工具        | 用途         | 安装命令             |
| ----------- | ------------ | -------------------- |
| black       | 代码格式化   | `pip install black`  |
| pylint      | 代码检查     | `pip install pylint` |
| pytest      | 测试框架     | `pip install pytest` |

---

## 学习资源

### Go 学习

- [Go 官方文档](https://go.dev/doc/)
- [Effective Go](https://go.dev/doc/effective_go)
- [Go by Example](https://gobyexample.com/)

### RAG 相关

- [LangChain 文档](https://python.langchain.com/)
- [Qdrant 文档](https://qdrant.tech/documentation/)
- [BGE Embeddings](https://github.com/FlagOpen/FlagEmbedding)

---

## 联系方式

- **问题和建议**: [GitHub Issues](https://github.com/your-repo/issues)
- **讨论**: [GitHub Discussions](https://github.com/your-repo/discussions)
- **邮件**: dev@example.com

