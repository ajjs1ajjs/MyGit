# Multi-stage build for production (Go)
FROM golang:1.25-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/mygit ./cmd/mygit

FROM alpine:3.20

RUN apk add --no-cache ca-certificates git openssh \
    && addgroup -S mygit && adduser -S -G mygit mygit \
    && mkdir -p /data/repos /config \
    && chown -R mygit:mygit /data /config

COPY --from=builder /out/mygit /usr/local/bin/mygit
COPY README.md CHANGELOG.md ./

ENV MYGIT_BASE_DIR=/data \
    MYGIT_REPOS_ROOT=/data/repos \
    MYGIT_DB_PATH=/data/mygit.db

USER mygit

EXPOSE 8060

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://localhost:8060/api/v1/health >/dev/null 2>&1 || exit 1

VOLUME ["/data"]

LABEL maintainer="MyGit"
LABEL version="3.0.13"
LABEL description="Self-hosted Git platform (Go)"

CMD ["mygit", "-port", "8060"]
