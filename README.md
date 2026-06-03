# Answer IM

基于微服务架构的 AI 增强即时通讯系统，融合 AI Bot、MCP 工具调用、RAG 知识库与长期记忆，提供智能对话体验。

## 架构概览

```
             ┌──────────────────────────────────────────────┐
             │               Infrastructure                 │
             │  MySQL │ PostgreSQL │ Qdrant │ Kafka │ etcd  │
             │  Garnet(Redis) │ SeaweedFS │ Jaeger │ Prometheus│
             └──────────────────────────────────────────────┘
                                   ▲
┌──────────────┐   HTTP    ┌──────────────┐   RPC   ┌──────────────────┐
│   Frontend   │──────────▶│  api_gateway │────────▶│  user_service    │
│   (React)    │           │   (Hertz)    │         │  group_service   │
└──────────────┘           └──────┬───────┘         │  chat_service    │
                                  │                 │  bot_service     │
                           ┌──────▼───────┐         │  knowledge_      │
                           │  msg_gateway │────────▶│  service         │
                           │  (WebSocket) │         └──────────────────┘
                           └──────┬───────┘
                                  │ Kafka
                           ┌──────▼───────┐
                           │ work_service │ ───▶ MCP Servers
                           │ (Eino Agent) │        ├─ mcp-mem0 (记忆)
                           └──────────────┘        ├─ mcp-knowledge
                                                   ├─ mcp-searxng (搜索)
                                                   ├─ mcp-weather (天气)
                                                   └─ mcp-timeserver
```

## 技术栈

| 类别 | 技术 |
|------|------|
| 语言 | Go 1.26 |
| RPC 框架 | [Kitex](https://github.com/cloudwego/kitex) |
| HTTP 网关 | [Hertz](https://github.com/cloudwego/hertz) |
| WebSocket | [NetPoll](https://github.com/cloudwego/netpoll) |
| AI Agent | [Eino](https://github.com/cloudwego/eino) (ReAct) |
| 协议定义 | Thrift IDL |
| 服务发现 | etcd |
| 前端 | React 19 + TypeScript + Vite 8 |
| 消息队列 | Apache Kafka |
| 关系数据库 | MySQL 8.0, PostgreSQL 16 |
| 向量数据库 | Qdrant |
| 缓存 | Garnet (Redis 兼容) |
| 对象存储 | SeaweedFS |
| 反向代理 | Traefik v3 |
| 可观测性 | OpenTelemetry + Jaeger + Prometheus + Grafana |
| MCP Server | Python 3.11+ (Starlette + Uvicorn) |

## 项目结构

```
answer/
├── api_gateway/              # HTTP API 网关 (Hertz)
├── msg_gateway/              # WebSocket 长连接网关
├── kitex_service/
│   ├── user_service/         # 用户注册/登录/好友管理 (MySQL)
│   ├── chat_service/         # 消息收发/会话管理 (PostgreSQL)
│   ├── group_service/        # 群组 CRUD/成员管理 (MySQL)
│   ├── bot_service/          # Bot 配置/MCP 绑定 (MySQL)
│   ├── work_service/         # Eino Agent + MCP 调度 (Kafka)
│   └── knowledge_service/    # 文档解析/向量化/检索 (MySQL+Qdrant)
├── mcp-servers/              # MCP Server (Python)
│   ├── mem0/                 # 长期/短期记忆
│   ├── knowledge/            # 知识库语义检索
│   └── searxng/              # Web 搜索代理
├── pkg/                      # 公共库 (ai/infra/observability/storage/…)
├── idl/                      # Thrift IDL
├── web/test_frontend/        # React 前端
├── deploy/                   # 部署配置
│   ├── docker-compose.yml
│   ├── .env                  # 环境变量
│   ├── traefik.yml           # Traefik 配置
│   ├── prometheus.yml        # Prometheus 配置
│   ├── otel-collector-config.yaml
│   ├── searxng-settings.yml
│   └── grafana/              # Grafana 仪表盘
├── docs/                     # 文档 (api_doc.md / swagger.json)
└── go.work                   # Go Workspace
```

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
| chat_service | `kitex_service/chat_service/config.yaml` | PostgreSQL / Redis 连接 |
| bot_service | `kitex_service/bot_service/config.yaml` | MySQL 连接、Bot 模型 API Key |
| work_service | `kitex_service/work_service/config.yaml` | Kafka brokers、MCP 地址 |
| knowledge_service | `kitex_service/knowledge_service/config.yaml` | MySQL 连接、Embedding API Key、Kafka/Qdrant 地址 |

> **说明**：config.yaml 是开发模板，代码中 `SetDefault` 已预设默认值。若用 Docker Compose 启动基础设施，默认值 (localhost / 127.0.0.1) 通常可直接使用。API Key 类字段（如 `bot_api_key`）需要手动填入。Docker 部署则读取 `deploy/.env` 中的环境变量，config.yaml 不会被使用。

### 2. 启动基础设施

```bash
cd deploy
docker compose up -d mysql postgres qdrant garnet kafka-1 kafka-2 etcd seaweedfs-master seaweedfs-volume seaweedfs-filer traefik jaeger otel-collector prometheus grafana
```

等基础设施就绪后启动 SearXNG（健康检查需 searxng 基础镜像拉取）：

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

## 核心功能

### 即时通讯
- 单聊 / 群聊消息收发
- WebSocket 长连接与心跳保活
- 消息漫游、离线推送、撤回、已读回执
- 好友管理、群组管理、在线状态同步

### AI Bot
- 自定义角色与 System Prompt
- OpenAI 兼容接口，多模型可切换
- 绑定 MCP Server 扩展能力
- 支持用户自带 API Key

### MCP 工具调用
- 基于 Eino ReAct Agent 自动工具编排
- MCP 连接池：健康检查 + 自动重连
- 超时控制：总 60s / LLM 30s / MCP 15s
- 可扩展插件体系

### 知识库 (RAG)
- 支持 PDF / Markdown / DOCX / PPTX 上传
- 异步文档解析 → 分块 → 向量化 (Kafka 驱动)
- Qdrant 语义检索 Top-K
- Bot 绑定知识库，对话自动注入上下文

### 长期记忆 (Mem0)
- 双层记忆：长期（跨会话）+ 短期（会话级）
- 对话前检索记忆注入 Prompt，对话后保存新记忆

## 可观测性

所有服务通过 OpenTelemetry 上报指标和追踪：

- **Metrics** → OTEL Collector → Prometheus → Grafana
- **Traces** → OTEL Collector → Jaeger
- **Grafana 仪表盘**：`http://localhost:3000`，预置 "AIM 监控仪表盘"
- **Jaeger**：`http://localhost:16686`

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
