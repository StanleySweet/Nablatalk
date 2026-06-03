FROM --platform=$BUILDPLATFORM golang:1.22-alpine AS builder
RUN apk add --no-cache git gcc musl-dev opus-dev opusfile-dev
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG TARGETOS TARGETARCH
RUN GOOS=$TARGETOS GOARCH=$TARGETARCH CGO_ENABLED=1 go build -o /server ./cmd/server/

FROM alpine:3.19 AS piper-dl
RUN apk add --no-cache curl
ARG TARGETARCH
RUN case "$TARGETARCH" in \
    arm64) ARCH=aarch64 ;; \
    amd64) ARCH=x86_64 ;; \
    *) ARCH=x86_64 ;; \
    esac; \
    curl -fsSL "https://github.com/rhasspy/piper/releases/download/2023.11.14-2/piper_linux_${ARCH}.tar.gz" | \
    tar xz -C /usr/local/bin/ piper && \
    chmod +x /usr/local/bin/piper

FROM alpine:3.19
RUN apk add --no-cache ca-certificates opus opusfile
COPY --from=builder /server /usr/local/bin/tts-api
COPY --from=piper-dl /usr/local/bin/piper /usr/local/bin/piper
EXPOSE 8765
ENTRYPOINT ["tts-api"]
