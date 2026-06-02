FROM golang:1.26-alpine AS builder
ENV GOPROXY=https://goproxy.cn,direct
WORKDIR /app

COPY pkg/go.mod pkg/go.sum pkg/
COPY kitex_service/group_service/go.mod kitex_service/group_service/go.sum kitex_service/group_service/
COPY kitex_service/chat_service/go.mod kitex_service/chat_service/go.sum kitex_service/chat_service/

RUN cd /app/kitex_service/chat_service && go mod download

COPY pkg/ pkg/
COPY kitex_service/group_service/ kitex_service/group_service/
COPY kitex_service/chat_service/ kitex_service/chat_service/

WORKDIR /app/kitex_service/chat_service
RUN go mod tidy && go build -o /chat_service .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /chat_service /chat_service
EXPOSE 4322
CMD ["/chat_service"]
