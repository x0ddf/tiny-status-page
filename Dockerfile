# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -o tiny-status-page ./cmd/backend

# Final stage
FROM alpine:3.19

WORKDIR /app

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

# Copy the binary from builder
COPY --from=builder /app/tiny-status-page .
# Copy templates and static files
COPY --from=builder /app/pkg/server/templates ./pkg/server/templates
COPY --from=builder /app/pkg/server/static ./pkg/server/static

# Create non-root user
RUN adduser -D -u 1000 appuser
USER appuser

# Expose port
EXPOSE 8080

# Run the binary
ENTRYPOINT ["./tiny-status-page"]
