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
COPY . .
COPY --from=frontend /build/web/dist frontend/dist
ARG VERSION=dev
RUN CGO_ENABLED=1 go build -ldflags="-s -w -X main.version=${VERSION}" -o /pt-forward ./cmd/pt-forward

FROM debian:trixie-slim
RUN sed -i 's/deb.debian.org/mirrors.ustc.edu.cn/g' /etc/apt/sources.list.d/debian.sources 2>/dev/null; \
    apt-get update && \
    apt-get install -y --no-install-recommends \
        ca-certificates tzdata wget \
        ffmpeg \
        fonts-noto-cjk \
        libass9 libfontconfig1 libharfbuzz0b libfribidi0 \
        libplacebo349 libzimg2 libjpeg62-turbo \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd -r pt-forward && useradd -r -g pt-forward pt-forward

COPY --from=builder /pt-forward /usr/local/bin/pt-forward
COPY bin/amd64/mpv-new /usr/local/bin/mpv
RUN chmod 755 /usr/local/bin/pt-forward /usr/local/bin/mpv

WORKDIR /
USER pt-forward
EXPOSE 8765
VOLUME /data
VOLUME /config
VOLUME /logs
HEALTHCHECK --interval=30s --timeout=3s --retries=3 CMD wget -qO- http://localhost:8765/healthz || exit 1
ENTRYPOINT ["/pt-forward"]
CMD ["--config", "/config/config.yaml"]
