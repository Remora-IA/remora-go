# Channel - JSON-RPC executor para Remora
# Build context: repo root

FROM golang:1.23-alpine AS builder

WORKDIR /build
COPY channel/go.mod ./
RUN go mod download

COPY channel/ ./
RUN go build -o /channel ./cmd/channel

FROM alpine:latest
RUN apk --no-cache add ca-certificates wget
WORKDIR /
COPY --from=builder /channel /channel

# Base dir se configura via env CHANNEL_BASE_DIR
EXPOSE 8765
ENTRYPOINT ["/channel"]
CMD ["--addr", ":8765"]
