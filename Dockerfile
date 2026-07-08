# Build stage
FROM golang:1.24 AS builder
ARG TARGETOS
ARG TARGETARCH

# Bypass Go proxy due to corporate network issues
ENV GOPROXY=direct

# Set working directory
WORKDIR /workspace

# Copy go.mod and go.sum first to leverage Docker cache
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY internal/ internal/
COPY cmd/ cmd/

# Build
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -a -o ws-proxy ./cmd/ws-proxy

# Use distroless as minimal base image to package the binary
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/ws-proxy /ws-proxy
USER 65532:65532

# Expose proxy port
EXPOSE 8080

# Set environment variables
ENV LISTEN_ADDR=:8080 \
    TARGET_HOST=127.0.0.1 \
    TARGET_PORT=2222 \
    MAX_SESSION_DURATION=12h \
    PING_INTERVAL=30s \
    PING_TIMEOUT=60s \
    MAX_CONNECTIONS=10

ENTRYPOINT ["/ws-proxy"]
