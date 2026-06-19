# Stage 1: Build frontend
FROM node:20-alpine AS frontend
WORKDIR /app/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# Stage 2: Build Go backend (Debian for glibc + go-sqlite3 compatibility)
FROM golang:1.22-bookworm AS backend
RUN apt-get update && apt-get install -y --no-install-recommends gcc libc6-dev && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY go.mod go.sum ./
ENV GOPROXY=https://goproxy.cn,direct
RUN go mod download
COPY . .
COPY --from=frontend /app/web/dist ./web/dist
RUN CGO_ENABLED=1 go build -o hubterm-center ./cmd/center && \
    CGO_ENABLED=1 go build -o hubterm-agent ./cmd/agent

# Stage 3: Runtime (Debian slim for glibc compatibility)
FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates tzdata && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=backend /app/hubterm-center .
COPY --from=backend /app/hubterm-agent .
COPY --from=backend /app/web/dist ./web/dist
EXPOSE 8080
VOLUME ["/data"]
ENV GIN_MODE=release
ENV DB_PATH=/data/hubterm.db
ENTRYPOINT ["./hubterm-center"]
