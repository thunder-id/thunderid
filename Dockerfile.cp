# Copyright 2026 The ThunderID Authors
# SPDX-License-Identifier: Apache-2.0

# Control Plane Docker Image
# Built from source for `make docker-build-cp*`. The release pipeline uses
# .github/docker/Dockerfile.release.cp. Keep the runtime stages in sync, and with ./Dockerfile
# (the Data Plane image).
# Build stage - compile the Control Plane binary and build the frontend for the target architecture
FROM golang:1.27-alpine3.23 AS builder

# Install build dependencies including Node.js and npm
RUN apk add --no-cache git make bash sqlite openssl zip nodejs npm curl python3 g++ build-base sqlite-dev

# Set environment variables for CI build
ENV CI=true

# Set the working directory
WORKDIR /app

# Copy the entire source code
COPY . .

# Modify the hostname in the deployment configuration
RUN sed -i 's/hostname: "localhost"/hostname: "0.0.0.0"/' backend/cmd/cpserver/deployment.yaml && \
    sed -i '/hostname: "0.0.0.0"/a\  public_url: "https://localhost:8095"' backend/cmd/cpserver/deployment.yaml

# Security material (TLS/JWT/AES keys) is not baked into the image; the setup step generates it per deployment.

# Build the frontend and the Control Plane for the target architecture
ARG TARGETARCH
RUN ./build.sh build_frontend && \
    if [ "$TARGETARCH" = "amd64" ]; then \
        ./build.sh build_cp_backend linux amd64; \
    else \
        ./build.sh build_cp_backend linux arm64; \
    fi

# List the contents of the dist directory to verify zip output
RUN ls -l /app/target/dist/

# Runtime stage
FROM alpine:3.23

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
WORKDIR /opt/thunderid-cp

# Copy and extract the package from builder stage
# TARGETARCH is automatically set by Docker during multi-arch builds
ARG TARGETARCH
COPY --from=builder /app/target/dist/ /tmp/dist/
RUN cd /tmp/dist && \
    if [ "$TARGETARCH" = "amd64" ]; then \
        find . -name "thunderid-cp-*-linux-x64.zip" | xargs -I{} cp {} /tmp/ ; \
    else \
        find . -name "thunderid-cp-*-linux-arm64.zip" | xargs -I{} cp {} /tmp/ ; \
    fi && \
    cd /tmp && \
    unzip thunderid-cp-*.zip && \
    cp -r thunderid-cp-*/* /opt/thunderid-cp/ && \
    rm -rf /tmp/thunderid-cp-* /tmp/dist

# Set ownership and permissions
RUN chown -R thunderid:thunderid /opt/thunderid-cp && \
    chmod +x thunderid-cp start-cp.sh setup-cp.sh scripts/init_script.sh

# Expose the default port
EXPOSE 8095

# Switch to user
USER thunderid

# Set environment variables
ENV BACKEND_PORT=8095

# Start the application
CMD ["./start-cp.sh"]
