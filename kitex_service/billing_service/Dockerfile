FROM golang:1.26-alpine AS builder
ENV GOPROXY=https://goproxy.cn,direct
WORKDIR /app

COPY pkg/go.mod pkg/go.sum pkg/
COPY kitex_service/billing_service/go.mod kitex_service/billing_service/go.sum kitex_service/billing_service/

RUN cd /app/kitex_service/billing_service && go mod download

COPY pkg/ pkg/
COPY kitex_service/billing_service/ kitex_service/billing_service/

WORKDIR /app/kitex_service/billing_service
RUN go mod tidy && go build -o /billing_service .

FROM alpine:3.21
RUN apk --no-cache add ca-certificates
COPY --from=builder /billing_service /billing_service
EXPOSE 4328
CMD ["/billing_service"]
