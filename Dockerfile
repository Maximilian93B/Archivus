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
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o archivus cmd/server/main.go

# Final stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1001 -S archivus && \
    adduser -u 1001 -S archivus -G archivus

# Set working directory
WORKDIR /app

# Create uploads directory
RUN mkdir -p uploads && chown -R archivus:archivus uploads

# Copy binary from builder stage
COPY --from=builder /app/archivus .

# Change ownership to non-root user
RUN chown -R archivus:archivus /app

# Switch to non-root user
USER archivus

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run the application
CMD ["./archivus"] 