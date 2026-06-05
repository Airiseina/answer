# Answer IM

Answer IM 是一个面向多人在线场景的 AI 增强即时通讯系统。

## 架构概览

项目采用**微服务架构**，分为三大领域层：

```
                          ┌──────────────┐
                          │ Traefik :80  │
                          └──────┬───────┘
                                 │
┌────────────────────────────────┼────────────────────────────────────┐
│                        Gateway (接入层)                              │
│  ┌──────────────────┐  ┌──────────────────┐                        │
│  │  api_gateway      │  │  msg_gateway     │                        │
│  │  :1234 (Hertz)    │  │  :8081 (WebSocket)│                       │
│  └──────────────────┘  └──────────────────┘                        │
└────────────────────────────────┬────────────────────────────────────┘
                                 │ Kitex RPC + Etcd
┌────────────────────────────────┼────────────────────────────────────┐
│                     Business (业务层)                                │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐            │
│  │   User   │  │  Group   │  │   Chat   │  │   Bot    │            │
│  │  :4320   │  │  :4321   │  │  :4322   │  │  :4323   │            │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘            │
│  ┌──────────────────┐  ┌──────────────────┐                         │
│  │  Knowledge        │  │  Work (Eino)     │                         │
│  │  :4326            │  │  :4324            │                         │
│  └──────────────────┘  └──────────────────┘                         │
└────────────────────────────────┬────────────────────────────────────┘
                                 │ Kafka / SSE
┌────────────────────────────────┼────────────────────────────────────┐
│                     AI (智能能力层)                                  │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐            │
│  │ mcp-mem0 │  │mcp-know- │  │mcp-sear- │  │mcp-wea-  │            │
│  │ (记忆)   │  │ ledge    │  │ xng      │  │ ther     │            │
│  │  :8000   │  │  :8001   │  │  :8001   │  │  :8080   │            │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘            │
│  ┌──────────┐                                                       │
│  │mcp-time- │                                                       │
│  │ server   │                                                       │
│  │  :8000   │                                                       │
│  └──────────┘                                                       │
└─────────────────────────────────────────────────────────────────────┘
                                 │
┌─────────────────────────────────────────────────────────────────────┐
│                        基础设施层                                    │
│  MySQL 8.0 │ PostgreSQL 16 │ ClickHouse │ Qdrant │ Kafka │ Etcd     │
│  Garnet(Redis) │ Meilisearch │ SeaweedFS │ Traefik v3 │ Prometheus  │
│  Grafana │ Jaeger │ SearXNG │ OTEL Collector                         │
└─────────────────────────────────────────────────────────────────────┘
```

## 核心功能

### 即时通讯
- **实时消息收发**：基于 WebSocket 长连接，支持单聊、群聊
- **消息类型**：文本、图片、文件
- **消息已读回执**：写扩散模型，PostgreSQL + Garnet 双写
- **在线状态管理**：Garnet 存储在线映射，支持跨网关路由
- **消息漫游与离线推送**：Kafka 驱动的消息投递，离线用户上线后同步
- **消息撤回**：支持限时撤回
- **好友关系管理**：添加、删除、好友请求处理
- **群组管理**：创建、邀请、入群申请、禁言、转让群主、管理员设置

### AI Bot
- **自定义角色与 System Prompt**：用户可创建不同人设/能力的 Bot
- **OpenAI 兼容接口**：多模型可切换，支持用户自带 API Key
- **绑定 MCP Server 扩展能力**：Bot 可调用外部工具（搜索、天气、知识库等）
- **多模态支持**：Bot 可识别图片内容，结合引用消息上下文回复
- **Agent 降级**：Agent 执行失败时自动降级为普通 LLM 调用

### MCP 工具调用
- **Eino ReAct Agent**：自动工具编排，MaxStep=8（最多 3 轮工具调用）
- **MCP 连接池**：健康检查（60s 间隔）+ 断线自动重连 + 指数退避重试
- **超时控制**：Agent 总 180s / LLM 120s / MCP 20s / 记忆 60s
- **可扩展插件体系**：内置 5 个 MCP Server，支持用户自定义扩展

### 知识库 (RAG)
- **多格式文档上传**：支持 PDF / Markdown / DOCX / PPTX
- **异步文档解析**：Kafka 驱动的解析 → 分块 → 向量化流水线
- **Qdrant 语义检索**：Top-K 向量检索，对话自动注入知识库上下文
- **Bot 绑定知识库**：对话时自动检索关联知识库内容

### 长期记忆 (Mem0)
- **双层记忆**：长期（跨会话）+ 短期（会话级）
- **对话前检索**：记忆注入 Prompt，增强上下文理解
- **对话后保存**：新记忆自动持久化到 Qdrant

### AI 辅助功能
- **消息总结**：一键总结群聊/单聊历史消息
- **智能回复候选**：根据上下文生成回复建议
- **实时多语言翻译**：支持指定目标语言翻译

## 技术栈

