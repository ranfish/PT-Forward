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

FROM debian:bookworm-slim
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        ca-certificates tzdata wget \
        libass9 liblcms2-2 zlib1g libfribidi0 libharfbuzz0b \
        libfontconfig1 libfreetype6 libpng16-16 libstdc++6 \
        libunibreak5 libglib2.0-0 libzimg2 && \
    rm -rf /var/lib/apt/lists/* && \
    groupadd -r pt-forward && useradd -r -g pt-forward pt-forward
ARG TARGETARCH
COPY --from=builder /pt-forward /usr/local/bin/pt-forward
COPY bin/${TARGETARCH}/mpv /usr/local/bin/mpv
COPY bin/${TARGETARCH}/ffprobe /usr/local/bin/ffprobe
COPY bin/${TARGETARCH}/ffmpeg /usr/local/bin/ffmpeg
RUN chmod 755 /usr/local/bin/mpv /usr/local/bin/ffprobe /usr/local/bin/ffmpeg
USER pt-forward
EXPOSE 8765
VOLUME /data
VOLUME /config
HEALTHCHECK --interval=30s --timeout=3s --retries=3 CMD wget -qO- http://localhost:8765/healthz || exit 1
ENTRYPOINT ["/pt-forward"]
CMD ["--config", "/config/config.yaml"]
