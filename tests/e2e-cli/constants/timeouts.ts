// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Timeouts
 *
 * The install budget covers downloading and extracting a real distribution (~40MB compressed)
 * plus setup.sh seeding the database, so it is deliberately generous.
 */
export const Timeouts = {
  /** Waiting on a frame the TUI should already be rendering. */
  SHORT: 30_000,
  /** Waiting for the server to boot and answer. */
  STARTUP: 180_000,
  /** Waiting on a fetch from the docs site. */
  REMOTE: 60_000,
  /** Waiting for a download, extract and setup to finish. */
  INSTALL: 600_000,
  /** Per-test budget for a spec that installs a distribution. */
  INSTALL_TEST: 900_000,
  /**
   * Waiting for `try` to bring a sample up. It downloads the sample bundle, runs a cold
   * `npm install` across a five-package workspace, restarts ThunderID and then waits on Vite,
   * so this is minutes rather than seconds on a first run.
   */
  SAMPLE: 900_000,
  /** Per-test budget for a spec that launches a sample. */
  SAMPLE_TEST: 1_200_000,
  /** A browser action against the sample app or the gate. */
  BROWSER: 60_000,
} as const;
