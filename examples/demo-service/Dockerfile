<<<<<<< HEAD
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o demo-service ./examples/demo-service/main.go

FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /app/demo-service .
EXPOSE 9001
ENV DEMO_SERVICE_PORT=9001
CMD ["./demo-service"]
=======
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
>>>>>>> origin/pull-request
