FROM debian:bookworm-slim
ENV DEBIAN_FRONTEND=noninteractive \
    GLAMA_VERSION="1.0.0"
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates curl git && curl -fsSL https://deb.nodesource.com/setup_24.x | bash - && apt-get install -y --no-install-recommends nodejs && npm install -g mcp-proxy@5.12.0 pnpm@10.14.0 && node --version && curl -LsSf https://astral.sh/uv/install.sh | UV_INSTALL_DIR="/usr/local/bin" sh && uv python install 3.13 --default --preview && ln -s $(uv python find) /usr/local/bin/python && python --version && apt-get clean && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*
WORKDIR /app
RUN git clone https://github.com/mholzen/workflowy . && git checkout d1762e7328c15262609d22bab09f458ec50e9015
RUN (curl -OL https://go.dev/dl/go1.21.10.linux-amd64.tar.gz) && (tar -xzf go1.21.10.linux-amd64.tar.gz) && (./go/bin/go build ./cmd/workflowy)
CMD ["mcp-proxy","./workflowy","--","mcp"]
