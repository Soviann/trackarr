# Frontend build
FROM node:22-bookworm AS frontend
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ .
RUN npx vite build

# Backend build
FROM golang:1.24-bookworm AS backend
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /app/frontend/dist ./frontend/dist
RUN CGO_ENABLED=1 go build -tags sqlite_fts5 -o plextracker .

# Runtime
FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y ca-certificates gosu curl && rm -rf /var/lib/apt/lists/* \
    && useradd -r -s /bin/false appuser \
    && mkdir -p /data && chown appuser:appuser /data
COPY --from=backend /app/plextracker /usr/local/bin/plextracker
COPY scripts/docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
RUN chmod +x /usr/local/bin/docker-entrypoint.sh
VOLUME /data
EXPOSE 8080
ENTRYPOINT ["docker-entrypoint.sh"]
CMD ["plextracker", "serve"]
