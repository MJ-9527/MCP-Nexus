# 二进制在宿主机交叉编译（GOOS=linux GOARCH=amd64 CGO_ENABLED=0），此处只打包。
# 避免在国内网络下拉取 golang:1.26-alpine 大镜像。
FROM alpine:3.20

RUN apk add --no-cache ca-certificates curl jq
WORKDIR /app
COPY cmd/server/server /app/server

EXPOSE 8080

ENTRYPOINT ["/app/server"]
