# syntax=docker/dockerfile:1

# ---------- 编译阶段 ----------
FROM golang:1.26-alpine AS builder

WORKDIR /src

# 先只拷贝 go.mod，让 go mod download 缓存依赖（首次会一并生成 go.sum）
COPY go.mod ./
RUN go mod download

# 拷贝源码并编译为静态二进制
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /out/demo-service .

# ---------- 运行阶段 ----------
FROM alpine:3.20

# ca-certificates 备用；curl + jq 供 compose 里的 healthcheck 使用
RUN apk add --no-cache ca-certificates curl jq

WORKDIR /app
COPY --from=builder /out/demo-service /app/demo-service

EXPOSE 8081

ENTRYPOINT ["/app/demo-service"]
