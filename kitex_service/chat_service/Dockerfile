FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.work go.work.sum ./
COPY go.mod go.sum ./
COPY pkg/ pkg/
COPY kitex_service/chat_service/ kitex_service/chat_service/
RUN go work sync && go build -o /chat_service ./kitex_service/chat_service/

FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /chat_service /chat_service
EXPOSE 4322
CMD ["/chat_service"]
