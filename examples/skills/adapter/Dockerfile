# 二进制在宿主机交叉编译（GOOS=linux GOARCH=amd64 CGO_ENABLED=0），此处只打包。
FROM alpine:3.20

RUN apk add --no-cache ca-certificates curl jq
WORKDIR /app
COPY skills-adapter /app/skills-adapter
COPY skills.json /app/skills.json

EXPOSE 8082

ENTRYPOINT ["/app/skills-adapter"]
