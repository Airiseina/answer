FROM golang:1.26-alpine AS builder
ENV GOPROXY=https://goproxy.cn,direct
WORKDIR /app

COPY pkg/go.mod pkg/go.sum pkg/
COPY kitex_service/bot_service/go.mod kitex_service/bot_service/go.sum kitex_service/bot_service/
COPY kitex_service/chat_service/go.mod kitex_service/chat_service/go.sum kitex_service/chat_service/
COPY kitex_service/group_service/go.mod kitex_service/group_service/go.sum kitex_service/group_service/
COPY kitex_service/user_service/go.mod kitex_service/user_service/go.sum kitex_service/user_service/
COPY msg_gateway/go.mod msg_gateway/go.sum msg_gateway/

RUN cd /app/msg_gateway && go mod download

COPY pkg/ pkg/
COPY kitex_service/bot_service/ kitex_service/bot_service/
COPY kitex_service/chat_service/ kitex_service/chat_service/
COPY kitex_service/group_service/ kitex_service/group_service/
COPY kitex_service/user_service/ kitex_service/user_service/
COPY msg_gateway/ msg_gateway/

WORKDIR /app/msg_gateway
RUN go mod tidy && go build -o /msg_gateway ./cmd/

FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /msg_gateway /msg_gateway
EXPOSE 8081
CMD ["/msg_gateway"]
