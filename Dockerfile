# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /workspace

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -o hailo-device-plugin .

# Runtime stage
FROM alpine:3.18

# Install necessary runtime dependencies
RUN apk add --no-cache ca-certificates

WORKDIR /

# Copy the binary from builder
COPY --from=builder /workspace/hailo-device-plugin .

# Create necessary directories
RUN mkdir -p /etc/cdi

ENTRYPOINT ["/hailo-device-plugin"]
