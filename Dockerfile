# syntax=docker/dockerfile:1.7

FROM golang:1.27.3-alpine@sha256:f2c872bcee949b7be9c490efd02b48a8a2fbf947cdf533f1c58525ec9d8170fd AS build
WORKDIR /src
ENV CGO_ENABLED=0 GOFLAGS=-trimpath
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -ldflags="-s -w" -o /out/awt ./cmd/awt

# Litestream 0.3.13 official image: a static binary.
FROM litestream/litestream:0.3.13@sha256:027eda2a89a86015b9797d2129d4dd447e8953097b4190e1d5a30b73e76d8d58 AS litestream

FROM alpine:3.22@sha256:5291449c3df73caf6ed85e649dec1b9e818b39a5d8c871e97afc13e9cd5e8fa8
# Why alpine and not distroless: the entrypoint is a POSIX sh script (restore,
# then replicate), wget gives the container a health check, and sqlite3 backs
# the weekly restore check. See docs/decisions/0013-runtime-image.md.
RUN apk add --no-cache ca-certificates tzdata sqlite \
 && addgroup -S -g 10001 awt && adduser -S -u 10001 -G awt awt \
 && mkdir /data && chown awt:awt /data
COPY --from=build /out/awt /usr/local/bin/awt
COPY --from=litestream /usr/local/bin/litestream /usr/local/bin/litestream
COPY litestream.yml /etc/litestream.yml
COPY deploy/entrypoint.sh /usr/local/bin/entrypoint.sh
# The VPS deploy agent (deploy/agent.sh) installs these from the image it
# deploys, so config and code always ship together. See docs/decisions/0016.
COPY compose.yaml Caddyfile /usr/share/awt/deploy/
# Commit the image was built from; the agent refuses an image without it.
ARG REVISION=unknown
LABEL org.opencontainers.image.revision=$REVISION \
      org.opencontainers.image.source=https://github.com/fatpat/acroworldtour.com
USER awt
ENV AWT_DB_PATH=/data/awt.db AWT_BLOB_DIR=/data/blobs AWT_ADDR=:8080
VOLUME /data
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=3s --start-period=20s --retries=3 \
  CMD wget -q -O /dev/null http://127.0.0.1:8080/healthz || exit 1
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
CMD ["serve"]
