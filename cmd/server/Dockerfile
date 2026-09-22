FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
COPY . .
RUN go build -o /out/server ./cmd/server

FROM alpine:3.20

RUN apk add --no-cache ca-certificates curl jq
WORKDIR /app
COPY --from=build /out/server /app/server

EXPOSE 8080

ENTRYPOINT ["/app/server"]
