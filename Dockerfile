# Build stage
FROM --platform=$BUILDPLATFORM golang:1.21 AS builder

ARG TARGETPLATFORM
ARG BUILDPLATFORM
ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary for target architecture
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -a -ldflags '-w -s' -o hailo-device-plugin .

# Build the static poststop hook that is installed onto the host at runtime
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -a -ldflags '-w -s' -o hailo-cdi-hook ./cmd/hailo-cdi-hook

# Runtime stage - using debian slim for better compatibility
FROM debian:bookworm-slim

# Install necessary runtime dependencies
RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /

# Copy the binary from builder
COPY --from=builder /workspace/hailo-device-plugin .

# Hook binary; the plugin copies it to /var/lib/hailo-cdi/ on the host at startup
COPY --from=builder /workspace/hailo-cdi-hook /opt/hailo/hailo-cdi-hook

# Create necessary directories
RUN mkdir -p /etc/cdi

ENTRYPOINT ["/hailo-device-plugin"]
