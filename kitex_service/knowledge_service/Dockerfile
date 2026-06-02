FROM golang:1.26-alpine AS builder
ENV GOPROXY=https://goproxy.cn,direct
WORKDIR /app

COPY pkg/go.mod pkg/go.sum pkg/
COPY kitex_service/knowledge_service/go.mod kitex_service/knowledge_service/go.sum kitex_service/knowledge_service/

RUN cd /app/kitex_service/knowledge_service && go mod download

COPY pkg/ pkg/
COPY kitex_service/knowledge_service/ kitex_service/knowledge_service/

WORKDIR /app/kitex_service/knowledge_service
RUN go mod tidy && go build -o /knowledge_service .

FROM alpine:latest
RUN apk --no-cache add ca-certificates poppler-utils
COPY --from=builder /knowledge_service /knowledge_service
EXPOSE 4326
CMD ["/knowledge_service"]
