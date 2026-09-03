// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/** The integrate command is a declared but unimplemented entry point. */

import { test, expect } from "@playwright/test";
import { CliSession } from "../../utils/cli-session";
import { Workspace } from "../../utils/workspace";

test.describe("thunderid integrate", () => {
  let workspace: Workspace;
  let session: CliSession;

  test.beforeEach(() => {
    workspace = Workspace.create();
  });

  test.afterEach(async () => {
    await session?.dispose();
    workspace.remove();
  });

  test("reports that it is not implemented", async () => {
    session = new CliSession(workspace, ["integrate", "react"]);

    await session.expect("`integrate react` is not yet implemented.");
    expect(await session.waitForExit()).toBe(1);
  });
});
