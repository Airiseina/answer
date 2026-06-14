FROM golang:1.26-alpine AS builder
ENV GOPROXY=https://goproxy.cn,direct
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o searxng-mcp-server .

FROM alpine:3.19

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /app/searxng-mcp-server .

EXPOSE 8001

CMD ["./searxng-mcp-server"]
