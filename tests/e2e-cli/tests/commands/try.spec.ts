// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/** The try command's refusal path. Launching a sample is covered in tests/sample. */

import { test, expect } from "@playwright/test";
import { CliSession } from "../../utils/cli-session";
import { Workspace } from "../../utils/workspace";

test.describe("thunderid try", () => {
  let workspace: Workspace;
  let session: CliSession;

  test.beforeEach(() => {
    workspace = Workspace.create();
  });

  test.afterEach(async () => {
    await session?.dispose();
    workspace.remove();
  });

  // With no install there is nothing to launch, and the message has to point at the command that
  // creates one rather than failing deeper inside the sample runner.
  test("without an install it directs the user to install first", async () => {
    session = new CliSession(workspace, ["try", "consumer"]);

    await session.expect("No active ThunderID install found. Run `npx thunderid` first.");
    expect(await session.waitForExit()).toBe(1);
    expect(workspace.state().active, "a failed try must not record state").toBeUndefined();
  });
});
