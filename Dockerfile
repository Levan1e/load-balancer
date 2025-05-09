# Build stage
FROM golang:1.24.3 AS builder

# Set working directory
WORKDIR /app

# Copy go.mod and go.sum for dependency caching
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download && go mod verify

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -ldflags '-s -w' -o load-balancer ./cmd/balancer

# Final stage
FROM alpine:3.20

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

# Set working directory
WORKDIR /app

# Copy binary from build stage
COPY --from=builder /app/load-balancer .

# Copy configuration files
COPY configs /app/configs

# Expose port
EXPOSE 8087

# Healthcheck for service
HEALTHCHECK --interval=5s --timeout=3s --retries=3 CMD curl -f http://localhost:8087/health || exit 1

# Command to run the application
CMD ["./load-balancer"]