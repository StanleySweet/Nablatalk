FROM --platform=$BUILDPLATFORM golang:1.22-alpine AS builder
RUN apk add --no-cache git gcc musl-dev opus-dev opusfile-dev
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG TARGETOS TARGETARCH
RUN GOOS=$TARGETOS GOARCH=$TARGETARCH CGO_ENABLED=1 \
    go build -ldflags '-linkmode external -extldflags "-static"' \
    -o /server ./cmd/server/

FROM alpine:3.19 AS piper-dl
RUN apk add --no-cache curl
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
    ca-certificates libstdc++6 \
    && rm -rf /var/lib/apt/lists/*
COPY --from=builder /server /usr/local/bin/tts-api
COPY --from=piper-dl /opt/piper /opt/piper
ENV LD_LIBRARY_PATH=/opt/piper \
    ESPEAK_DATA_PATH=/opt/piper/espeak-ng-data
EXPOSE 8765
ENTRYPOINT ["tts-api"]
