# ----------------------------------------------------------------------------
# Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

# Product Docker Image
# Build stage - compile the Go binary and build frontend for the target architecture
FROM golang:1.26-alpine3.23 AS builder

# Install build dependencies including Node.js and npm
RUN apk add --no-cache git make bash sqlite openssl zip nodejs npm curl

# Set environment variables for CI build
ENV CI=true

# Set the working directory
WORKDIR /app

# Copy the entire source code
COPY . .

# Accept build arguments for certificate files
ARG CERT_FILE
ARG KEY_FILE

# Modify the hostname in the deployment configuration
RUN sed -i 's/hostname: "localhost"/hostname: "0.0.0.0"/' backend/cmd/server/deployment.yaml && \
    sed -i '/hostname: "0.0.0.0"/a\  public_url: "https://localhost:8090"' backend/cmd/server/deployment.yaml

# Handle shared certificates - use provided certificates or generate new ones
RUN if [ -n "$CERT_FILE" ] && [ -n "$KEY_FILE" ] && [ -f "$CERT_FILE" ] && [ -f "$KEY_FILE" ]; then \
        echo "🔐 Using shared certificates: $CERT_FILE and $KEY_FILE"; \
        mkdir -p target/out/.cert; \
        cp "$CERT_FILE" target/out/.cert/server.cert; \
        cp "$KEY_FILE" target/out/.cert/server.key; \
        echo "✅ Shared certificates copied successfully"; \
    else \
        echo "🔐 Generating new certificates (shared certificates not found)"; \
        mkdir -p target/out/.cert; \
        openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
            -keyout target/out/.cert/server.key \
            -out target/out/.cert/server.cert \
            -subj "/O=WSO2/OU=ThunderID/CN=localhost"; \
        echo "✅ New certificates generated"; \
    fi

# Build both frontend and backend for the target architecture
ARG TARGETARCH
ARG WITH_CONSENT=true
RUN WITHOUT_CONSENT=$([ "$WITH_CONSENT" = "false" ] && echo "true" || echo "false") && \
    export WITHOUT_CONSENT && \
    if [ "$TARGETARCH" = "amd64" ]; then \
        ./build.sh build linux amd64; \
    else \
        ./build.sh build linux arm64; \
    fi

# List the contents of the dist directory to verify zip output
RUN ls -l /app/target/dist/

# Runtime stage
FROM alpine:3.19

# Install required packages
RUN apk add --no-cache \
    ca-certificates \
    lsof \
    sqlite \
    bash \
    curl \
    unzip

# Create user and group
RUN addgroup -S thunderid -g 10001 && adduser -S thunderid -u 10001 -G thunderid

# Create application directory
WORKDIR /opt/thunderid

# Copy and extract the package from builder stage
# TARGETARCH is automatically set by Docker during multi-arch builds
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

# Set ownership and permissions
RUN chown -R thunderid:thunderid /opt/thunderid && \
    chmod +x thunderid start.sh setup.sh scripts/init_script.sh && \
    (find consent -name "consent-server" -o -name "start.sh" 2>/dev/null | xargs -r chmod +x) && \
    (find bootstrap -name "*.sh" -type f -exec chmod +x {} \; 2>/dev/null || true)

# Expose the default port
EXPOSE 8090

# Switch to user
USER thunderid

# Set environment variables
ENV BACKEND_PORT=8090
ARG WITH_CONSENT=true
ENV WITH_CONSENT=${WITH_CONSENT}

# Start the application
CMD ["./start.sh"]
