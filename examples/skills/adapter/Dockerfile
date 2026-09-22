FROM golang:1.26-alpine AS build
WORKDIR /src
COPY examples/skills/adapter ./examples/skills/adapter
RUN cd examples/skills/adapter && go build -o /out/skills-adapter .

FROM alpine:3.20

RUN apk add --no-cache ca-certificates curl jq
WORKDIR /app
COPY --from=build /out/skills-adapter /app/skills-adapter
COPY examples/skills/adapter/skills.json /app/skills.json

EXPOSE 8082

ENTRYPOINT ["/app/skills-adapter"]
