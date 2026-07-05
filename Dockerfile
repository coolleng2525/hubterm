# syntax=docker/dockerfile:1

# Stage 1: Build frontend
FROM node:20-bookworm-slim AS frontend
WORKDIR /app/web
COPY web/package.json web/package-lock.json ./
RUN --mount=type=cache,target=/root/.npm \
    npm ci
COPY web/ ./
RUN npm run build

# Stage 2: Build Go backend (pure Go SQLite, no CGO toolchain required)
FROM golang:1.22-alpine AS backend
WORKDIR /app
COPY go.mod go.sum ./
ENV GOPROXY=https://goproxy.cn,https://proxy.golang.org,direct
ARG HTTP_PROXY
ARG HTTPS_PROXY
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download
COPY . .
COPY --from=frontend /app/web/dist ./web/dist
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -o hubterm-center ./cmd/center && \
    CGO_ENABLED=0 go build -o hubterm-agent ./cmd/agent

# Stage 3: Minimal runtime
FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=backend /app/hubterm-center .
COPY --from=backend /app/hubterm-agent .
COPY --from=backend /app/web/dist ./web/dist
COPY --from=backend /app/presets ./presets
EXPOSE 8080
VOLUME ["/data"]
ENV GIN_MODE=release
ENV DB_PATH=/data/hubterm.db
ENTRYPOINT ["./hubterm-center"]
