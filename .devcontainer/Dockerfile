FROM mcr.microsoft.com/devcontainers/base:debian@sha256:a07f1804d8e665ff64bf6adcd60f4d9b9223dc52ee18a1f1a7a8e1ee1a134d34

LABEL org.opencontainers.image.description="Development Container"

RUN apt-get update && \
  apt-get install -y --no-install-recommends \
    chromium \
    chromium-driver \
    postgresql-client && \
  apt-get clean && rm -rf /var/lib/apt/lists/*

COPY --chmod=755 scripts/ /usr/local/bin/

USER vscode
