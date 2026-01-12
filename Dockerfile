# Frontend build stage
FROM node:20-alpine AS frontend-builder

WORKDIR /web

# Copy frontend package files
COPY web/package*.json ./

# Install dependencies
RUN npm install

# Copy frontend source
COPY web/ ./

# Build frontend
RUN npm run build

# Backend build stage
FROM golang:1.23-alpine AS backend-builder

# Install build dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /app

# Copy source code first (needed for go mod tidy to detect imports)
COPY . .

# Tidy and download dependencies
RUN go mod tidy && go mod download

# Copy built frontend from previous stage
COPY --from=frontend-builder /web/dist ./web/dist

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o trading-bot main.go

# Runtime stage
FROM alpine:latest

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

# Set working directory
WORKDIR /app

# Copy binary from backend builder
COPY --from=backend-builder /app/trading-bot .

# Copy frontend build from backend builder
COPY --from=backend-builder /app/web/dist ./web/dist

# Copy config example (user will mount real config)
COPY config.json.example ./config.json.example

# Copy default settings for admin restore functionality
COPY default-settings.json ./default-settings.json

# Copy migrations folder for database migrations
COPY migrations/ ./migrations/

# Change ownership
RUN chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

# Default port (overridden by WEB_PORT env var in docker-compose)
# Development: 8094, Production: 8095
ENV WEB_PORT=8094

# Expose both possible ports (actual port determined by WEB_PORT env var)
EXPOSE 8094 8095

# Health check uses the WEB_PORT environment variable
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:${WEB_PORT}/api/health || exit 1

# Run the application
CMD ["./trading-bot"]
