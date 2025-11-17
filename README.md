# WHU 课程推荐系统（RAG + Qdrant + Redis + DeepSeek）

本项目是一个基于 **RAG（Retrieval-Augmented Generation）** 的课程推荐后端服务，使用 Go + Gin 实现。 
它可以根据用户的自然语言提问，从课程向量库中检索相关课程，并调用大语言模型生成解释性推荐结果。

---

## 🎓 项目背景



在高校选课场景里，学生经常会遇到这些问题：

- 想找「没有期末考试」「作业少一点」的课，只能在群聊 / 论坛里到处翻评价；
- 教学系统只支持按课程名、教师名等精确检索，没法理解「轻松一点的公选课」这类模糊需求；
- 新同学不了解课程结构，很难根据兴趣和负担做整体规划。

**WHU 课程推荐系统** 希望做三件事：

1. 把课程信息整理成结构化+向量化的知识库，让「经验」可以被机器检索；
2. 用 RAG 方式把检索结果喂给大模型，让模型给出有依据的推荐和解释；
3. 加上访问频控和设备指纹，让这个后端可以比较安全、稳定地对接前端小程序 / 网页 / 机器人。

它既是一个实用的选课辅助工具，也是一个 **RAG + 向量数据库 + 限流策略** 的完整工程实践样例，适合二次开发和学习。

---

## 🌱 项目意义



从工程和产品两个角度，这个项目的意义在于：

- **从「关键词检索」升级到「语义检索」**：  
  通过向量数据库 Qdrant，把课程描述、评价等信息编码成向量，使系统能够理解「不卷、不考期末」这类模糊需求，而不是只支持课名关键字。

- **把「数据 + 模型」解耦**：  
  RAG 让课程数据的维护和大模型的升级解耦：  
  - 想更新课程数据 → 只需要重建索引；  
  - 想换模型 → 只要替换 DeepSeek/OpenAI 兼容接口即可。

- **面向真实应用的「访问治理」实践**：  
  通过设备指纹 + Redis 控制调用频率，模拟真实产品环境中对免费调用额度、滥用风险的控制逻辑，为未来扩展为正式校园服务打基础。

- **作为 Go RAG 服务端的参考实现**：  
  对于正在学习 / 使用 Go、想接入 Qdrant、Redis、DeepSeek 的同学，这个仓库可以直接作为脚手架或示例项目。

---

## 🧱 系统架构概览



简化后的调用链路如下：

```text
用户请求（含设备指纹 + 问题）
           │
           ▼
   Gin HTTP Server（Go）
           │
           ├─▶ 访问频控模块（Redis + 设备指纹）
           │       └─ 超出配额则直接返回错误
           │
           ├─▶ 嵌入向量服务（Cloudflare Worker / OpenAI 接口）
           │
           ├─▶ 向量检索（Qdrant：相似课程 Top K）
           │
           ├─▶ 构造 RAG Prompt（课程信息 + 用户问题）
           │
           └─▶ 大语言模型（DeepSeek 多轮对话）
                       │
                       ▼
                 推荐结果返回给前端
```

- 向量构建由 `build_index` 目录下的脚本负责，将课程数据批量写入 Qdrant
- 在线查询由 `go-server` 提供 HTTP 接口，完成限流、检索、调用大模型等逻辑

------

## 🧠 功能亮点



- ✅ **支持课程向量检索（Qdrant）**
  - 将课程信息向量化存入 Qdrant，实现基于语义相似度的检索，而不是简单字符串匹配。
- ✅ **支持用户访问频控（Redis）**
  - 使用 Redis 记录设备调用次数，实现按周期的配额控制，避免滥用。
- ✅ **支持 DeepSeek 多轮对话模型**
  - 通过兼容 OpenAI 风格的 API 调用 DeepSeek，支持将前文上下文、检索结果拼接成更自然的推荐对话。
- ✅ **支持设备指纹限制每周调用次数（默认 10 次）**
  - 通过 `X-Device-Fingerprint` 识别设备，将每个指纹的请求次数限制在每周固定配额内（默认 10 次，可在代码中调整）。

------

## 🛠 技术栈



- **语言 & 框架**
  - Go 1.20+
  - Gin Web Framework（HTTP 服务）
- **检索 & 存储**
  - Qdrant：向量数据库，用于存放课程向量、做相似度检索
  - Redis：用作计数器，记录每个设备指纹的调用次数
- **大模型 & 向量服务**
  - DeepSeek API（OpenAI 接口兼容格式）
  - Cloudflare Worker / OpenAI 接口：用于生成课程文本的向量嵌入

------

## 📁 目录结构（简要）



