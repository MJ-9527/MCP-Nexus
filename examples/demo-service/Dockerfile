FROM golang:1.26-alpine AS build
WORKDIR /src
COPY . .
WORKDIR /src/examples/demo-service
RUN go build -o /out/demo-service .

FROM alpine:3.20

# ca-certificates 备用；curl + jq 供 compose 里的 healthcheck 使用
RUN apk add --no-cache ca-certificates curl jq

WORKDIR /app
COPY --from=build /out/demo-service /app/demo-service
COPY examples/demo-service/data/ /app/data/

EXPOSE 8081

ENTRYPOINT ["/app/demo-service"]
