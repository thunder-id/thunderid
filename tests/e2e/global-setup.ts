// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Global Setup
 *
 * Runs once before all tests. Use for expensive operations
 * that only need to happen once per test run.
 */

import dotenv from "dotenv";
import path from "path";
import fs from "fs";

async function globalSetup() {
  console.log("🚀 Running global setup...");

  // Load environment variables
  const envPath = path.resolve(__dirname, ".env");
  dotenv.config({ path: envPath });

  // Verify required environment variables
  const requiredVars = ["BASE_URL", "ADMIN_USERNAME", "ADMIN_PASSWORD"];
  const missingVars = requiredVars.filter(varName => !process.env[varName]);

  if (missingVars.length > 0) {
    console.error("❌ Missing required environment variables:", missingVars.join(", "));
    console.error("Please create a .env file based on .env.example");
    process.exit(1);
  }

  // Clear cached per-worker auth files from a previous run. checkTokensExpired() in
  // console-admin-auth-utils.ts only looks at a token's own expiry claim - it has no way to tell
  // that a not-yet-expired token was signed by a server that has since restarted (a new signing
  // key), so a stale file would otherwise be replayed against a server that will reject it. The
  // per-worker `authenticatedPage` fixture recreates this directory lazily on its own first login.
  const authDir = path.join(__dirname, "playwright/.auth");
  if (fs.existsSync(authDir)) {
    fs.rmSync(authDir, { recursive: true, force: true });
  }

  console.log("✅ Global setup complete");
  console.log(`   Base URL: ${process.env.BASE_URL}`);
  console.log(`   Admin User: ${process.env.ADMIN_USERNAME}`);
}

export default globalSetup;
