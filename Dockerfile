ARG TARGETPLATFORM

FROM node:24-alpine AS frontend
WORKDIR /build/web
COPY web/package.json web/package-lock.json ./
RUN npm ci --ignore-scripts
COPY web/ .
RUN npm run build

FROM golang:1.25-bookworm AS builder
RUN apt-get update && apt-get install -y --no-install-recommends git gcc libc6-dev && rm -rf /var/lib/apt/lists/*
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
ARG CACHEBUST=1
COPY . .
COPY --from=frontend /build/web/dist frontend/dist
ARG VERSION=dev
RUN CGO_ENABLED=1 go build -ldflags="-s -w -X main.version=${VERSION}" -o /pt-forward ./cmd/pt-forward

FROM debian:trixie-slim
# §56.27: 先装基础工具（trixie-slim 不含 wget）
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates wget && \
    rm -rf /var/lib/apt/lists/*
# trixie 无 mediainfo 包，从 bookworm 源安装（glibc 向后兼容）
RUN echo "deb http://deb.debian.org/debian bookworm main" > /etc/apt/bookworm.list && \
    apt-get update && \
    apt-get install -y --no-install-recommends --no-install-suggests \
        tzdata \
        ffmpeg \
        mediainfo \
        fonts-noto-cjk \
        libass9 libfontconfig1 libharfbuzz0b libfribidi0 \
        libplacebo349 libzimg2 libjpeg62-turbo \
    && rm /etc/apt/bookworm.list \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd -r pt-forward && useradd -r -g pt-forward pt-forward

COPY --from=builder /pt-forward /usr/local/bin/pt-forward
COPY bin/amd64/mpv-new /usr/local/bin/mpv
RUN chmod 755 /usr/local/bin/pt-forward /usr/local/bin/mpv

WORKDIR /
EXPOSE 8765
VOLUME /data
VOLUME /config
VOLUME /logs
HEALTHCHECK --interval=30s --timeout=3s --retries=3 CMD wget -qO- http://localhost:8765/healthz || exit 1
ENTRYPOINT ["/usr/local/bin/pt-forward"]
CMD ["--config", "/config/config.yaml"]
