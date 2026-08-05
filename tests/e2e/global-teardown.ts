// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Global Teardown
 *
 * Runs once after all tests. Use for cleanup operations.
 */

import fs from "fs";
import path from "path";

async function globalTeardown() {
  console.log("🧹 Running global teardown...");

  // Optional: Clean up old auth files (keep the latest)
  const authDir = path.join(__dirname, "playwright/.auth");
  if (fs.existsSync(authDir)) {
    const files = fs.readdirSync(authDir);
    const oldFiles = files.filter(f => f.startsWith("working-login") && f !== "working-login.json");

    oldFiles.forEach(file => {
      try {
        fs.unlinkSync(path.join(authDir, file));
      } catch (err) {
        console.warn(`Could not delete ${file}:`, err);
      }
    });
  }

  console.log("✅ Global teardown complete");
}

export default globalTeardown;
