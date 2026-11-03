FROM golang:1.26-alpine AS builder
ENV GOPROXY=https://goproxy.cn,direct
WORKDIR /app

COPY pkg/go.mod pkg/go.sum pkg/
COPY kitex_service/bot_service/go.mod kitex_service/bot_service/go.sum kitex_service/bot_service/
COPY kitex_service/chat_service/go.mod kitex_service/chat_service/go.sum kitex_service/chat_service/
COPY kitex_service/open_service/go.mod kitex_service/open_service/go.sum kitex_service/open_service/

RUN cd /app/kitex_service/open_service && go mod download

COPY pkg/ pkg/
COPY kitex_service/bot_service/ kitex_service/bot_service/
COPY kitex_service/chat_service/ kitex_service/chat_service/
COPY kitex_service/open_service/ kitex_service/open_service/

WORKDIR /app/kitex_service/open_service
RUN go mod tidy && go build -o /open_service .

FROM alpine:3.21
RUN apk --no-cache add ca-certificates
COPY --from=builder /open_service /open_service
EXPOSE 4327
CMD ["/open_service"]
