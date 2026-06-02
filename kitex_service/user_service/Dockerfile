FROM golang:1.26-alpine AS builder
ENV GOPROXY=https://goproxy.cn,direct
WORKDIR /app

COPY pkg/go.mod pkg/go.sum pkg/
COPY kitex_service/user_service/go.mod kitex_service/user_service/go.sum kitex_service/user_service/

RUN cd /app/kitex_service/user_service && go mod download

COPY pkg/ pkg/
COPY kitex_service/user_service/ kitex_service/user_service/

WORKDIR /app/kitex_service/user_service
RUN go mod tidy && go build -o /user_service .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /user_service /user_service
EXPOSE 4320
CMD ["/user_service"]
