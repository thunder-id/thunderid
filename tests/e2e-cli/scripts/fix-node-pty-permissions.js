// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Restores the executable bit on node-pty's spawn-helper.
 *
 * node-pty ships prebuilt binaries, and the extraction into the pnpm store drops the mode bits on
 * `spawn-helper`. The native addon then loads fine but every pty.spawn() fails with the opaque
 * "posix_spawnp failed", because the helper it execs is not executable. Without this the whole
 * suite fails on a fresh install.
 *
 * Not needed on Windows, which uses conpty rather than a helper binary.
 */

const fs = require("fs");
const path = require("path");

if (process.platform === "win32") process.exit(0);

let packageDir;
try {
  packageDir = path.dirname(require.resolve("node-pty/package.json"));
} catch {
  // node-pty is absent (a lint- or type-check-only install), so there is nothing to fix.
  process.exit(0);
}

const candidates = [
  path.join(packageDir, "prebuilds", `${process.platform}-${process.arch}`, "spawn-helper"),
  path.join(packageDir, "build", "Release", "spawn-helper"),
];

for (const helper of candidates) {
  if (!fs.existsSync(helper)) continue;
  const mode = fs.statSync(helper).mode;
  if (mode & 0o111) continue;
  fs.chmodSync(helper, 0o755);
  console.log(`node-pty: restored the executable bit on ${path.relative(process.cwd(), helper)}`);
}
