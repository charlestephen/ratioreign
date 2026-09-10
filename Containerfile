FROM golang:1.27-alpine@sha256:cf6fca6641884b8433441b2b0652976f975e1d0fdd26d177eaaf8596087f3125 AS build
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
