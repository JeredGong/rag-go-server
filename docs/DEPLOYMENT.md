# 部署指南

本文档提供 RAG Go Server 的详细部署说明，涵盖本地开发、测试和生产环境部署。

## 目录

- [环境准备](#环境准备)
- [本地部署](#本地部署)
- [生产部署](#生产部署)
- [Docker 部署](#docker-部署)
- [云服务配置](#云服务配置)
- [监控和日志](#监控和日志)
- [故障排查](#故障排查)

---

## 环境准备

### 系统要求

- **操作系统**: Linux / macOS / Windows
- **Go 版本**: 1.20 或更高
- **Python 版本**: 3.8 或更高（仅索引构建需要）
- **内存**: 最低 2GB RAM
- **磁盘**: 最低 10GB 可用空间

### 必需的外部服务

| 服务          | 用途           | 注册地址                                    |
| ------------- | -------------- | ------------------------------------------- |
| Qdrant Cloud  | 向量数据库     | https://cloud.qdrant.io                     |
| Redis         | 限流和缓存     | 本地安装或云服务（如 Redis Cloud）          |
| DeepSeek API  | 大语言模型     | https://platform.deepseek.com               |
| Cloudflare    | 向量嵌入服务   | https://workers.cloudflare.com              |

---

## 本地部署

### 步骤1：克隆项目

```bash
git clone https://github.com/your-username/rag-go-server.git
cd rag-go-server
```

### 步骤2：安装 Go 依赖

```bash
cd go-server
go mod download
```

### 步骤3：安装 Python 依赖（用于索引构建）

```bash
cd ../build_index
pip install -r requirements.txt
```

创建 `requirements.txt`：

```txt
faiss-cpu==1.7.4
qdrant-client==1.7.0
FlagEmbedding==1.2.5
pandas==2.0.3
numpy==1.24.3
tqdm==4.66.1
```

### 步骤4：配置环境变量

在项目根目录创建 `.env` 文件：

```env
# DeepSeek API 配置
OPENAI_API_KEY=sk-your-deepseek-api-key

# Qdrant 配置
QDRANT_HOST=your-cluster-id.us-west-1-0.aws.cloud.qdrant.io
QDRANT_API_KEY=your-qdrant-api-key

# Redis 配置
REDIS_HOST=127.0.0.1:6379
REDIS_PASSWORD=

# 向量嵌入服务
EMBED_ENDPOINT=https://your-worker.your-subdomain.workers.dev

# 服务配置
LISTEN_ADDR=127.0.0.1:8091
```

### 步骤5：启动 Redis

#### macOS (Homebrew)

```bash
brew install redis
brew services start redis
```

#### Ubuntu/Debian

```bash
sudo apt update
sudo apt install redis-server
sudo systemctl start redis-server
```

#### Docker

```bash
docker run -d --name redis -p 6379:6379 redis:7-alpine
```

### 步骤6：构建向量索引

```bash
cd build_index

# 构建本地 FAISS 索引
python build_db.py --csv CouresesData.csv --db ./db

# 上传到 Qdrant Cloud
# 需要先在 push_cloud.py 中配置 Qdrant 连接信息
python push_cloud.py
```

### 步骤7：启动服务

```bash
cd ../go-server
go run main.go
```

看到以下输出表示启动成功：

```
✅ Qdrant 客户端初始化成功
✅ Redis 初始化成功
🚀 RAG 服务启动，监听地址: 127.0.0.1:8091
```

### 步骤8：测试接口

```bash
curl -X POST http://127.0.0.1:8091/rag \
  -H "X-Device-Fingerprint: test-device-123" \
  -H "Content-Type: application/json" \
  -d '{
    "userQuestion": "推荐一些轻松的课程",
    "catagory": 0
  }'
```

---

## 生产部署

### 架构建议

```
                    ┌──────────────┐
                    │   CDN/WAF    │
                    │  (Cloudflare)│
                    └──────┬───────┘
                           │
                    ┌──────▼───────┐
                    │ Load Balancer│
                    │    (Nginx)   │
                    └──────┬───────┘
                           │
           ┌───────────────┼───────────────┐
           │               │               │
      ┌────▼───┐      ┌───▼────┐     ┌───▼────┐
      │ Go App │      │ Go App │     │ Go App │
      │  Node1 │      │  Node2 │     │  Node3 │
      └────┬───┘      └───┬────┘     └───┬────┘
           │              │               │
           └──────────────┼───────────────┘
                          │
              ┌───────────▼──────────┐
              │  Redis / Qdrant      │
              │  (Managed Services)  │
              └──────────────────────┘
```

### 步骤1：准备服务器

**推荐配置**：
- **CPU**: 2 核或更高
- **内存**: 4GB 或更高
- **系统**: Ubuntu 22.04 LTS

### 步骤2：安装系统依赖

```bash
# 更新系统
sudo apt update && sudo apt upgrade -y

# 安装基础工具
sudo apt install -y git curl wget vim

# 安装 Go（如果未安装）
wget https://go.dev/dl/go1.21.0.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.21.0.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc
```

### 步骤3：部署应用

```bash
# 创建应用目录
sudo mkdir -p /opt/rag-go-server
sudo chown $USER:$USER /opt/rag-go-server

# 克隆代码
cd /opt/rag-go-server
git clone https://github.com/your-username/rag-go-server.git .

# 编译 Go 应用
cd go-server
go build -o rag-server main.go

# 创建配置文件
sudo vim /opt/rag-go-server/.env
```

### 步骤4：创建 systemd 服务

创建 `/etc/systemd/system/rag-server.service`：

```ini
[Unit]
Description=RAG Go Server
After=network.target

[Service]
Type=simple
User=ubuntu
WorkingDirectory=/opt/rag-go-server/go-server
ExecStart=/opt/rag-go-server/go-server/rag-server
Restart=on-failure
RestartSec=5s

# 环境变量
EnvironmentFile=/opt/rag-go-server/.env

# 日志
StandardOutput=journal
StandardError=journal
SyslogIdentifier=rag-server

# 安全配置
NoNewPrivileges=true
PrivateTmp=true

[Install]
WantedBy=multi-user.target
```

启动服务：

```bash
sudo systemctl daemon-reload
sudo systemctl enable rag-server
sudo systemctl start rag-server

# 检查状态
sudo systemctl status rag-server
```

### 步骤5：配置 Nginx 反向代理

安装 Nginx：

```bash
sudo apt install -y nginx
```

创建配置文件 `/etc/nginx/sites-available/rag-server`：

```nginx
upstream rag_backend {
    # 如果有多个实例，在这里添加
    server 127.0.0.1:8091;
    # server 127.0.0.1:8092;
    # server 127.0.0.1:8093;
}

server {
    listen 80;
    server_name your-domain.com;

    # HTTPS 重定向
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name your-domain.com;

    # SSL 证书（使用 Let's Encrypt）
    ssl_certificate /etc/letsencrypt/live/your-domain.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/your-domain.com/privkey.pem;

    # SSL 配置
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;
    ssl_prefer_server_ciphers on;

    # 日志
    access_log /var/log/nginx/rag-server-access.log;
    error_log /var/log/nginx/rag-server-error.log;

    # 代理配置
    location /rag {
        proxy_pass http://rag_backend;
        proxy_http_version 1.1;
        
        # 请求头
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # 超时配置
        proxy_connect_timeout 10s;
        proxy_send_timeout 30s;
        proxy_read_timeout 30s;

        # 限流（可选）
        limit_req zone=api_limit burst=20 nodelay;
    }

    # 健康检查端点
    location /health {
        access_log off;
        return 200 "OK\n";
        add_header Content-Type text/plain;
    }
}

# 限流配置
limit_req_zone $binary_remote_addr zone=api_limit:10m rate=10r/s;
```

启用配置：

```bash
sudo ln -s /etc/nginx/sites-available/rag-server /etc/nginx/sites-enabled/
sudo nginx -t
sudo systemctl restart nginx
```

### 步骤6：配置 SSL 证书（Let's Encrypt）

```bash
sudo apt install -y certbot python3-certbot-nginx

# 获取证书
sudo certbot --nginx -d your-domain.com

# 自动续期（证书有效期 90 天）
sudo certbot renew --dry-run
```

### 步骤7：配置防火墙

```bash
# 开放 HTTP 和 HTTPS
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp

# 如果使用 SSH
sudo ufw allow 22/tcp

# 启用防火墙
sudo ufw enable
```

---

## Docker 部署

### Dockerfile

创建 `go-server/Dockerfile`：

```dockerfile
# 构建阶段
FROM golang:1.21-alpine AS builder

WORKDIR /app

# 复制 go mod 文件
COPY go.mod go.sum ./
RUN go mod download

# 复制源代码
COPY . .

# 编译
RUN CGO_ENABLED=0 GOOS=linux go build -o rag-server main.go

# 运行阶段
FROM alpine:3.18

# 安装 CA 证书（HTTPS 请求需要）
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /app/rag-server .

# 暴露端口
EXPOSE 8091

# 运行
CMD ["./rag-server"]
```

### docker-compose.yml

```yaml
version: '3.8'

services:
  rag-server:
    build: ./go-server
    ports:
      - "8091:8091"
    environment:
      - OPENAI_API_KEY=${OPENAI_API_KEY}
      - QDRANT_HOST=${QDRANT_HOST}
      - QDRANT_API_KEY=${QDRANT_API_KEY}
      - REDIS_HOST=redis:6379
      - REDIS_PASSWORD=
      - EMBED_ENDPOINT=${EMBED_ENDPOINT}
      - LISTEN_ADDR=0.0.0.0:8091
    depends_on:
      - redis
    restart: unless-stopped
    networks:
      - rag-network

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    volumes:
      - redis-data:/data
    command: redis-server --appendonly yes
    restart: unless-stopped
    networks:
      - rag-network

  nginx:
    image: nginx:1.25-alpine
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./nginx.conf:/etc/nginx/nginx.conf:ro
      - ./ssl:/etc/nginx/ssl:ro
    depends_on:
      - rag-server
    restart: unless-stopped
    networks:
      - rag-network

volumes:
  redis-data:

networks:
  rag-network:
    driver: bridge
```

### 构建和运行

```bash
# 构建镜像
docker-compose build

# 启动服务
docker-compose up -d

# 查看日志
docker-compose logs -f rag-server

# 停止服务
docker-compose down
```

---

## 云服务配置

### Qdrant Cloud

1. **注册账号**：https://cloud.qdrant.io/login
2. **创建集群**：选择地区（建议选择离用户最近的）
3. **创建集合**：

```python
from qdrant_client import QdrantClient
from qdrant_client.models import Distance, VectorParams

client = QdrantClient(
    url="https://xxx.us-west-1-0.aws.cloud.qdrant.io:6333",
    api_key="your-api-key"
)

client.create_collection(
    collection_name="WHUCoursesDB",
    vectors_config=VectorParams(
        size=1024,          # BGE-M3 向量维度
        distance=Distance.COSINE
    )
)
```

4. **上传数据**：运行 `build_index/push_cloud.py`

### Redis Cloud（可选）

如果不想自己维护 Redis：

1. **注册**：https://redis.com/try-free/
2. **创建数据库**
3. **获取连接信息**：
   - Endpoint: `redis-xxxxx.cloud.redislabs.com:12345`
   - Password: `your-password`
4. **更新 `.env`**：

```env
REDIS_HOST=redis-xxxxx.cloud.redislabs.com:12345
REDIS_PASSWORD=your-password
```

### Cloudflare Worker（向量嵌入）

1. **安装 Wrangler CLI**：

```bash
npm install -g wrangler
wrangler login
```

2. **创建 Worker**：

```bash
wrangler init embedding-worker
cd embedding-worker
```

3. **编写 Worker 代码**（`src/index.ts`）：

```typescript
import { BGEM3FlagModel } from '@flagopen/flag-embedding';

export default {
  async fetch(request: Request): Promise<Response> {
    if (request.method !== 'POST') {
      return new Response('Method Not Allowed', { status: 405 });
    }

    try {
      const { text } = await request.json();
      
      // 加载模型（仅首次调用时）
      const model = await BGEM3FlagModel.load();
      const embedding = await model.encode(text);

      return new Response(JSON.stringify({
        embedding: { data: [embedding] }
      }), {
        headers: { 'Content-Type': 'application/json' }
      });
    } catch (error) {
      return new Response(JSON.stringify({ error: error.message }), {
        status: 500,
        headers: { 'Content-Type': 'application/json' }
      });
    }
  }
};
```

4. **部署**：

```bash
wrangler publish
```

---

## 监控和日志

### 日志收集

#### 使用 journald（systemd）

```bash
# 查看实时日志
sudo journalctl -u rag-server -f

# 查看最近 100 条日志
sudo journalctl -u rag-server -n 100

# 查看今天的日志
sudo journalctl -u rag-server --since today

# 导出日志
sudo journalctl -u rag-server > rag-server.log
```

#### 使用文件日志

修改代码，将日志输出到文件：

```go
logFile, err := os.OpenFile("/var/log/rag-server.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
if err != nil {
    log.Fatal(err)
}
log.SetOutput(logFile)
```

### 性能监控

#### Prometheus + Grafana

1. **安装 Prometheus 客户端**：

```bash
go get github.com/prometheus/client_golang/prometheus
go get github.com/prometheus/client_golang/prometheus/promhttp
```

2. **添加监控指标**：

```go
var (
    requestCounter = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "rag_requests_total",
            Help: "Total number of RAG requests",
        },
        []string{"status"},
    )
    
    requestDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "rag_request_duration_seconds",
            Help: "RAG request duration",
        },
        []string{"endpoint"},
    )
)

func init() {
    prometheus.MustRegister(requestCounter)
    prometheus.MustRegister(requestDuration)
}
```

3. **暴露 metrics 端点**：

```go
r.GET("/metrics", gin.WrapH(promhttp.Handler()))
```

---

## 故障排查

### 常见问题

#### 1. 无法连接 Qdrant

**症状**：

```
❌ Qdrant 初始化失败: connection refused
```

**排查步骤**：
1. 检查 Qdrant URL 和 API Key 是否正确
2. 测试网络连接：`curl https://your-qdrant-host:6333/health`
3. 检查防火墙规则
4. 验证 Qdrant Cloud 集群状态

#### 2. Redis 连接失败

**症状**：

```
❌ Redis 初始化失败: dial tcp 127.0.0.1:6379: connect: connection refused
```

**排查步骤**：
1. 检查 Redis 是否启动：`redis-cli ping`
2. 检查配置：`REDIS_HOST` 和 `REDIS_PASSWORD`
3. 查看 Redis 日志：`sudo journalctl -u redis`

#### 3. DeepSeek API 调用失败

**症状**：

```
LLM API 调用失败: 401 Unauthorized
```

**排查步骤**：
1. 验证 `OPENAI_API_KEY` 是否正确
2. 检查 API 配额是否用尽
3. 测试 API：

```bash
curl -X POST https://api.deepseek.com/chat/completions \
  -H "Authorization: Bearer $OPENAI_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model": "deepseek-chat", "messages": [{"role": "user", "content": "Hi"}]}'
```

#### 4. 服务响应慢

**排查步骤**：
1. 检查各环节耗时（查看日志）
2. 监控资源使用：`htop`、`free -h`
3. 检查网络延迟：`ping your-qdrant-host`
4. 优化查询参数（减少 `limit`）

### 调试技巧

#### 启用详细日志

```go
gin.SetMode(gin.DebugMode)
```

#### 使用 curl 测试

```bash
# 添加 -v 查看详细信息
curl -v -X POST http://127.0.0.1:8091/rag \
  -H "X-Device-Fingerprint: test-123" \
  -H "Content-Type: application/json" \
  -d '{"userQuestion": "test", "catagory": 0}'
```

#### 检查环境变量

```bash
# 在服务器上
printenv | grep -E 'OPENAI|QDRANT|REDIS'
```

---

## 附录

### 环境变量完整列表

| 变量名           | 必填 | 默认值                                         | 说明                      |
| ---------------- | ---- | ---------------------------------------------- | ------------------------- |
| OPENAI_API_KEY   | 是   | -                                              | DeepSeek API 密钥         |
| QDRANT_HOST      | 是   | -                                              | Qdrant 主机地址           |
| QDRANT_API_KEY   | 是   | -                                              | Qdrant API 密钥           |
| REDIS_HOST       | 否   | 127.0.0.1:6379                                 | Redis 地址                |
| REDIS_PASSWORD   | 否   | ""                                             | Redis 密码                |
| EMBED_ENDPOINT   | 否   | https://whuworkers.jeredgong.workers.dev       | 向量嵌入服务地址          |
| LISTEN_ADDR      | 否   | 127.0.0.1:8091                                 | HTTP 监听地址             |

### 端口列表

| 端口  | 服务              | 说明                  |
| ----- | ----------------- | --------------------- |
| 8091  | Go HTTP Server    | 主服务端口            |
| 6379  | Redis             | 限流和缓存            |
| 6333  | Qdrant HTTP       | Qdrant HTTP API       |
| 6334  | Qdrant gRPC       | Qdrant gRPC API       |
| 80    | Nginx HTTP        | HTTP 访问（重定向）   |
| 443   | Nginx HTTPS       | HTTPS 访问            |

### 有用的命令

```bash
# 查看服务状态
sudo systemctl status rag-server

# 重启服务
sudo systemctl restart rag-server

# 查看实时日志
sudo journalctl -u rag-server -f

# 测试 Nginx 配置
sudo nginx -t

# 重载 Nginx 配置
sudo nginx -s reload

# 查看端口占用
sudo netstat -tulpn | grep :8091

# 查看进程
ps aux | grep rag-server
```

