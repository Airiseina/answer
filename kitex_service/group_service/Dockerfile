FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.work go.work.sum ./
COPY go.mod go.sum ./
COPY pkg/ pkg/
COPY kitex_service/group_service/ kitex_service/group_service/
COPY kitex_service/user_service/ kitex_service/user_service/
RUN go work sync && go build -o /group_service ./kitex_service/group_service/

FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /group_service /group_service
EXPOSE 4321
CMD ["/group_service"]
