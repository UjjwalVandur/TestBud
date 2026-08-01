# ── Stage 1: Build ──────────────────────────────────────────────
FROM golang:1.25-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /api  ./cmd/api \
 && CGO_ENABLED=0 go build -o /seed ./cmd/seed

# ── Stage 2: Runtime ────────────────────────────────────────────
FROM alpine:latest

RUN apk --no-cache add ca-certificates

COPY --from=build /api  /usr/local/bin/api
COPY --from=build /seed /usr/local/bin/seed

# Entrypoint: seed the database, then start the API server.
COPY <<'EOF' /entrypoint.sh
#!/bin/sh
set -e
echo "→ Running database seed…"
seed
echo "→ Starting API server…"
exec api
EOF

RUN chmod +x /entrypoint.sh

EXPOSE 8080
ENTRYPOINT ["/entrypoint.sh"]
