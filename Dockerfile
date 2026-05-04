# ─── INHERITANCE CHOIR — Go Gin Backend — Multi-stage Dockerfile ─
# Stage 1: Build a tiny static binary
FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w -X main.Version=$(git describe --tags 2>/dev/null || echo dev)" \
    -trimpath \
    -o choir-server \
    ./cmd/server

# Stage 2: Minimal runtime image (~15 MB total)
FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /build/choir-server /choir-server

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD ["/choir-server", "-health"]

ENTRYPOINT ["/choir-server"]
