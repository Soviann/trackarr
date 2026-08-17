FROM python:3.11-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    git \
    curl \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

RUN git config --global --add safe.directory "*" && \
    git config --global user.name "Antigravity NAS Agent" && \
    git config --global user.email "antigravity-bot@plextracker.local"

WORKDIR /app

COPY server.py /scripts/server.py

CMD ["python3", "/scripts/server.py"]
