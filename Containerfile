FROM golang:1.27-alpine@sha256:4cb7ac979db5fcc41cae44b2227ba5ab8a51e8807f40d9ba4dee20a0ad960b5b AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOMEMLIMIT=768MiB go build -p 2 -ldflags="-s -w" -o /out/ratioreign ./cmd/ratioreign

FROM alpine:3.24@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b
# apk upgrade pulls in any package security patches Alpine has released
# since this base image digest was published (e.g. libssl3/libcrypto3
# fixes) -- without it, a pinned digest can silently carry known-fixed
# CVEs until Renovate happens to bump to a newer base image digest.
RUN apk upgrade --no-cache && \
    apk add --no-cache ca-certificates && \
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
