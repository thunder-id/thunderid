// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Whether a client is the Console's own, ignoring case. The Console's client id comes from its runtime
 * config and defaults to `CONSOLE`.
 *
 * @param clientId - The application's OAuth client id, if it has one.
 * @param consoleClientId - The Console's configured client id (`config.client.client_id`).
 */
export default function isSystemConsoleClient(
  clientId: string | undefined,
  consoleClientId: string | undefined,
): boolean {
  return clientId?.toUpperCase() === (consoleClientId ?? 'CONSOLE').toUpperCase();
}
