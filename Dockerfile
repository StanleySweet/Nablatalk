FROM --platform=$BUILDPLATFORM golang:1.22-bookworm AS builder
RUN apt-get update && apt-get install -y --no-install-recommends \
    gcc pkgconf libopus-dev libopusfile-dev \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG TARGETOS TARGETARCH
RUN GOOS=$TARGETOS GOARCH=$TARGETARCH CGO_ENABLED=1 go build -o /server ./cmd/server/

FROM debian:bookworm-slim AS piper-dl
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates curl \
    && rm -rf /var/lib/apt/lists/*
ARG TARGETARCH
RUN case "$TARGETARCH" in \
    arm64) ARCH=aarch64 ;; \
    amd64) ARCH=x86_64 ;; \
    *) ARCH=x86_64 ;; \
    esac; \
    curl -fsSL "https://github.com/rhasspy/piper/releases/download/2023.11.14-2/piper_linux_${ARCH}.tar.gz" | \
    tar xz -C /opt && \
    chmod +x /opt/piper/piper

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates libopus0 libopusfile0 \
    && rm -rf /var/lib/apt/lists/*
COPY --from=builder /server /usr/local/bin/tts-api
COPY --from=piper-dl /opt/piper /opt/piper
ENV LD_LIBRARY_PATH=/opt/piper \
    ESPEAK_DATA_PATH=/opt/piper/espeak-ng-data
EXPOSE 8765
ENTRYPOINT ["tts-api"]
