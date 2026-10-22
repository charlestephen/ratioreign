FROM node:26-alpine AS webui
WORKDIR /webui
COPY webui/package.json webui/package-lock.json ./
RUN npm ci
COPY webui/ ./
# Vite writes to ../internal/api/webui/dist, i.e. /internal/api/webui/dist here.
RUN npm run build

FROM golang:1.27-alpine@sha256:b8c9b82affbbb82a020bbcf998dad2ba9516944ac7e883524642a481a9505e9c AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Rebuild the SPA from source rather than trusting the committed dist.
COPY --from=webui /internal/api/webui/dist ./internal/api/webui/dist
RUN CGO_ENABLED=0 GOMEMLIMIT=768MiB go build -p 2 -ldflags="-s -w" -o /out/ratioreign ./cmd/ratioreign

FROM alpine:3.24@sha256:294b683cb724975bec92580e1e685676bd4b50bda910ddb8c51d4cabeaec77e6
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
