FROM --platform=${BUILDPLATFORM:-linux/amd64} golang:1.24 AS builder

ARG BUILDPLATFORM
ARG TARGETOS
ARG TARGETARCH
ARG VERSION
ENV VERSION=$VERSION

WORKDIR /app/
ADD . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build \
    -ldflags="-X github.com/clambin/intel-gpu-exporter/internal/collector.version=$VERSION" \
    -o intel-gpu-exporter \
    intel-gpu-exporter.go

FROM ghcr.io/linuxserver/baseimage-ubuntu:noble

RUN \
  apt-get update && \
  apt-get install -y \
    udev \
    intel-gpu-tools

WORKDIR /app
COPY --from=builder /app/intel-gpu-exporter /app/intel-gpu-exporter

ENTRYPOINT ["/app/intel-gpu-exporter"]
CMD []
