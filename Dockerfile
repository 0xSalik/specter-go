# ---- Build stage ----
FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

# Cache dependency downloads separately from source.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Migrations are embedded into the binary via go:embed, so the runtime image
# does not need a separate migrations directory.
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s -X main.version=docker" \
    -o /specter ./cmd/specter

# ---- Runtime stage ----
FROM alpine:3.19

# ca-certificates: HTTPS to Discord/external APIs.
# ffmpeg + python3 + yt-dlp: /tiktok and /ytdownload media commands (music is
# handled by the separate Lavalink node, not this image).
# ttf-dejavu: fonts for rank-card and tweet image generation.
RUN apk add --no-cache \
        ca-certificates \
        tzdata \
        ffmpeg \
        python3 \
        py3-pip \
        ttf-dejavu \
    && pip3 install --no-cache-dir yt-dlp --break-system-packages \
    && rm -rf /var/cache/apk/*

RUN addgroup -S specter && adduser -S specter -G specter

WORKDIR /app
COPY --from=builder /specter /app/specter
RUN chown -R specter:specter /app
USER specter

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=10s --start-period=15s --retries=3 \
    CMD wget -qO- http://localhost:8080/health || exit 1

ENTRYPOINT ["/app/specter"]
