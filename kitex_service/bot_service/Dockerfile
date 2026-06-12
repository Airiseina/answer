FROM golang:1.26-alpine AS builder
ENV GOPROXY=https://goproxy.cn,direct
WORKDIR /app

COPY pkg/go.mod pkg/go.sum pkg/
COPY kitex_service/chat_service/go.mod kitex_service/chat_service/go.sum kitex_service/chat_service/
COPY kitex_service/knowledge_service/go.mod kitex_service/knowledge_service/go.sum kitex_service/knowledge_service/
COPY kitex_service/user_service/go.mod kitex_service/user_service/go.sum kitex_service/user_service/
COPY kitex_service/bot_service/go.mod kitex_service/bot_service/go.sum kitex_service/bot_service/

RUN cd /app/kitex_service/bot_service && go mod download

COPY pkg/ pkg/
COPY kitex_service/chat_service/ kitex_service/chat_service/
COPY kitex_service/knowledge_service/ kitex_service/knowledge_service/
COPY kitex_service/user_service/ kitex_service/user_service/
COPY kitex_service/bot_service/ kitex_service/bot_service/

WORKDIR /app/kitex_service/bot_service
RUN go mod tidy && go build -o /bot_service .

FROM alpine:3.21
RUN apk --no-cache add ca-certificates
COPY --from=builder /bot_service /bot_service
COPY --from=builder /app/kitex_service/bot_service/avatar/ /avatar/
COPY --from=builder /app/kitex_service/bot_service/prompt/ /prompt/
WORKDIR /
EXPOSE 4323
CMD ["/bot_service"]
