// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/** The version output. Needs no install and no network. */

import { test, expect } from "@playwright/test";
import { CliSession } from "../../utils/cli-session";
import { Workspace } from "../../utils/workspace";

test.describe("thunderid --version", () => {
  let workspace: Workspace;
  let session: CliSession;

  test.beforeEach(() => {
    workspace = Workspace.create();
  });

  test.afterEach(async () => {
    await session?.dispose();
    workspace.remove();
  });

  for (const flag of ["--version", "-V"]) {
    test(`${flag} prints the CLI version and exits`, async () => {
      session = new CliSession(workspace, [flag]);

      expect(await session.waitForExit()).toBe(0);
      expect(session.output.trim()).toMatch(/^thunderid \S+$/);
    });
  }

  test("takes precedence over a command", async () => {
    session = new CliSession(workspace, ["upgrade", "--version"]);

    expect(await session.waitForExit()).toBe(0);
    expect(session.output.trim()).toMatch(/^thunderid \S+$/);
  });

  test("is documented in the usage output", async () => {
    session = new CliSession(workspace, ["--help"]);

    expect(await session.waitForExit()).toBe(0);
    expect(session.output).toContain("--version, -V");
  });
});
