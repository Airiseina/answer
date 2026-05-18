FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.work go.work.sum ./
COPY go.mod go.sum ./
COPY pkg/ pkg/
COPY kitex_service/user_service/ kitex_service/user_service/
RUN go work sync && go build -o /user_service ./kitex_service/user_service/

FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /user_service /user_service
EXPOSE 4320
CMD ["/user_service"]
