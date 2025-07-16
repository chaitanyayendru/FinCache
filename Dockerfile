# Build stage
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o fincache ./cmd/fincache

# Final stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1001 -S fincache && \
    adduser -u 1001 -S fincache -G fincache

# Create necessary directories
RUN mkdir -p /app/data && \
    chown -R fincache:fincache /app

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/fincache .

# Copy configuration
COPY --from=builder /app/config.yaml .

# Change ownership
RUN chown fincache:fincache fincache config.yaml

# Switch to non-root user
USER fincache

# Expose ports
EXPOSE 6379 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run the application
CMD ["./fincache"] 