# syntax=docker/dockerfile:1.7
#
# Multi-arch build. With `docker buildx build --platform
# linux/amd64,linux/arm64,linux/arm/v7 .` the builder fans out per
# platform; BUILDPLATFORM / TARGETPLATFORM let us cross-compile from a
# single x86 builder, which is a lot faster than emulating each arch.

FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .

ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT
ARG VERSION=dev

# GOARM is empty for non-arm targets, "7" for linux/arm/v7. Alpine's
# busybox `sh` doesn't have bash-isms, so keep this portable.
RUN set -eu; \
    if [ "$TARGETARCH" = "arm" ] && [ -n "${TARGETVARIANT:-}" ]; then \
        export GOARM="${TARGETVARIANT#v}"; \
    fi; \
    CGO_ENABLED=0 GOOS="${TARGETOS}" GOARCH="${TARGETARCH}" \
        go build -ldflags="-s -w -X main.version=${VERSION}" \
        -o /zendure-exporter ./cmd/zendure-exporter

FROM alpine:3.23

RUN adduser -D -u 1000 exporter
COPY --from=builder /zendure-exporter /usr/local/bin/zendure-exporter

USER exporter
EXPOSE 9854

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=2 \
  CMD wget -qO- http://localhost:9854/health >/dev/null 2>&1 || exit 1

ENTRYPOINT ["zendure-exporter"]
CMD ["--config", "/etc/zendure-exporter/config.yml"]
