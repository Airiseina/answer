FROM golang:1.26-alpine AS builder
ENV GOPROXY=https://goproxy.cn,direct
WORKDIR /app

COPY pkg/go.mod pkg/go.sum pkg/
COPY kitex_service/bot_service/go.mod kitex_service/bot_service/go.sum kitex_service/bot_service/
COPY kitex_service/chat_service/go.mod kitex_service/chat_service/go.sum kitex_service/chat_service/
COPY kitex_service/user_service/go.mod kitex_service/user_service/go.sum kitex_service/user_service/
COPY kitex_service/work_service/go.mod kitex_service/work_service/go.sum kitex_service/work_service/

RUN cd /app/kitex_service/work_service && go mod download

COPY pkg/ pkg/
COPY kitex_service/bot_service/ kitex_service/bot_service/
COPY kitex_service/chat_service/ kitex_service/chat_service/
COPY kitex_service/user_service/ kitex_service/user_service/
COPY kitex_service/work_service/ kitex_service/work_service/

WORKDIR /app/kitex_service/work_service
RUN go mod tidy && go build -o /work_service .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /work_service /work_service
WORKDIR /
EXPOSE 4324
CMD ["/work_service"]
