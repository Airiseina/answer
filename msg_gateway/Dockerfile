FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.work go.work.sum ./
COPY go.mod go.sum ./
COPY pkg/ pkg/
COPY msg_gateway/ msg_gateway/
COPY kitex_service/chat_service/ kitex_service/chat_service/
RUN go work sync && go build -o /msg_gateway ./msg_gateway/cmd/

FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /msg_gateway /msg_gateway
EXPOSE 8081
CMD ["/msg_gateway"]
