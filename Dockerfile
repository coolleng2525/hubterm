# Stage 1: Build frontend
FROM node:20-alpine AS frontend
WORKDIR /app/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# Stage 2: Build Go backend
FROM golang:1.22-alpine AS backend
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /app/web/dist ./web/dist
RUN CGO_ENABLED=0 go build -o hubterm-center ./cmd/center && \
    CGO_ENABLED=0 go build -o hubterm-agent ./cmd/agent

# Stage 3: Runtime
FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=backend /app/hubterm-center .
COPY --from=backend /app/hubterm-agent .
COPY --from=backend /app/web/dist ./web/dist
EXPOSE 8080
VOLUME ["/data"]
ENV GIN_MODE=release
ENV DB_PATH=/data/hubterm.db
ENTRYPOINT ["./hubterm-center"]
