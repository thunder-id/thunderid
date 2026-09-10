# Copyright 2025 The ThunderID Authors
# SPDX-License-Identifier: Apache-2.0

# Product Docker Image
# Built from source for `make docker-build*`. The release pipeline uses
# .github/docker/Dockerfile.release. Keep the runtime stages in sync.
# Build stage - compile the Go binary and build frontend for the target architecture
FROM golang:1.26-alpine3.23 AS builder

# Install build dependencies including Node.js and npm
RUN apk add --no-cache git make bash sqlite openssl zip nodejs npm curl python3 g++ build-base sqlite-dev

# Set environment variables for CI build
ENV CI=true

# Set the working directory
WORKDIR /app

# Copy the entire source code
COPY . .

# Modify the hostname in the deployment configuration
RUN sed -i 's/hostname: "localhost"/hostname: "0.0.0.0"/' backend/cmd/server/deployment.yaml && \
    sed -i '/hostname: "0.0.0.0"/a\  public_url: "https://localhost:8090"' backend/cmd/server/deployment.yaml

# Security material (TLS/JWT/AES keys) is not baked into the image; the setup step generates it per deployment.

# Build both frontend and backend for the target architecture
ARG TARGETARCH
RUN if [ "$TARGETARCH" = "amd64" ]; then \
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
    openssl \
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
    (find bootstrap -name "*.sh" -type f -exec chmod +x {} \; 2>/dev/null || true)

# Expose the default port
EXPOSE 8090

# Switch to user
USER thunderid

# Set environment variables
ENV BACKEND_PORT=8090

# Start the application
CMD ["./start.sh"]
