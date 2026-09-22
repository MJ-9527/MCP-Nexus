FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -buildvcs=false -o mcp-nexus ./cmd/server/

FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /app/mcp-nexus .
EXPOSE 8080
ENV PORT=8080
CMD ["./mcp-nexus"]
