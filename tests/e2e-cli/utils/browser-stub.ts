// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Browser stub
 *
 * `utils.OpenBrowser` shells out to `open` (macOS) or `xdg-open` (Linux) with no way to suppress
 * it, so a spec that exercises /open-console would launch a real browser window on whatever
 * machine it runs on.
 *
 * This puts a recording stub earlier on PATH instead. Nothing opens, and the URL the CLI asked
 * for becomes something the spec can assert, which is the interesting part of that command
 * anyway.
 */

import fs from "fs";
import os from "os";
import path from "path";

export class BrowserStub {
  private constructor(
    /** Prepend to PATH so the stub shadows the real opener. */
    readonly binDir: string,
    private readonly recordFile: string
  ) {}

  static create(): BrowserStub {
    const root = fs.mkdtempSync(path.join(os.tmpdir(), "thunderid-browser-stub-"));
    const recordFile = path.join(root, "opened.txt");
    fs.writeFileSync(recordFile, "");

    // Both names are stubbed regardless of platform: it costs nothing and keeps the helper
    // working if the suite later runs somewhere the CLI picks the other one.
    for (const name of ["open", "xdg-open"]) {
      const script = path.join(root, name);
      fs.writeFileSync(script, `#!/bin/sh\nprintf '%s\\n' "$@" >> ${JSON.stringify(recordFile)}\n`);
      fs.chmodSync(script, 0o755);
    }
    return new BrowserStub(root, recordFile);
  }

  /** Every URL the CLI asked the system to open, in order. */
  openedUrls(): string[] {
    return fs
      .readFileSync(this.recordFile, "utf8")
      .split("\n")
      .map(line => line.trim())
      .filter(Boolean);
  }

  remove(): void {
    fs.rmSync(this.binDir, { recursive: true, force: true });
  }
}
