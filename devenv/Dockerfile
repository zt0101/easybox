# EasyBox dev image: Go + Node + C toolchain + Playwright, so nothing is installed on the host.
FROM node:26-bookworm-slim AS node

FROM golang:1.27-bookworm
ARG TARGETARCH

RUN apt-get update && apt-get install -y --no-install-recommends \
      build-essential curl unzip git make ca-certificates sqlite3 jq \
    && rm -rf /var/lib/apt/lists/*

COPY --from=node /usr/local/bin/ /usr/local/bin/
COPY --from=node /usr/local/lib/node_modules/ /usr/local/lib/node_modules/
COPY --from=node /usr/local/include/node/ /usr/local/include/node/

RUN curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh \
      | sh -s -- -b /usr/local/bin

# Chromium for the Storybook browser tests in `make verify`; version pinned to frontend/package-lock.json.
ENV PLAYWRIGHT_BROWSERS_PATH=/ms-playwright
RUN npx -y playwright@1.64.0 install --with-deps chromium \
    && rm -rf /var/lib/apt/lists/* /root/.npm

# Xray / mtg / geodata exactly as the upstream image fetches them.
COPY DockerInit.sh /tmp/DockerInit.sh
RUN mkdir -p /opt/xui && cd /opt/xui && sh /tmp/DockerInit.sh "${TARGETARCH}" && rm /tmp/DockerInit.sh

ENV CGO_ENABLED=1 \
    XUI_BIN_FOLDER=/opt/xui/build/bin \
    XUI_DB_FOLDER=/etc/x-ui \
    XUI_LOG_FOLDER=/var/log/x-ui \
    XUI_DEBUG=false

RUN git config --global --add safe.directory /src
WORKDIR /src
