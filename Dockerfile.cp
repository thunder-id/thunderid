# ----------------------------------------------------------------------------
# Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
#
# WSO2 LLC. licenses this file to you under the Apache License,
# Version 2.0 (the "License"); you may not use this file except
# in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing,
# software distributed under the License is distributed on an
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
# KIND, either express or implied. See the License for the
# specific language governing permissions and limitations
# under the License.
# ----------------------------------------------------------------------------

# Control Plane image.
#
# The Control Plane is where configuration is authored, versioned, and promoted between environments.
# It serves no runtime traffic; that is the Data Plane's (see Dockerfile.dp).
#
# The distribution layout, the scripts, and the databases are the product's own, so this builds them
# the same way the hybrid image does and swaps in the Control Plane binary, configuration, and
# console. The binary is installed under the name start.sh expects, so nothing downstream has to know
# which plane it is running.

FROM golang:1.26-alpine3.23 AS builder

RUN apk add --no-cache git make bash sqlite openssl zip nodejs npm curl python3 g++ build-base sqlite-dev

ENV CI=true

WORKDIR /app

COPY . .

# Serve on every interface rather than on loopback, which is what a container needs.
RUN sed -i 's/hostname: "localhost"/hostname: "0.0.0.0"/' backend/cmd/cpserver/deployment.yaml

# Security material (TLS/JWT/AES keys) is not baked into the image; the setup step generates it per
# deployment.

ARG TARGETARCH
RUN if [ "$TARGETARCH" = "amd64" ]; then \
        ./build.sh build linux amd64; \
    else \
        ./build.sh build linux arm64; \
    fi

# The distribution above carries the hybrid binary. Build the Control Plane one alongside it and let
# the runtime stage install it in place of the packaged binary. The version is stamped from the same
# file the distribution takes it from, so the two do not disagree.
RUN mkdir -p target/out && \
    GOOS=linux GOARCH=$TARGETARCH CGO_ENABLED=0 go build -C backend \
        -ldflags "-X \"main.version=$(cat version.txt)\" \
        -X \"main.buildDate=$(date -u '+%Y-%m-%d %H:%M:%S UTC')\"" \
        -o ../target/out/thunderid-cp ./cmd/cpserver

# The Control Plane console is a separate app: it compiles the Data Plane console's source in full and
# ships a configuration that gates the navigation down to authoring. The build above already produces
# it, because build.sh builds every frontend workspace app, so this only asserts it is there rather
# than building it a second time.
RUN test -f frontend/apps/cp-console/dist/index.html

# Runtime stage
FROM alpine:3.19

# postgresql-client rather than sqlite: these planes run against PostgreSQL, because several pods of
# one deployment share a database and SQLite cannot be shared. psql is here so an operator can reach
# that database from the container.
RUN apk add --no-cache \
    ca-certificates \
    lsof \
    postgresql-client \
    bash \
    curl \
    openssl \
    unzip

RUN addgroup -S thunderid -g 10001 && adduser -S thunderid -u 10001 -G thunderid

WORKDIR /opt/thunderid

ARG TARGETARCH
COPY --from=builder /app/target/dist/ /tmp/dist/
RUN cd /tmp/dist && \
    if [ "$TARGETARCH" = "amd64" ]; then \
        find . -name "thunderid-*-linux-x64.zip" | grep -E '^.*/thunderid-v?[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9]+(-[A-Z]+)?)?-linux-x64\.zip$' | xargs -I{} cp {} /tmp/ ; \
    else \
        find . -name "thunderid-*-linux-arm64.zip" | grep -E '^.*/thunderid-v?[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9]+(-[A-Z]+)?)?-linux-arm64\.zip$' | xargs -I{} cp {} /tmp/ ; \
    fi && \
    cd /tmp && \
    unzip thunderid-*.zip && \
    cp -r thunderid-*/* /opt/thunderid/ && \
    rm -rf /tmp/thunderid-* /tmp/dist

# Install the Control Plane binary and configuration over the packaged hybrid ones. start.sh runs
# ./thunderid, so the binary keeps that name.
COPY --from=builder /app/target/out/thunderid-cp /opt/thunderid/thunderid
COPY --from=builder /app/backend/cmd/cpserver/deployment.yaml /opt/thunderid/deployment.yaml

# The Control Plane's additions to the baseline, alongside the packaged bundle rather than replacing
# it: bootstrap reads the directory as one, so a tenant is provisioned from both files together.
COPY --from=builder /app/backend/cmd/cpserver/bootstrap/ /opt/thunderid/bootstrap/

# Serve the Control Plane console in place of the packaged one. The gate is dropped with it: it is the
# runtime login UI, and this plane serves no runtime traffic.
RUN rm -rf /opt/thunderid/apps/console /opt/thunderid/apps/gate
COPY --from=builder /app/frontend/apps/cp-console/dist/ /opt/thunderid/apps/console/

# Where environments and their captured versions are kept. Mount a volume over it: losing it loses the
# version history that promotion compares against.
RUN mkdir -p /opt/thunderid/repository/envmgr

RUN chown -R thunderid:thunderid /opt/thunderid && \
    chmod +x thunderid start.sh setup.sh scripts/init_script.sh && \
    (find bootstrap -name "*.sh" -type f -exec chmod +x {} \; 2>/dev/null || true) && \
    chmod 700 /opt/thunderid/repository/envmgr

EXPOSE 8095

USER thunderid

ENV BACKEND_PORT=8095

CMD ["./start.sh"]
