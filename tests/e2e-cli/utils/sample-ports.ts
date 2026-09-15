// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * The fixed ports the Wayfinder sample binds, from sampleServicePorts in
 * internal/commands/sample/sample.go. None are configurable, and the bundle's OAuth redirect
 * URIs hardcode the frontend origin, so the sample cannot be moved off them.
 */

import { DEFAULT_PORT, freePort } from "./ports";

export const SAMPLE_PORTS = {
  /** Vite frontend. */
  FRONTEND: 5173,
  /** Backend API. */
  BACKEND: 8787,
  /** SMTP inbox UI. */
  INBOX: 8788,
  /** SMTP server. */
  SMTP: 2525,
  /** Lounge kiosk. */
  LOUNGE: 8795,
} as const;

/**
 * Releases everything a sample run leaves behind.
 *
 * `try` installs no signal handler and never calls StopServices, so the CLI exits with npm and
 * all five services still running, plus the ThunderID server it restarted. Cleaning up is
 * entirely the caller's job.
 */
export async function stopSample(): Promise<void> {
  for (const port of [...Object.values(SAMPLE_PORTS), DEFAULT_PORT]) {
    await freePort(port);
  }
}
