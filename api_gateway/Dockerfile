FROM golang:1.26-alpine AS builder
ENV GOPROXY=https://goproxy.cn,direct
WORKDIR /app

COPY pkg/go.mod pkg/go.sum pkg/
COPY kitex_service/bot_service/go.mod kitex_service/bot_service/go.sum kitex_service/bot_service/
COPY kitex_service/chat_service/go.mod kitex_service/chat_service/go.sum kitex_service/chat_service/
COPY kitex_service/group_service/go.mod kitex_service/group_service/go.sum kitex_service/group_service/
COPY kitex_service/knowledge_service/go.mod kitex_service/knowledge_service/go.sum kitex_service/knowledge_service/
COPY kitex_service/user_service/go.mod kitex_service/user_service/go.sum kitex_service/user_service/
COPY kitex_service/work_service/go.mod kitex_service/work_service/go.sum kitex_service/work_service/
COPY api_gateway/go.mod api_gateway/go.sum api_gateway/

RUN cd /app/api_gateway && go mod download

COPY pkg/ pkg/
COPY kitex_service/bot_service/ kitex_service/bot_service/
COPY kitex_service/chat_service/ kitex_service/chat_service/
COPY kitex_service/group_service/ kitex_service/group_service/
COPY kitex_service/knowledge_service/ kitex_service/knowledge_service/
COPY kitex_service/user_service/ kitex_service/user_service/
COPY kitex_service/work_service/ kitex_service/work_service/
COPY api_gateway/ api_gateway/

WORKDIR /app/api_gateway
RUN go mod tidy && go build -o /api_gateway ./cmd/

FROM alpine:3.21
RUN apk --no-cache add ca-certificates
COPY --from=builder /api_gateway /api_gateway
EXPOSE 1234
CMD ["/api_gateway"]