```text
rag-go-server/
├── go-server/          # Go 后端服务（Gin、Redis、Qdrant、DeepSeek 调用）
├── build_index/        # 索引构建脚本（将课程数据写入 Qdrant）
├── .gitignore
└── README.md
```

> 具体脚本名和实现可参考各目录下源码，根据自己的课程数据源进行适配。

------

## ⚙️ 环境准备



### 必须安装

- Go 1.20+
- Redis 服务（本地或远程）
- Qdrant 实例（可使用 Qdrant Cloud 或本地部署）
- 可用的 DeepSeek API Key（支持 OpenAI 兼容接口）

------

## 🚀 快速开始



### 1. 克隆项目

```bash
git clone https://github.com/JeredGong/rag-go-server.git
cd rag-go-server
```

### 2. 安装依赖

```bash
bash


CopyEdit
go mod tidy
```

> 如需在 `go-server` 子目录下单独运行，可进入该目录再执行一次。

### 3. 配置环境变量

在项目根目录新建 `.env` 文件：

```env
envCopyEditOPENAI_API_KEY=sk-xxx                    # 你的 DeepSeek API Key
QDRANT_HOST=a7dcca84-xxx.cloud.qdrant.io
QDRANT_API_KEY=your_qdrant_key
REDIS_HOST=localhost:6379
REDIS_PASSWORD=                         # 如无密码则留空
```

> - `OPENAI_API_KEY` 用于调用 DeepSeek 模型；
> - `QDRANT_HOST` / `QDRANT_API_KEY` 用于连接 Qdrant；
> - `REDIS_*` 用于访问 Redis 实例。

### 4. 构建课程向量索引（离线）

1. 准备课程数据（例如包含课程名、简介、类别等字段的 JSON/CSV）。
2. 根据 `build_index` 目录下脚本的格式，把课程数据转换成向量，并写入 Qdrant 指定的 collection。
3. 索引构建完成后，在线服务即可直接查询使用。

> 索引构建流程与具体数据格式强相关，请根据脚本注释和自己掌握的课程数据进行适配。

### 5. 启动服务

在项目根目录执行：

```bash
bash


CopyEdit
go run main.go
```

默认监听地址为：

```text
cpp


CopyEdit
http://127.0.0.1:8089
```

如需更改端口，可在代码中调整配置。

------

## 📡 API 使用说明



### 请求路径

```http
bash


CopyEdit
POST /rag
```

### 必需请求头

```http
pgsqlCopyEditX-Device-Fingerprint: your-unique-device-id
Content-Type: application/json
```

- `X-Device-Fingerprint`：设备指纹，用于身份区分和频控统计。
  - 可以用浏览器指纹、设备 ID、登录用户 ID 等自定义生成。

### 请求体示例

```json
jsonCopyEdit{
  "userQuestion": "我想选一些没有期末考试的课程",
  "catagory": 0
}
```

- `userQuestion`：用户自然语言问题；
- `catagory`：
  - `0` 表示不限制课程分类；
  - 其他数值可在服务端配置为不同课程类别（例如：通识课、专业课等），按自己数据约定扩展。

### 成功响应示例

```json
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

```json
jsonCopyEdit{
  "status": "error",
  "data": {
    "message": "缺少设备指纹"
  }
}
```

> 服务端还会对超过周调用上限的设备返回错误，前端可根据错误信息提示用户。

------

## 📊 访问频控 & 配额策略



- **识别维度**：`X-Device-Fingerprint`
- **统计存储**：Redis 中为每个设备指纹维护一个计数与时间窗口；
- **周期**：以「周」为单位（具体从哪一天开始记一周，可以在代码逻辑里调整）；
- **默认配额**：每周最多调用 10 次（可根据实际需求在代码中修改）；

这样设计的好处是：

- 对于前端，可以极简接入：只要生成一个稳定的设备指纹即可；
- 对于后端，可以快速防止单设备刷接口，同时支持后续增加「登录用户维度配额」「按用户等级配额」等策略。

------

## 🔧 可扩展方向



本项目当前是一个简洁可用的后端原型，后续可以考虑：

1. **丰富课程数据源**
   - 接入更多历史评价、作业量、考试形式等字段，让推荐更细致。
2. **前端接入**
   - 对接 Web 前端 / 小程序 / 公众号，做成面向学生的实用选课助手。
3. **更精细的限流策略**
   - 按用户登录态 + 设备指纹双重限制；
   - 区分「探索期」（新用户更多免费额度）和「稳定期」配额。
4. **模型多样化**
   - 支持切换不同大模型（如 OpenAI、其他国产模型），做 A/B Test 对比效果。
5. **可观测性与监控**
   - 增加日志、指标上报，对命中率、响应时间、错误率做监控和可视化。