| 层级 | 技术 | 说明 |
|------|------|------|
| 编程语言 | Go 1.26 | 高性能后端 |
| HTTP 网关 | [Hertz](https://github.com/cloudwego/hertz) | CloudWeGo 高性能 HTTP 框架 |
| RPC 框架 | [Kitex](https://github.com/cloudwego/kitex) | CloudWego 高性能 RPC 框架 |
| WebSocket | [gorilla/websocket](https://github.com/gorilla/websocket) | 实时通信 |
| AI Agent | [Eino](https://github.com/cloudwego/eino) (ReAct) | 字节跳动 LLM + 工具编排框架 |
| 协议定义 | Thrift IDL | Kitex 服务间通信契约 |
| 服务发现 | etcd | 服务注册与发现 |
| 关系数据库 | MySQL 8.0 + GORM | 用户/群组/Bot/知识库元数据持久化 |
| 关系数据库 | PostgreSQL 16 + GORM | 消息/会话持久化 (JSONB) |
| 向量数据库 | Qdrant | 语义检索 / 记忆 / 知识库向量化 |
| 列式数据库 | ClickHouse | 冷库存储 / 历史消息归档 |
| 全文检索引擎 | Meilisearch | BM25 关键词检索 / RRF 融合排序 |
| 缓存 | Garnet (Redis 兼容) | 在线状态 / 已读序号 / 限流 / 消息缓存 |
| 消息队列 | Apache Kafka | 异步消息投递 / 文档解析 / Bot 回复 |
| 对象存储 | SeaweedFS | 文件/图片/文档存储 |
| 反向代理 | Traefik v3 | HTTP/WebSocket 路由 + 自动服务发现 |
| 可观测性 | OpenTelemetry + Prometheus + Grafana + Jaeger | 链路追踪 / 指标 / 可视化 |
| MCP 协议 | [MCP Go SDK](https://github.com/mark3labs/mcp-go) | SSE 传输 / 工具调用 |
| MCP Server | Python 3.11+ (Starlette + Uvicorn) | 记忆 / 知识库 / 搜索 / 天气 / 时间 |
| 前端 | React 19 + TypeScript + Vite 8 | 用户界面 |
| Embedding 模型 | 豆包多模态 Embedding | 知识库向量化 / 语义检索 |
| 文档解析 | pdf/DOCX/Markdown/PPTX parser | 知识库文档内容提取 |
| 服务治理 | 令牌桶限流 + 指数退避重试 + 超时控制 | API 级别限流 / MCP 重试 |

## 项目结构

```
answer/
├── api_gateway/                          # HTTP API 网关 (Hertz)
│   ├── cmd/main.go                       #   启动入口，端口 :1234
│   ├── handle/                           #   HTTP handler (用户/好友/群组/聊天/Bot/知识库/MCP)
│   ├── middleware/                       #   JWT 认证 / CORS / 令牌桶限流
│   ├── routes/                           #   路由注册
│   ├── rpc/                              #   Kitex RPC 客户端
│   └── config.yaml                       #   本地开发配置
├── msg_gateway/                          # WebSocket 长连接网关
│   ├── cmd/main.go                       #   启动入口，端口 :8081
│   ├── core/                             #   WebSocket Hub / 连接管理 / 消息路由
│   │   ├── hub.go                        #     在线状态管理 / Kafka 消费 / 跨网关推送
│   │   └── client.go                     #     客户端读写循环 / 心跳保活
│   ├── rpc/                              #   Kitex RPC 客户端
│   └── config.yaml
├── kitex_service/
│   ├── user_service/                     # 用户注册/登录/好友管理 (MySQL)
│   │   ├── main.go                       #   :4320
│   │   └── internal/                     #   handler / dao / model / service
│   ├── group_service/                    # 群组 CRUD/成员管理 (MySQL)
│   │   ├── main.go                       #   :4321
│   │   └── internal/
│   ├── chat_service/                     # 消息收发/会话管理 (PostgreSQL + ClickHouse + Garnet)
│   │   ├── main.go                       #   :4322
│   │   └── internal/                     #   写/读扩散模型 / Redis 缓存 / 冷热分离
│   │       ├── dal/                      #     数据访问层 (热库/冷库/收件箱/缓存)
│   │       │   ├── chat.go               #       消息 CRUD + Redis List 缓存
│   │       │   ├── inbox.go              #       写扩散收件箱查询
│   │       │   ├── cold_storage.go       #       ClickHouse 冷库归档
│   │       │   └── conversation.go       #       会话 + 成员缓存 (Garnet)
│   │       ├── model/                    #     消息 / 收件箱 / 冷消息 / 会话模型
│   │       └── service/                  #     消息发送 / 历史查询 / 冷热路由
│   ├── bot_service/                      # Bot 配置/MCP Server 绑定 (MySQL)
│   │   ├── main.go                       #   :4323
│   │   └── internal/
│   ├── work_service/                     # Eino Agent + MCP 调度 (Kafka Consumer)
│   │   ├── main.go                       #   :4324
│   │   └── internal/
│   │       ├── agent/react.go            #     ReAct Agent (MaxStep=8)
│   │       ├── mcp/                      #     MCP 连接池 / 重试 / 健康检查
│   │       │   ├── pool.go               #       连接管理 / 断线重连 / 指标上报
│   │       │   ├── builtin.go            #       内置 MCP Server 配置
│   │       │   ├── memory.go             #       记忆检索 / 保存
│   │       │   └── knowledge.go          #       知识库检索
│   │       ├── llm/                      #     LLM 客户端封装
│   │       ├── service/service.go        #     Bot 消息处理 / 总结 / 翻译 / 回复候选
│   │       └── consumer/bot_task.go      #     Kafka 消费者
│   └── knowledge_service/                # 文档解析/向量化/检索 (MySQL + Qdrant + Meilisearch)
│       ├── main.go                       #   :4326
│       └── internal/
│           ├── service/                  #   知识库 CRUD / 文档管理 / 混合检索
│           │   └── service.go            #     RRF 融合排序 / 向量存储双索引
│           ├── parser/                   #   文档解析 (PDF/DOCX/MD/PPTX)
│           ├── chunker/                  #   智能分块 (结构化/递归/Overlap)
│           │   ├── structural.go         #     按标题层级分块
│           │   └── recursive.go          #     分隔符递归降级分块
│           ├── consumer/doc_parse.go     #   Kafka 驱动的异步文档解析
│           └── dal/                      #   数据访问层
├── mcp-servers/                          # MCP Server (Python)
│   ├── mem0/                             #   长期/短期记忆 (Qdrant + Mem0)
│   ├── knowledge/                        #   知识库语义检索
│   └── searxng/                          #   Web 搜索代理 (SearXNG)
├── pkg/                                  # 跨服务公共包
│   ├── infra/                            #   MySQL/PostgreSQL/ClickHouse/Kafka 连接封装
│   │   ├── connect.go                    #     GORM 连接池 (MaxOpen=100, MaxIdle=10)
│   │   ├── kafka.go                      #     Kafka Producer/Consumer 封装
│   │   ├── qdrant/                       #     Qdrant 客户端封装
│   │   └── meilisearch/                  #     Meilisearch BM25 检索封装
│   ├── ai/                               #   AI 相关 (Embedding 等)
│   │   └── embedding.go                  #     豆包多模态 Embedding 客户端
│   ├── observability/                    #   OpenTelemetry 初始化
│   │   ├── tracer/                       #     Jaeger 链路追踪
│   │   └── meter/                        #     Prometheus 指标 (WS连接/消息延迟/Bot/MCP)
│   ├── snowflake/                        #   雪花 ID 生成 (时钟回拨保护)
│   ├── config/                           #   Viper 配置加载
│   ├── response/                         #   HTTP 统一响应结构 {code, msg, data}
│   └── storage/                          #   SeaweedFS 文件上传 (SHA256 去重)
├── idl/                                  # Thrift IDL 定义
├── web/test_frontend/                    # React 前端
├── deploy/                               # 部署配置
│   ├── docker-compose.yml                #   完整 Docker 编排
│   ├── .env                              #   环境变量
│   ├── traefik.yml                       #   Traefik 配置
│   ├── prometheus.yml                    #   Prometheus 配置
│   ├── otel-collector-config.yaml        #   OTEL Collector 配置
│   ├── searxng-settings.yml              #   SearXNG 配置
│   └── grafana/                          #   Grafana 仪表盘 (AIM 监控仪表盘)
├── docs/                                 # 文档
│   ├── api_doc.md                        #   API 接口文档
│   └── swagger.json                      #   Swagger 3.0 定义
├── build.go                              # Go 构建脚本 (跨平台)
├── go.work                               # Go Workspace
└── .github/workflows/                    # CI/CD (Go Lint + Build + Docker Release)
```

## 服务端口

| 领域 | 服务 | 端口 | 说明 |
|------|------|------|------|
| Gateway | api_gateway | 1234 | HTTP API 网关，RESTful 接口入口 |
| Gateway | msg_gateway | 8081 | WebSocket 网关，长连接入口 |
| Business | user_service | 4320 | 用户注册/登录/好友管理 |
| Business | group_service | 4321 | 群组 CRUD/成员管理 |
| Business | chat_service | 4322 | 消息收发/会话管理 |
| Business | bot_service | 4323 | Bot 配置/MCP 绑定 |
| Business | work_service | 4324 | Eino Agent + MCP 调度 |
| Business | knowledge_service | 4326 | 文档解析/向量化/检索 |
| AI | mcp-mem0 | 8000 | 长期/短期记忆 |
| AI | mcp-knowledge | 8001 | 知识库语义检索 |
| AI | mcp-searxng | 8001 | Web 搜索代理 |
| AI | mcp-weather | 8080 | 天气查询 |
| AI | mcp-timeserver | 8000 | 时间服务 |
| Infra | Traefik | 80/443 | 反向代理 |
| Infra | Grafana | 3000 | 监控仪表盘 |
| Infra | Jaeger | 16686 | 链路追踪 |
| Infra | Prometheus | 9090 | 指标采集 |
| Infra | Qdrant | 6333 | 向量数据库 |
| Infra | Meilisearch | 7700 | 全文检索引擎 |
| Infra | ClickHouse | 9000 | 冷库列式数据库 |

## 快速开始 (Docker)

一条命令启动完整系统（含所有基础设施、后端服务、MCP Server、前端）：

```bash
cd deploy
cp .env .env  # 检查并修改 .env 中的 API Key
docker compose up -d
```

首次构建约需 5-10 分钟。启动后：

| 服务 | 地址 |
|------|------|
| 前端 | http://localhost |
| Grafana 监控 | http://localhost:3000 (admin / admin) |
| Jaeger 链路追踪 | http://localhost:16686 |
| Prometheus | http://localhost:9090 |
| Qdrant Dashboard | http://localhost:6333/dashboard |

### Docker 常用命令

```bash
# 查看运行状态
docker compose ps

# 查看所有服务日志
docker compose logs -f

# 查看某个服务日志
docker compose logs -f api-gateway

# 重新构建并启动
docker compose up -d --build

# 停止所有服务
docker compose down

# 停止并删除数据卷
docker compose down -v
```

## 本地开发

### 环境要求

- Go 1.26+
- Node.js 18+
- Python 3.11+
- Docker & Docker Compose（用于运行基础设施）

### 1. 配置各服务的 config.yaml

本地开发时，每个 Go 服务读取自己目录下的 `config.yaml`。启动前请检查并修改以下文件中的数据库连接地址和 API Key：

| 服务 | 配置文件 | 需关注项 |
|------|---------|----------|
| api_gateway | `api_gateway/config.yaml` | etcd 地址、SeaweedFS 地址 |
| msg_gateway | `msg_gateway/config.yaml` | etcd 地址、Kafka brokers |
| user_service | `kitex_service/user_service/config.yaml` | MySQL 连接信息 |
| group_service | `kitex_service/group_service/config.yaml` | MySQL 连接信息 |
| chat_service | `kitex_service/chat_service/config.yaml` | PostgreSQL / Redis / ClickHouse 连接 |
| bot_service | `kitex_service/bot_service/config.yaml` | MySQL 连接、Bot 模型 API Key |
| work_service | `kitex_service/work_service/config.yaml` | Kafka brokers、MCP 地址 |
| knowledge_service | `kitex_service/knowledge_service/config.yaml` | MySQL 连接、Embedding API Key、Kafka/Qdrant/Meilisearch 地址 |

> **说明**：config.yaml 是开发模板，代码中 `SetDefault` 已预设默认值。若用 Docker Compose 启动基础设施，默认值 (localhost / 127.0.0.1) 通常可直接使用。API Key 类字段（如 `bot_api_key`）需要手动填入。Docker 部署则读取 `deploy/.env` 中的环境变量，config.yaml 不会被使用。

### 2. 启动基础设施

```bash
cd deploy
docker compose up -d mysql postgres qdrant garnet kafka-1 kafka-2 etcd seaweedfs-master seaweedfs-volume seaweedfs-filer traefik jaeger otel-collector prometheus grafana clickhouse meilisearch mem0-postgres
```

等基础设施就绪后启动 SearXNG：

```bash
docker compose up -d searxng
```

### 3. 安装 MCP Server 依赖

```bash
cd mcp-servers/mem0 && pip install -r requirements.txt
cd ../knowledge && pip install -r requirements.txt
cd ../searxng && pip install -r requirements.txt
```

或用 pip 直接安装公共 MCP Server：

```bash
pip install mcp_weather_server
pip install MCP-timeserver
```

### 4. 启动后端服务

每个服务需在其目录下运行（会读取当前目录的 `config.yaml`）：

```bash
# API 网关 (端口 1234)
cd api_gateway && go run cmd/main.go

# 消息网关 (端口 8081)
cd msg_gateway && go run cmd/main.go

# 微服务
cd kitex_service/user_service && go run main.go        # :4320
cd kitex_service/group_service && go run main.go       # :4321
cd kitex_service/chat_service && go run main.go        # :4322
cd kitex_service/bot_service && go run main.go         # :4323
cd kitex_service/work_service && go run main.go        # :4324
cd kitex_service/knowledge_service && go run main.go   # :4326
```

### 5. 启动 MCP Server

```bash
# Mem0 记忆 (端口 8000)
cd mcp-servers/mem0 && python -m mem0_mcp_server --mode sse --host 0.0.0.0 --port 8000

# Knowledge 知识库 (端口 8001)
cd mcp-servers/knowledge && python -m knowledge --mode sse --host 0.0.0.0 --port 8001

# SearXNG 搜索 (端口 8001，需 searxng 容器已启动)
cd mcp-servers/searxng && python -m searxng --mode sse --host 0.0.0.0 --port 8001

# 天气 (端口 8080)
python -m mcp_weather_server --mode sse --host 0.0.0.0 --port 8080

# 时间 (端口 8000，需 supergateway + npm)
supergateway --stdio 'python -c "from mcp_timeserver import main; main()"' --port 8000 --ssePath /sse --messagePath /message
```

### 6. 启动前端

```bash
cd web/test_frontend
npm install
npm run dev
```

## 构建脚本

项目提供 `build.go`（Go 编写，跨平台兼容）简化常用操作：

```bash
# 查看所有可用命令
go run build.go help

# 启动全部基础设施（不含业务服务）
go run build.go infra-up

# 启动完整 Docker 环境
go run build.go docker-up

# 停止 Docker 环境
go run build.go docker-down

# 编译所有 Go 服务
go run build.go build

# 安装 MCP Server 依赖
go run build.go mcp-install

# 格式化代码
go run build.go fmt

# 运行代码检查
go run build.go lint
```

## 性能优化

### 连接池与数据库优化
- **GORM 连接池**：MySQL / PostgreSQL 统一配置 `MaxOpenConns=100`、`MaxIdleConns=10`，避免频繁建连
- **Redis Pipeline**：在线状态批量续期、已读序号批量查询均使用 Garnet Pipeline，减少网络往返
- **Kafka Producer**：`BatchSize=1` + `RequiredAcks=All` + `MaxAttempts=5`，确保消息可靠投递
- **Kafka Consumer**：Consumer Group 模式，支持水平扩展

### 缓存策略
- **写扩散模型**：消息已读序号同时写入 PostgreSQL（持久化）和 Garnet（缓存），读取优先走缓存
- **在线状态缓存**：用户上线时写入 Garnet（TTL 90s），心跳 30s 间隔批量续期，Garnet 不可用时降级到 RPC
- **MCP 连接池**：已建立的 SSE 连接复用，避免重复握手；连接断开时自动重连

### 限流与降级
- **令牌桶限流**（`golang.org/x/time/rate`）：
  - 翻译：3s/次，Burst=3
  - 回复候选：10s/次，Burst=2
  - 总结：30s/次，Burst=1
  - 过期桶 5 分钟自动清理
- **Agent 降级**：ReAct Agent 执行失败时自动降级为普通 LLM Chat 调用，确保 Bot 始终能回复
- **MCP 降级**：`mcp.fallback.enabled=true` 时，MCP 调用失败后跳过工具调用直接 LLM 回复
- **Garnet 降级**：Garnet 不可用时自动回退到 RPC 调用（在线状态、已读序号）

### 超时控制
- **Agent 总超时**：180s（含多轮 LLM + 工具调用）
- **LLM 超时**：120s
- **MCP 工具超时**：20s（记忆操作 60s）
- **MCP 重试**：指数退避，最大重试 2 次，仅对连接断开/超时/transport 错误重试
- **RPC 超时**：Kitex 默认超时 + 业务级 context timeout

### 消息投递优化
- **WebSocket Hub**：基于 channel 的事件驱动模型（Register/Unregister/Message），避免锁竞争
- **跨网关推送**：Garnet 存储 userId → gatewayAddr 映射，支持多网关实例水平扩展
- **Bot 回复异步化**：work_service 通过 Kafka Consumer 异步消费 Bot 任务，回复通过 Kafka Producer 推送到 msg_gateway
- **消息批量操作**：群消息推送使用 Garnet MGet 批量查询在线状态，避免 N+1 查询

## 核心技术实现

### 冷热分离存储

消息存储采用**三级分层架构**：Redis 缓存 → PostgreSQL 热库 → ClickHouse 冷库，在存储成本和查询性能之间取得平衡。

```
                   消息查询路由

  客户端请求
      │
      ▼
  Redis List (消息首屏缓存, TTL 24h)
      │
      ├── 命中 → 直接返回
      │
      ▼ 未命中
  PostgreSQL 热库 (6个月内消息)
      │
      ├── 命中 → 返回 + 回写 Redis 缓存
      │
      ▼ 超过6个月
  ClickHouse 冷库 (历史归档消息)
      │
      └── 返回 (列式存储，高效分析查询)
```

**热库（PostgreSQL）**：
- 存储近 6 个月（`HotDataMonths=6`）的活跃消息
- 使用 JSONB 字段存储消息体，支持灵活的查询和索引
- 索引策略：`(conversation_id, timestamp)` + `(conversation_id, seq)` 复合索引

**冷库（ClickHouse）**：
- 存储 6 个月前的历史消息，使用 `clickhouse-go` 原生客户端
- 列式存储，适合海量数据的高效分析查询
- 批量归档接口：每次 500 条为一组，使用 ClickHouse `PrepareBatch` 批量写入
- 归档完成后从热库删除已迁移记录，释放 PostgreSQL 存储空间

**缓存（Garnet/Redis）**：
- 首屏加载优先从 Redis List 缓存读取最近 50 条消息
- Key 格式：`conv:msgs:{conversationID}`，TTL 24 小时
- 发消息时同步 LPUSH + LTRIM 维护缓存，保证缓存与数据一致

**归档调度**：
- 由定时任务触发（默认每天凌晨 3 点，`archive_cron: "0 0 3 * * ?"`）
- 按会话维度批量处理：`GetHotMessagesBeforeTime()` → `ArchiveMessages()` → `DeleteMessagesBeforeTime()`
- 冷库归档可在配置中关闭（`cold_storage.enabled: false`），默认关闭

### 写扩散 / 读扩散

消息投递采用**混合扩散模式**，根据群成员数动态选择写扩散或读扩散路径。

**写扩散（Inbox，≤100 人）**：
- 发消息时，为群内每个成员写入一条 `inbox_message` 收件箱记录
- 拉取消息时直接查询个人收件箱，无需实时计算成员关系
- 读多写少的场景下性能最优：一次写入 N 条，后续 N 次读取均为纯索引扫描
- 成员数阈值：`WriteDiffusionThreshold=100`

**读扩散（>100 人）**：
- 发消息时仅写入一条群消息记录，不写入收件箱
- 拉取消息时通过群会话 ID 查询消息表
- 写入成本恒定（O(1)），适合大群推送场景

**扩散模式切换**：
- 发送消息时，先查询会话成员数
- 成员数 ≤ 100：走写扩散，为每个成员创建收件箱记录
- 成员数 > 100：走读扩散，仅写入群消息
- 拉取消息时同样根据成员数选择路径，写扩散走收件箱，读扩散走群信箱

### 多级缓存体系

| 层级 | 存储 | 内容 | Key 格式 | TTL | 说明 |
|------|------|------|---------|-----|------|
| L1 | Garnet List | 消息首屏缓存 | `conv:msgs:{id}` | 24h | 每会话最近 50 条，LPUSH+LTRIM 维护 |
| L2 | Garnet KV | 在线状态映射 | `online:user:{id}` | 90s | userId→gatewayAddr，心跳 30s 续期 |
| L3 | Garnet KV | 会话成员缓存 | `conv:members:{id}` | 1h-24h | 单聊 24h / 群聊 1h，JSON 编码 |
| L4 | PostgreSQL | 消息持久层 | — | 持久 | JSONB 字段存储，索引加速 |

**缓存降级策略**：
- Garnet 不可用时自动回退到 RPC 调用（在线状态查询、已读序号查询）
- Redis List 未命中时自动回源 PostgreSQL
- 成员缓存过期时直接查询数据库

### 混合检索引擎 (RAG)

知识库检索采用**向量检索 + 关键词检索双路并行 + RRF 融合排序**的混合检索策略。

```
用户查询: "如何配置 Kafka 集群？"

   ├────────────── 并行执行 ──────────────┐
   ▼                                     ▼
向量检索 (Qdrant)              BM25 关键词 (Meilisearch)
   │                                     │
   ▼                                     ▼
语义相似度排序                 关键词相关性排序
Score: 0.92                   Score: 0.85
   │                                     │
   └──────────────┬──────────────────────┘
                  ▼
         RRF 融合排序
    score = Σ 1/(60 + rank_i)
                  │
                  ▼
        融合结果 Top-K (降序)
   1. Chunk A (RRF: 0.032)
   2. Chunk B (RRF: 0.028)
   3. Chunk C (RRF: 0.021)
```

**向量检索（Qdrant + 豆包多模态 Embedding）**：
- 使用豆包多模态 Embedding 模型将查询文本转为向量
- Qdrant 进行 ANN 近似最近邻检索，返回 Top-K 结果及相似度分数
- 按 KB ID 过滤（`Should` 条件），支持跨知识库联合检索

**关键词检索（Meilisearch BM25）**：
- 文档分块后同步索引到 Meilisearch（双写：Qdrant + Meilisearch）
- 使用 BM25 算法计算关键词相关性分数
- 可搜索字段：`content`、`source`；可过滤字段：`kb_id`、`doc_id`

**RRF 融合排序**：
- 算法公式：`RRF_score = Σ 1/(k + rank_i)`，其中 `k=60`（平滑常数）
- 向量检索排名 + BM25 检索排名各自贡献权重
- 使用 `(doc_id, chunk_index)` 作为去重键合并两路结果
- 按 RRF 分数降序排列后截取 Top-K 返回

**降级策略**：
- Meilisearch 不可用时，自动降级为纯向量检索
- 向量检索不可用时，自动降级为纯 BM25 检索
- 两路均失败时，返回错误信息

### RAG 文档处理管道

文档从上传到可检索，经过以下异步流水线：

```
上传文件 (PDF/DOCX/Markdown/PPTX)
    │
    ▼
[1] SeaweedFS 存储 (SHA256 内容去重)
    │
    ▼
[2] Kafka 异步消息 (topic: doc_parse)
    │  按 KB ID 分组排队，同知识库内串行解析
    ▼
[3] 文档解析
    ├─ PDF   → 文本提取 + 段落 + 页码
    ├─ DOCX  → 段落 + 表格 + 页码
    ├─ MD    → 按标题层级解析为 Section
    └─ PPTX  → 幻灯片文本提取
    │
    ▼
[4] 智能分块 (Chunking)
    ├─ 结构化分块 (有 Heading 层级时)
    │   └─ 超长 Section → 递归分块 + 标题前缀
    ├─ 递归分块 (分隔符递归降级)
    │   分隔符: \n\n → \n → 。→ . → ！→ ? → ；→ ' ' → 字符
    └─ 参数: ChunkSize=800, Overlap=150
    │
    ▼
[5] 向量化 + 双索引
    ├─ 豆包多模态 Embedding
    ├─ Qdrant Upsert (向量)
    └─ Meilisearch Index (倒排)
    │
    ▼
[6] 状态更新 (status=done, doc_count++, chunk_count++)
```

**异步处理与容错**：
- 文档解析通过 Kafka Consumer Group 异步消费，最长解析时间 10 分钟
- 按 KB ID 分组串行解析（同知识库内文档按顺序处理），避免 Embedding API 并发超限
- Panic 恢复：解析过程 Panic 时自动标记为 `Failed` 状态
- 卡住文档恢复：启动时自动扫描 `Parsing` 状态超过阈值的文档，重置为 `Pending` 重新入队
- 支持文档重试：失败的文档可通过 `RepublishDocParse` 重新发送到 Kafka

### MCP 连接池架构

**连接生命周期**：
- **初始化**：SSE 传输 → MCP Initialize 握手（协议版本协商） → 获取工具列表
- **复用**：已建立连接直接复用，`lastUsed` 时间戳记录最后活跃时间
- **断线检测**：`OnConnectionLost` 回调 + 60s 健康检查双重机制
- **重连策略**：关闭旧连接 → 删除池中条目 → 重新 Connect（per-server mutex 防止并发重连）

**调用保障机制**：
- **指数退避重试**：最大 2 次重试，间隔公式 `InitialInterval * 2^(attempt-1)`
- **可重试错误**：连接断开、超时、transport 错误 → 触发重试
- **不可重试错误**：业务逻辑错误（如参数无效）→ 直接返回
- **连接断开处理**：检测到连接断开时先尝试重连，再重试调用
- **指标上报**：每次调用上报延迟直方图 + 调用计数 + 错误/超时/降级计数

**多级降级策略**：

| 降级级别 | 触发条件 | 降级行为 |
|---------|---------|---------|
| Agent 降级 | ReAct Agent 执行失败 | 降级为普通 LLM Chat 调用 |
| MCP 降级 | `mcp.fallback.enabled=true` 且 MCP 调用失败 | 跳过工具调用，直接 LLM 回复 |
| 记忆降级 | Mem0 不可用或返回错误 | 跳过记忆检索和保存，不影响基本对话 |
| 知识库降级 | 知识库检索失败 | 不阻断对话，仅丢失上下文增强 |
| Garnet 降级 | Garnet 不可用 | 回退到 RPC 查询在线状态/已读序号 |
| 缓存降级 | Redis List 未命中 | 回源 PostgreSQL 查询 |

**内置工具过滤**：
- 5 个内置 Server：mem0（记忆）、knowledge（知识库检索）、searxng（Web 搜索）、weather（天气）、timeserver（时间）
- mem0 和 knowledge 为内部 Server，Agent 工具列表中自动过滤，通过专用方法调用
- 外部 Server（searxng/weather/timeserver）作为通用工具暴露给 ReAct Agent

### Agent 上下文增强流程

```
用户消息 "上次和 Alice 讨论的 Kafka 配置方案是什么？"
    │
    ├─ 1. 记忆检索 (SearchMemories)
    │     query=用户消息, user_id, agent_id, run_id, limit=3
    │     → "[用户相关记忆]\n{memories_json}"
    │
    ├─ 2. 知识库检索 (GetBotKnowledgeBases + SearchKnowledge)
    │     查询 Bot 绑定知识库 → 混合检索(RRF融合) → Top-5
    │     → "[相关文档]\n{chunks_text}"
    │
    ├─ 3. ReAct Agent 执行
    │     SystemPrompt + 记忆上下文 + 知识库上下文 + 工具列表
    │     MaxStep=8 (最多 3 轮工具调用)，支持多模态输入
    │
    ├─ 4. 对话后保存 (SaveMemory)
    │     content=对话摘要, user_id, agent_id, run_id
    │     → 异步保存到 Mem0
    │
    └─ 5. 返回 + 降级处理
          Agent 成功 → 返回 Agent 生成内容
          Agent 失败 → 降级为普通 LLM Chat 调用
```

**记忆策略**：
- **长期记忆**：跨会话持久化，`searchRunID=""` 时搜索全局记忆
- **短期记忆**：会话级记忆，`searchRunID=conversationID` 时搜索当前会话记忆
- **记忆注入**：`\n\n[用户相关记忆]\n{memories_json}` 拼接到 System Prompt 末尾

### 分布式 ID 生成 (Snowflake)

全局唯一 ID 使用雪花算法生成，不同服务使用不同 Worker ID 避免冲突：

| 服务 | Worker ID | 用途 |
|------|-----------|------|
| user_service | 1 | 用户 ID |
| group_service | 2 | 群组 ID |
| chat_service (消息) | 3 | 消息 ID |
| chat_service (会话) | 4 | 会话 ID |
| bot_service | 5 | Bot ID |
| knowledge_service | 6 | 文档分块 ID |

**ID 结构**（64 位）：`[1bit 保留] [41bit 时间戳] [10bit Worker] [12bit 序列号]`

**时钟回拨保护**：检测到回拨 ≤ 500ms 时等待追上；超过 500ms 时拒绝生成并 Panic。

### 文件去重存储

文件上传采用**内容哈希去重**策略：

- 上传时计算文件内容的 SHA256 哈希值
- 存储路径格式：`data/{hash[:2]}/{hash}{ext}`，实现两层目录分片
- 上传前先检查 ObjectExists，已存在则跳过上传直接返回 URL
- MIME 类型自动检测（50+ 种文件格式）
- SeaweedFS 初始化时进行 10 次健康检查重试（3s 间隔）
- 消息中的文件 URL 通过 `NormalizeContentURLs` 将内部地址替换为公开访问地址

### 服务治理

**Kitex RPC 限流**（服务端）：
- knowledge_service: `MaxConnections=200`, `MaxQPS=300`
- 通过 `server.WithLimit()` 配置，保护服务端不被突发流量击垮

**令牌桶限流（API 层）**：

| 接口 | 速率限制 | Burst | 过期清理 |
|------|---------|-------|---------|
| 翻译 | 3s/次 | 3 | 15分钟 |
| 回复候选 | 10s/次 | 2 | 15分钟 |
| 总结 | 30s/次 | 1 | 15分钟 |

- 使用 `golang.org/x/time/rate` 实现，按 `userID:endpoint` 维度隔离
- `sync.Map` 存储用户桶，后台协程每 5 分钟清理过期桶
- 超限时返回 `Retry-After` 提示，并上报 `RateLimitHitTotal` 指标

**超时控制矩阵**：

| 组件 | 超时 | 说明 |
|------|------|------|
| Agent 总超时 | 180s | 含多轮 LLM + 工具调用 + 重试 |
| LLM 推理 | 120s | 单次 ChatModel 调用 |
| MCP 工具调用 | 20s | 单次工具调用（记忆 60s） |
| MCP 连接建立 | 20s | SSE 连接 + Initialize 握手 |
| MCP 健康检查 | 10s | Ping 超时 |
| 文档解析 | 10min | 单文档解析总超时 |
| RPC 调用 | 5s | 业务 RPC 调用超时 |
| Kafka 投递 | 3s | 消息推送到 Kafka |
| 跨网关推送 | 3s | HTTP 推送到其他网关 |

## 压力测试

### 预期指标

| 指标 | 目标值 | 说明 |
|------|--------|------|
| WebSocket 并发连接 | 1,000+ | 单网关实例 |
| HTTP API QPS | 5,000+ | 会话列表等读接口 |
| 消息投递延迟 P50 | < 50ms | 本地网关直连 |
| 消息投递延迟 P95 | < 200ms | 含跨网关推送 |
| 消息投递延迟 P99 | < 500ms | 极端场景 |
| Bot 响应延迟 P50 | < 3s | 含 LLM 推理 |
| Bot 响应延迟 P95 | < 10s | 含 MCP 工具调用 |
| MCP 工具调用延迟 P95 | < 5s | 单次工具调用 |
| Kafka 消息吞吐 | 10,000+ msg/s | 2 Broker 集群 |
| 数据库连接池利用率 | < 80% | MaxOpen=100 |

## 可观测性

所有服务通过 OpenTelemetry 上报指标和追踪：

- **Metrics** → OTEL Collector → Prometheus → Grafana
- **Traces** → OTEL Collector → Jaeger
- **Grafana 仪表盘**：`http://localhost:3000`，预置 "AIM 监控仪表盘"
- **Jaeger**：`http://localhost:16686`

### 监控指标

| 指标名 | 类型 | 说明 |
|--------|------|------|
| `aim_ws_connect_total` | Counter | WebSocket 连接总数 |
| `aim_ws_disconnect_total` | Counter | WebSocket 断开总数 |
| `aim_message_latency` | Histogram | 消息投递延迟 (P50/P95/P99) |
| `aim_bot_request_total` | Counter | Bot 请求总数 |
| `aim_bot_response_latency` | Histogram | Bot 响应延迟 |
| `aim_mcp_call_total` | Counter | MCP 工具调用总数 |
| `aim_mcp_call_errors` | Counter | MCP 调用错误数 |
| `aim_mcp_call_timeout_total` | Counter | MCP 调用超时数 |
| `aim_mcp_call_latency` | Histogram | MCP 调用延迟 |
| `aim_mcp_connect_total` | Counter | MCP 连接总数 |
| `aim_mcp_connect_errors` | Counter | MCP 连接错误数 |
| `aim_mcp_reconnect_total` | Counter | MCP 重连总数 |

## 环境变量参考

完整配置见 [deploy/.env](deploy/.env)。关键变量：

| 变量 | 说明 |
|------|------|
| `JWT_KEY` | JWT 签名密钥 |
| `EMBEDDING_API_KEY` | 向量模型 API Key |
| `AI_SYSTEM_BOT_API_KEY` | Bot 大模型 API Key |
| `AI_SYSTEM_BOT_MODEL` | Bot 模型名称 |
| `MEM0_LLM_API_KEY` | Mem0 记忆层 LLM Key |
| `MEM0_EMBEDDING_API_KEY` | Mem0 向量模型 Key |
| `KNOWLEDGE_DOUBAO_EMBEDDING_API_KEY` | 知识库向量模型 Key |

## API 文档

- **Markdown 版**：[docs/api_doc.md](docs/api_doc.md) — 按模块组织的接口说明（用户认证、消息、群组、Bot、知识库等）
- **Swagger 3.0**：[docs/swagger.json](docs/swagger.json) — 可导入 Swagger Editor / Postman 查看和调试

> Base URL: `http://localhost:1234`（Docker 部署经由 Traefik 反向代理则为 `http://localhost`）
> 认证方式：JWT Bearer Token（注册和登录外均需 Header `Authorization: <token>`）

## CI/CD

- **CI** (`.github/workflows/ci.yml`): Push/PR 触发 Go Lint + Build + Test
- **Release** (`.github/workflows/release.yml`): 推送 `v*` 标签时构建 Docker 镜像并推送 GHCR

## License

All Rights Reserved.
