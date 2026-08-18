FROM python:3.11-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    git \
    curl \
    ca-certificates \
    make \
    docker.io \
    && rm -rf /var/lib/apt/lists/*

# Install Docker Compose v2 CLI plugin
RUN mkdir -p /usr/local/lib/docker/cli-plugins && \
    curl -SL https://github.com/docker/compose/releases/download/v2.32.4/docker-compose-linux-x86_64 -o /usr/local/lib/docker/cli-plugins/docker-compose && \
    chmod +x /usr/local/lib/docker/cli-plugins/docker-compose && \
    ln -s /usr/local/lib/docker/cli-plugins/docker-compose /usr/local/bin/docker-compose

RUN git config --global --add safe.directory "*" && \
    git config --global user.name "Antigravity NAS Agent" && \
    git config --global user.email "antigravity-bot@plextracker.local"

WORKDIR /app

COPY server.py /scripts/server.py

CMD ["python3", "/scripts/server.py"]
