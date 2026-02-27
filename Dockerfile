# Multi-stage build for Watchtower
FROM golang:1.21-alpine AS builder

# Install build dependencies including C compiler for CGO and sqlite
# Required for building go-sqlite3 which uses CGO
RUN apk add --no-cache \
    git \
    make \
    gcc \
    musl-dev \
    sqlite-dev \
    linux-headers

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=1 GOOS=linux go build -o watchtower main.go

# Final stage
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    sqlite \
    curl \
    bash

# Install Go (needed for installing subfinder and httpx)
# We'll install tools at runtime via entrypoint to ensure they're always up to date
RUN apk add --no-cache go

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/watchtower .

# Copy web assets
COPY --from=builder /build/web ./web

# Create directory for database
RUN mkdir -p /app/data

# Copy entrypoint script
COPY docker-entrypoint.sh /usr/local/bin/
RUN chmod +x /usr/local/bin/docker-entrypoint.sh

# Make sure Go bin is in PATH
ENV PATH="/root/go/bin:/usr/local/go/bin:${PATH}"

# Expose web port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/ || exit 1

# Use entrypoint script
ENTRYPOINT ["docker-entrypoint.sh"]

# Run the application
CMD ["./watchtower"]
