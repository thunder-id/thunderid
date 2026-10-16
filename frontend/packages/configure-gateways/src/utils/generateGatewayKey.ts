// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Generates a random key for a gateway: 32 bytes from the platform's secure random source,
 * encoded as unpadded base64url so it survives being pasted into a configuration file.
 */
export default function generateGatewayKey(): string {
  const bytes = new Uint8Array(32);
  crypto.getRandomValues(bytes);
  return btoa(String.fromCharCode(...bytes))
    .replace(/\+/g, '-')
    .replace(/\//g, '_')
    .replace(/=+$/, '');
}
