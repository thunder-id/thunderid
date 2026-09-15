// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/** The usage output. Needs no install and no network. */

import { test, expect } from "@playwright/test";
import { CliSession } from "../../utils/cli-session";
import { Workspace } from "../../utils/workspace";

test.describe("thunderid --help", () => {
  let workspace: Workspace;
  let session: CliSession;

  test.beforeEach(() => {
    workspace = Workspace.create();
  });

  test.afterEach(async () => {
    await session?.dispose();
    workspace.remove();
  });

  for (const flag of ["--help", "-h"]) {
    test(`${flag} lists the available commands`, async () => {
      session = new CliSession(workspace, [flag]);

      expect(await session.waitForExit()).toBe(0);

      for (const phrase of [
        "Usage: thunderid [command] [flags]",
        "upgrade Upgrade to the latest release",
        "try <usecase> Download and launch a use-case sample app",
        "--verbose, -v",
        "--setup",
      ]) {
        expect(session.output, `help output should mention ${JSON.stringify(phrase)}`).toContain(phrase);
      }
    });
  }
});
