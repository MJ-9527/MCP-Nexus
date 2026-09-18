FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY main.go ./
COPY skills.json ./
RUN go build -o /out/skills-adapter .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=build /out/skills-adapter /app/skills-adapter
COPY skills.json /app/skills.json
EXPOSE 8082
ENTRYPOINT ["/app/skills-adapter"]
