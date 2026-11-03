FROM golang:1.26-alpine AS builder
ENV GOPROXY=https://goproxy.cn,direct
WORKDIR /app

COPY pkg/go.mod pkg/go.sum pkg/
COPY kitex_service/admin_service/go.mod kitex_service/admin_service/go.sum kitex_service/admin_service/

RUN cd /app/kitex_service/admin_service && go mod download

COPY pkg/ pkg/
COPY kitex_service/admin_service/ kitex_service/admin_service/

WORKDIR /app/kitex_service/admin_service
RUN go mod tidy && go build -o /admin_service .

FROM alpine:3.21
RUN apk --no-cache add ca-certificates
COPY --from=builder /admin_service /admin_service
EXPOSE 4329
CMD ["/admin_service"]
