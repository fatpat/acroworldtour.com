# syntax=docker/dockerfile:1.7

FROM golang:1.26.10-alpine@sha256:9ce231b28998b32e116927c61aa5c406183061d4f3f2d61b292c88e7f6bdf7ca AS build
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
USER awt
ENV AWT_DB_PATH=/data/awt.db AWT_BLOB_DIR=/data/blobs AWT_ADDR=:8080
VOLUME /data
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=3s --start-period=20s --retries=3 \
  CMD wget -q -O /dev/null http://127.0.0.1:8080/healthz || exit 1
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
CMD ["serve"]
