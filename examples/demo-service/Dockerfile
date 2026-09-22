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
