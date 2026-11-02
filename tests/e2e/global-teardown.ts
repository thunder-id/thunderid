// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Global Teardown
 *
 * Runs once after all tests complete.
 */

import fs from "fs";
import path from "path";

async function globalTeardown() {
  // The backend restarts fresh for every run, so a per-worker auth file can still look unexpired
  // by the wall clock (see checkTokensExpired() in console-admin-auth-utils.ts) while holding a
  // token the next run's server instance has never heard of. Clearing them here forces every
  // worker to log in again next run instead of failing mid-test on a stale token.
  const authDir = path.join(__dirname, "playwright/.auth");
  if (!fs.existsSync(authDir)) {
    return;
  }

  fs.readdirSync(authDir)
    .filter(file => file.startsWith("console-admin-") && file.endsWith(".json"))
    .forEach(file => fs.unlinkSync(path.join(authDir, file)));
}

export default globalTeardown;
