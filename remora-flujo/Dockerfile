# Build stage — context must be remora-go-lite/ (parent dir)
FROM golang:1.25-alpine AS builder
WORKDIR /app

# Copy local module dependencies first
COPY channel/ ./channel/
COPY framework-paladin/ ./framework-paladin/

# Copy remora-flujo module files
COPY remora-flujo/go.mod remora-flujo/go.sum ./remora-flujo/
WORKDIR /app/remora-flujo
RUN go mod download

# Copy source
COPY remora-flujo/ .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -o api_rest ./cmd/api_rest/

# Runtime stage
FROM alpine:3.19
RUN apk add --no-cache ca-certificates
WORKDIR /app

COPY --from=builder /app/remora-flujo/api_rest .

# Port
EXPOSE 8080

CMD ["./api_rest"]
