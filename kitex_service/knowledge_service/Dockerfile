FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.work go.work.sum ./
COPY go.mod go.sum ./
COPY pkg/ pkg/
COPY kitex_service/knowledge_service/ kitex_service/knowledge_service/
RUN go work sync && go build -o /knowledge_service ./kitex_service/knowledge_service/

FROM alpine:latest
RUN apk --no-cache add ca-certificates poppler-utils
COPY --from=builder /knowledge_service /knowledge_service
EXPOSE 4326
CMD ["/knowledge_service"]
