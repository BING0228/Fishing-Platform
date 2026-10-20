# syntax=docker/dockerfile:1.7

# ---- Frontend (Next.js static export) ----
FROM node:22-bookworm-slim AS frontend-build
WORKDIR /src/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY frontend/ ./
RUN npm run build

# ---- Go control plane ----
FROM golang:1.24-bookworm AS go-build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend-build /src/frontend/dist ./frontend/dist
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/spearhook .

# ---- Runtime with headless Chromium for SPA / QR clone ----
FROM alpine:3.20
RUN apk add --no-cache \
        ca-certificates \
        chromium \
        font-liberation \
        python3 \
        py3-flask \
        py3-flask-cors \
        py3-requests \
        tzdata

WORKDIR /app
COPY --from=go-build /out/spearhook /app/spearhook
COPY config.yaml.example /app/config.yaml.example
RUN mkdir -p /app/data

ENV CHROME_PATH=/usr/bin/chromium \
    TERM=dumb \
    TZ=Asia/Shanghai

EXPOSE 8000
VOLUME ["/app/data"]
ENTRYPOINT ["/app/spearhook"]
