# ---- build stage ----
FROM golang:1.25.3-alpine AS builder
WORKDIR /src

# Cache dependencies independently of source changes.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Static binary: no libc dependency, ready for a scratch/distroless-style base.
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath -ldflags="-s -w" \
    -o /out/server ./cmd/server

# ---- runtime stage ----
FROM alpine:3.22
# ca-certificates for HTTPS targets; busybox provides wget for the healthcheck.
RUN apk add --no-cache ca-certificates \
    && adduser -D -u 10001 appuser
COPY --from=builder /out/server /usr/local/bin/server

USER appuser
EXPOSE 8080

HEALTHCHECK --interval=15s --timeout=3s --start-period=10s --retries=3 \
    CMD wget -qO- http://127.0.0.1:8080/healthz || exit 1

ENTRYPOINT ["/usr/local/bin/server"]
