# ---------- 运行阶段 ----------
# 二进制在宿主机交叉编译（GOOS=linux GOARCH=amd64 CGO_ENABLED=0），此处只打包，
# 避免在国内网络下拉取 golang:1.26-alpine 大镜像。
FROM alpine:3.20

# ca-certificates 备用；curl + jq 供 compose 里的 healthcheck 使用
RUN apk add --no-cache ca-certificates curl jq

WORKDIR /app
COPY demo-service /app/demo-service
COPY data/ /app/data/

EXPOSE 8081

ENTRYPOINT ["/app/demo-service"]
