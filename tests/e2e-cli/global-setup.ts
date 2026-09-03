// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Global Setup
 *
 * Builds the host binary into the npx wrapper's dist directory. Without it the wrapper falls back
 * to `go run` against a tools/npx/cli directory that only exists in the published package, so the
 * specs would exercise a path users never take.
 */

import { execFile } from "child_process";
import fs from "fs";
import path from "path";
import { promisify } from "util";
import { clearShared } from "./fixtures/shared-install";
import { localDistDir, startDistServer } from "./utils/release-source";
import { publishedTag } from "./utils/cli-session";

const execFileAsync = promisify(execFile);

/** Mirrors the platform and architecture maps in tools/npx/bin/thunderid.js. */
const PLATFORMS: Record<string, string> = { darwin: "darwin", linux: "linux" };
const ARCHITECTURES: Record<string, string> = { x64: "x64", arm64: "arm64" };

async function globalSetup() {
  // The shared install is deliberately at a fixed path so it can outlive the worker that creates
  // it, which also means a previous run's copy would otherwise be inherited.
  clearShared();

  const teardowns: Array<() => Promise<void>> = [];

  // Serve a locally built distribution when one is configured, so the CLI installs the product
  // from this checkout rather than from thunderid.dev.
  const dist = localDistDir();
  if (dist) {
    teardowns.push(await startDistServer(dist));
  }

  const tag = publishedTag();
  if (tag) {
    // Nothing to build: this run drives the CLI as published, which is the point of the mode.
    console.log(`📦 Driving the published CLI: npx thunderid@${tag}`);
    return async () => {
      for (const stop of teardowns) await stop();
    };
  }

  console.log("🔧 Building the ThunderID CLI for the npx wrapper...");

  const platform = PLATFORMS[process.platform];
  const arch = ARCHITECTURES[process.arch];
  if (!platform || !arch) {
    throw new Error(`Unsupported platform: ${process.platform}/${process.arch}`);
  }

  const cliDir = path.resolve(__dirname, "..", "..", "tools", "cli");
  const distDir = path.resolve(__dirname, "..", "..", "tools", "npx", "dist");
  fs.mkdirSync(distDir, { recursive: true });

  const binary = path.join(distDir, `thunderid-${platform}-${arch}`);
  await execFileAsync("go", ["build", "-o", binary, "./cmd/thunderid"], { cwd: cliDir });

  console.log(`✅ Built ${path.relative(process.cwd(), binary)}`);

  return async () => {
    for (const stop of teardowns) await stop();
  };
}

export default globalSetup;
