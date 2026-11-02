// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

const LOOPBACK_HOSTS = ['localhost', '127.0.0.1', '[::1]'];

/**
 * Whether a redirect URI returns to the user's device: `http` on a loopback address.
 */
export default function isLoopbackRedirectUri(uri: string): boolean {
  try {
    const url = new URL(uri);
    return url.protocol === 'http:' && LOOPBACK_HOSTS.includes(url.hostname);
  } catch {
    return false;
  }
}
