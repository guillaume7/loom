FROM golang:1.23-bookworm

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        ca-certificates \
        curl \
        git \
        nodejs \
        npm \
        ripgrep \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /workspaces