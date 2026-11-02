// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Workspace
 *
 * An isolated pair of directories standing in for a user's machine.
 *
 * `home` redirects the CLI's state file (config.StateDir reads $HOME) and `dir` is the working
 * directory the CLI installs into (cli.BaseDir is "./thunderid"). Together they keep a test run
 * from seeing, or disturbing, a real install on the developer's machine.
 */

import fs from "fs";
import os from "os";
import path from "path";

/** The on-disk shape written by internal/services/config. */
export interface CliState {
  active?: string;
  versions?: Record<
    string,
    {
      installPath?: string;
      setupComplete?: boolean;
      onboardingDone?: boolean;
    }
  >;
}

export class Workspace {
  private constructor(
    readonly home: string,
    readonly dir: string
  ) {}

  /** A throwaway workspace under the OS temp directory, for a spec that needs a clean machine. */
  static create(): Workspace {
    return Workspace.at(fs.mkdtempSync(path.join(os.tmpdir(), "thunderid-e2e-")));
  }

  /**
   * A workspace rooted at a fixed path, for the install shared between projects. Existing
   * contents are kept, so a later project reads back what the install project wrote.
   */
  static at(root: string): Workspace {
    const home = path.join(root, "home");
    const dir = path.join(root, "work");
    fs.mkdirSync(home, { recursive: true });
    fs.mkdirSync(dir, { recursive: true });
    return new Workspace(home, dir);
  }

  /** The CLI's persisted state, or an empty object when it has not written one yet. */
  state(): CliState {
    const file = path.join(this.home, ".thunderid", "state.json");
    if (!fs.existsSync(file)) return {};
    return JSON.parse(fs.readFileSync(file, "utf8")) as CliState;
  }

  /** The install directory the CLI extracts into, relative to its working directory. */
  installDir(version: string): string {
    return path.join(this.dir, "thunderid", `v${version}`);
  }

  /** Removes the workspace. Registered as test cleanup. */
  remove(): void {
    fs.rmSync(path.dirname(this.home), { recursive: true, force: true });
  }
}
