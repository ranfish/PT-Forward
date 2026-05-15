ARG TARGETPLATFORM

FROM node:24-alpine AS frontend
WORKDIR /build/web
COPY web/package.json web/package-lock.json ./
RUN npm ci --ignore-scripts
COPY web/ .
RUN npm run build

FROM golang:1.25-alpine AS builder
RUN apk add --no-cache git gcc musl-dev
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /build/web/dist frontend/dist
ARG VERSION=dev
RUN CGO_ENABLED=1 go build -ldflags="-s -w -X main.version=${VERSION}" -o /pt-forward ./cmd/pt-forward

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata wget && \
    addgroup -S pt-forward && adduser -S -G pt-forward pt-forward
COPY --from=builder /pt-forward /pt-forward
USER pt-forward
EXPOSE 8765
VOLUME /data
VOLUME /config
HEALTHCHECK --interval=30s --timeout=3s --retries=3 CMD wget -qO- http://localhost:8765/healthz || exit 1
ENTRYPOINT ["/pt-forward"]
CMD ["--config", "/config/config.yaml"]
