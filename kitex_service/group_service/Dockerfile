FROM golang:1.26-alpine AS builder
ENV GOPROXY=https://goproxy.cn,direct
WORKDIR /app

COPY pkg/go.mod pkg/go.sum pkg/
COPY kitex_service/chat_service/go.mod kitex_service/chat_service/go.sum kitex_service/chat_service/
COPY kitex_service/user_service/go.mod kitex_service/user_service/go.sum kitex_service/user_service/
COPY kitex_service/group_service/go.mod kitex_service/group_service/go.sum kitex_service/group_service/

RUN cd /app/kitex_service/group_service && go mod download

COPY pkg/ pkg/
COPY kitex_service/chat_service/ kitex_service/chat_service/
COPY kitex_service/user_service/ kitex_service/user_service/
COPY kitex_service/group_service/ kitex_service/group_service/

WORKDIR /app/kitex_service/group_service
RUN go mod tidy && go build -o /group_service .

FROM alpine:3.21
RUN apk --no-cache add ca-certificates
COPY --from=builder /group_service /group_service
EXPOSE 4321
CMD ["/group_service"]
