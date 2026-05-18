FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.work go.work.sum ./
COPY go.mod go.sum ./
COPY pkg/ pkg/
COPY api_gateway/ api_gateway/
COPY kitex_service/user_service/ kitex_service/user_service/
COPY kitex_service/group_service/ kitex_service/group_service/
COPY kitex_service/chat_service/ kitex_service/chat_service/
RUN go work sync && go build -o /api_gateway ./api_gateway/cmd/

FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /api_gateway /api_gateway
EXPOSE 1234
CMD ["/api_gateway"]
