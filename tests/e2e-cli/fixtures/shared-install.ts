// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Shared install
 *
 * A distribution is ~85MB extracted and takes about ten seconds to download and set up, so the
 * suite installs one and reuses it. The `install` project populates it; the `repl` and `restart`
 * projects declare a dependency on that project and read it back here.
 *
 * The workspace lives at a fixed path rather than a temp directory precisely because it has to
 * outlive the process that created it: Playwright runs each project in its own worker.
 */

import fs from "fs";
import path from "path";
import { Workspace } from "../utils/workspace";

/** Where the shared install lives. Gitignored, and cleared by global setup on every run. */
export function sharedRoot(): string {
  return path.resolve(__dirname, "..", "test-results", "shared-install");
}

/** Creates the shared workspace. Called by the install spec before it installs anything. */
export function createShared(): Workspace {
  return Workspace.at(sharedRoot());
}

/**
 * Reads back the workspace the install spec populated.
 *
 * Throws rather than silently installing again: a missing install means the dependency graph did
 * not run the install project first, and quietly re-downloading would hide that.
 */
export function useShared(): Workspace {
  const workspace = Workspace.at(sharedRoot());
  const state = workspace.state();
  if (!state.active) {
    throw new Error(
      `No shared install at ${sharedRoot()}. The install project has to run first; ` +
        `run the whole suite rather than this spec on its own.`
    );
  }
  return workspace;
}

/** The version the shared install is on. */
export function sharedVersion(): string {
  const active = useShared().state().active;
  return active as string;
}

/** Removes the shared install. Called by global setup so every run starts clean. */
export function clearShared(): void {
  fs.rmSync(sharedRoot(), { recursive: true, force: true });
}
