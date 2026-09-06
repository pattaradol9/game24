# 24 Game — single static binary (web SPA embedded), SQLite + AES-256-GCM secret-key encryption
# The encryption key is injected at runtime via ENCRYPTION_KEY (e.g. from
# Google Secret Manager); it is never baked into the image.
# Stage 1: build the Vue SPA
FROM node:22-alpine AS web
WORKDIR /src
COPY web/package.json web/package-lock.json* ./
RUN npm ci --no-audit --no-fund 2>/dev/null || npm install --no-audit --no-fund
COPY web/ ./
RUN npm run build

# Stage 2: build the Go server with the SPA embedded
FROM golang:1.26-alpine AS server
WORKDIR /src
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ ./
COPY --from=web /src/dist ./internal/webui/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-w -s -extldflags=-static" -o /out/game24-server ./cmd/server

# Stage 3: distroless nonroot runtime
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=server /out/game24-server /game24-server
VOLUME ["/data"]
ENV APP_PORT=8080 DB_PATH=/data/game24.db
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/game24-server"]
