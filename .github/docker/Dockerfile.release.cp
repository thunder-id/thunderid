# Copyright 2026 The ThunderID Authors
# SPDX-License-Identifier: Apache-2.0

# Control Plane release image. The release pipeline packages the Control Plane archives first
# (see Dockerfile.release for the Data Plane image); this only unpacks the one for the target
# architecture. Keep the runtime stage in sync with ./Dockerfile.cp.
# Unpack the target architecture's archive, on the build platform to avoid emulation.
FROM --platform=$BUILDPLATFORM alpine:3.23 AS dist

ARG TARGETARCH

RUN apk add --no-cache unzip

WORKDIR /dist

COPY thunderid-cp-*-linux-*.zip ./

RUN set -eux; \
    case "$TARGETARCH" in \
      amd64) archive_arch=x64 ;; \
      arm64) archive_arch=arm64 ;; \
      *) echo "unsupported target architecture: $TARGETARCH" >&2; exit 1 ;; \
    esac; \
    archive="$(ls thunderid-cp-*-linux-"$archive_arch".zip)"; \
    unzip -q "$archive" -d /extracted; \
    mkdir -p /opt/thunderid-cp; \
    cp -r /extracted/thunderid-cp-*/. /opt/thunderid-cp/

# ./Dockerfile.cp edits these before compiling; here the archive is already packaged.
RUN sed -i 's/hostname: "localhost"/hostname: "0.0.0.0"/' /opt/thunderid-cp/deployment.yaml && \
    sed -i '/hostname: "0.0.0.0"/a\  public_url: "https://localhost:8095"' /opt/thunderid-cp/deployment.yaml

# Security material (TLS/JWT/AES keys) is not baked into the image; the setup step generates it per deployment.

# Runtime stage
FROM alpine:3.23

RUN apk add --no-cache \
    ca-certificates \
    lsof \
    sqlite \
    bash \
    curl \
    openssl \
    unzip

RUN addgroup -S thunderid -g 10001 && adduser -S thunderid -u 10001 -G thunderid

WORKDIR /opt/thunderid-cp

COPY --from=dist --chown=thunderid:thunderid /opt/thunderid-cp /opt/thunderid-cp

RUN chmod +x thunderid-cp start-cp.sh setup-cp.sh scripts/init_script.sh

EXPOSE 8095

USER thunderid

ENV BACKEND_PORT=8095

CMD ["./start-cp.sh"]
