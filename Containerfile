FROM golang:1.23-alpine@sha256:383395b794dffa5b53012a212365d40c8e37109a626ca30d6151c8348d380b5f AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOMEMLIMIT=768MiB go build -p 2 -ldflags="-s -w" -o /out/ratioreign ./cmd/ratioreign

FROM alpine:3.24@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b
RUN apk add --no-cache ca-certificates && \
    adduser -D -u 1000 ratioreign
COPY --from=build /out/ratioreign /usr/local/bin/ratioreign
COPY profiles /app/profiles
COPY config/config.example.yaml /app/config/config.example.yaml
WORKDIR /app
USER ratioreign
VOLUME ["/app/data", "/app/config"]
EXPOSE 7070
ENTRYPOINT ["ratioreign"]
CMD ["-config", "/app/config/config.yaml"]
