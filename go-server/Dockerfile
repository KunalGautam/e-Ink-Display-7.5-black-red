# Build stage
FROM golang:1.25-alpine AS builder
RUN apk add --no-cache git
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o epaper-server .

# Runtime stage
FROM alpine:latest
RUN apk add --no-cache tzdata ca-certificates
WORKDIR /app
COPY --from=builder /app/epaper-server /app/epaper-server
COPY static/ /app/static/
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/healthz || exit 1
ENTRYPOINT ["/app/epaper-server"]
