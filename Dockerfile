FROM golang:1.23-alpine AS builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=${VERSION}" -o /zendure-exporter ./cmd/zendure-exporter

FROM alpine:3.20

RUN adduser -D -u 1000 exporter
COPY --from=builder /zendure-exporter /usr/local/bin/zendure-exporter

USER exporter
EXPOSE 9854

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=2 \
  CMD wget -qO- http://localhost:9854/health || exit 1

ENTRYPOINT ["zendure-exporter"]
CMD ["--config", "/etc/zendure-exporter/config.yml"]
