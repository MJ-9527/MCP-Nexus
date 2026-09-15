# 构建网关二进制与示例 MCP Server 二进制
FROM docker.m.daocloud.io/library/golang:1.26 AS builder
WORKDIR /src
ENV GOPROXY=https://goproxy.cn,direct
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /gateway ./cmd/server
RUN CGO_ENABLED=0 go build -o /demo-service ./examples/demo-service
RUN CGO_ENABLED=0 go build -o /file-server ./examples/file-server
RUN CGO_ENABLED=0 go build -o /http-server ./examples/http-server
RUN CGO_ENABLED=0 go build -o /db-server ./examples/db-server

# 运行镜像：同一镜像可启动 gateway 或任一示例 MCP Server
FROM docker.m.daocloud.io/library/alpine:3.20
RUN apk add --no-cache wget
COPY --from=builder /gateway /gateway
COPY --from=builder /demo-service /demo-service
COPY --from=builder /file-server /file-server
COPY --from=builder /http-server /http-server
COPY --from=builder /db-server /db-server
