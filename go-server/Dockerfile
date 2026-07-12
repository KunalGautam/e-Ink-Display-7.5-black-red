# --- Stage 1: Build Go Server binary ---
FROM golang:1.21-bookworm AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o eink-server main.go

# --- Stage 2: Final Slim Runtime ---
FROM python:3.10-slim-bookworm

# Install required system rendering package (libraqm) and CA certificates
RUN apt-get update && apt-get install -y --no-install-recommends \
    libraqm-dev \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Install Python rendering packages globally inside the container
RUN pip install --no-cache-dir pillow requests

# Copy built server binary, layout generator, and static resources from builder stage
COPY --from=builder /app/eink-server .
COPY --from=builder /app/renderer.py .
COPY --from=builder /app/static ./static

# Expose HTTP port
EXPOSE 8080

# Run server with configuration mapping
CMD ["./eink-server", "-config", "config.yaml"]
