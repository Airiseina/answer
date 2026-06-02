.PHONY: help infra-up infra-down docker-up docker-down docker-build \
        build fmt lint vet mcp-install mcp-run-mem0 mcp-run-knowledge mcp-run-searxng \
        clean run-api-gateway run-msg-gateway \
        run-user-service run-chat-service run-group-service \
        run-bot-service run-work-service run-knowledge-service

## 默认目标
.DEFAULT_GOAL := help

## —— Docker Compose ——

infra-up:
	cd deploy && docker compose up -d mysql postgres qdrant garnet kafka-1 kafka-2 etcd seaweedfs-master seaweedfs-volume seaweedfs-filer traefik jaeger otel-collector prometheus grafana searxng

infra-down:
	cd deploy && docker compose down

docker-up:
	cd deploy && docker compose up -d

docker-down:
	cd deploy && docker compose down

docker-down-v:
	cd deploy && docker compose down -v

docker-build:
	cd deploy && docker compose build

docker-logs:
	cd deploy && docker compose logs -f

docker-ps:
	cd deploy && docker compose ps

## —— Go 编译 ——

build: build-api-gateway build-msg-gateway build-user-service build-chat-service build-group-service build-bot-service build-work-service build-knowledge-service

build-api-gateway:
	go build -o bin/api_gateway ./api_gateway/cmd/

build-msg-gateway:
	go build -o bin/msg_gateway ./msg_gateway/cmd/

build-user-service:
	go build -o bin/user_service ./kitex_service/user_service/

build-chat-service:
	go build -o bin/chat_service ./kitex_service/chat_service/

build-group-service:
	go build -o bin/group_service ./kitex_service/group_service/

build-bot-service:
	go build -o bin/bot_service ./kitex_service/bot_service/

build-work-service:
	go build -o bin/work_service ./kitex_service/work_service/

build-knowledge-service:
	go build -o bin/knowledge_service ./kitex_service/knowledge_service/

## —— Go 代码质量 ——

fmt:
	go fmt ./...

vet:
	go vet ./...

lint:
	golangci-lint run ./...

## —— 本地运行 Go 服务 ——

run-api-gateway:
	cd api_gateway && go run cmd/main.go

run-msg-gateway:
	cd msg_gateway && go run cmd/main.go

run-user-service:
	cd kitex_service/user_service && go run main.go

run-chat-service:
	cd kitex_service/chat_service && go run main.go

run-group-service:
	cd kitex_service/group_service && go run main.go

run-bot-service:
	cd kitex_service/bot_service && go run main.go

run-work-service:
	cd kitex_service/work_service && go run main.go

run-knowledge-service:
	cd kitex_service/knowledge_service && go run main.go

## —— MCP Server ——

mcp-install:
	cd mcp-servers/mem0 && pip install -r requirements.txt
	cd mcp-servers/knowledge && pip install -r requirements.txt
	cd mcp-servers/searxng && pip install -r requirements.txt
	pip install mcp_weather_server
	pip install MCP-timeserver

mcp-run-mem0:
	cd mcp-servers/mem0 && python -m mem0_mcp_server --mode sse --host 0.0.0.0 --port 8000

mcp-run-knowledge:
	cd mcp-servers/knowledge && python -m knowledge --mode sse --host 0.0.0.0 --port 8001

mcp-run-searxng:
	cd mcp-servers/searxng && python -m searxng --mode sse --host 0.0.0.0 --port 8001

mcp-run-weather:
	python -m mcp_weather_server --mode sse --host 0.0.0.0 --port 8080

mcp-run-timeserver:
	supergateway --stdio 'python -c "from mcp_timeserver import main; main()"' --port 8000 --ssePath /sse --messagePath /message

## —— 清理 ——

clean:
	rm -rf bin/

## —— 帮助 ——

help:
	@echo "Answer IM Makefile"
	@echo ""
	@echo "Docker:"
	@echo "  make infra-up           仅启动基础设施 (DB/Kafka/Qdrant/etcd/…) "
	@echo "  make infra-down         停止基础设施"
	@echo "  make docker-up          启动完整 Docker 环境"
	@echo "  make docker-down        停止 Docker 环境"
	@echo "  make docker-down-v      停止并清理数据卷"
	@echo "  make docker-build       构建所有 Docker 镜像"
	@echo "  make docker-logs        查看所有服务日志"
	@echo "  make docker-ps          查看运行状态"
	@echo ""
	@echo "Go:"
	@echo "  make build              编译所有 Go 服务到 bin/"
	@echo "  make fmt                格式化代码"
	@echo "  make vet                运行 go vet"
	@echo "  make lint               运行 golangci-lint"
	@echo "  make run-api-gateway    启动 API 网关"
	@echo "  make run-msg-gateway    启动消息网关"
	@echo "  make run-user-service   启动用户服务"
	@echo "  make run-chat-service   启动聊天服务"
	@echo "  make run-group-service  启动群组服务"
	@echo "  make run-bot-service    启动 Bot 服务"
	@echo "  make run-work-service   启动工作服务"
	@echo "  make run-knowledge-service 启动知识库服务"
	@echo ""
	@echo "MCP:"
	@echo "  make mcp-install        安装所有 MCP Server 依赖"
	@echo "  make mcp-run-mem0       启动 Mem0 记忆服务"
	@echo "  make mcp-run-knowledge  启动知识库 MCP"
	@echo "  make mcp-run-searxng    启动 SearXNG MCP"
	@echo "  make mcp-run-weather    启动天气服务"
	@echo "  make mcp-run-timeserver 启动时间服务"
	@echo ""
	@echo "  make clean              清理编译产物"
