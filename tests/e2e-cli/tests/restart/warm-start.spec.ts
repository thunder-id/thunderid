// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/** Running the CLI again against an install that is already set up. */

import { test, expect } from "@playwright/test";
import { CliSession } from "../../utils/cli-session";
import { useShared } from "../../fixtures/shared-install";
import { DEFAULT_PORT, freePort, waitForReady } from "../../utils/ports";
import { Timeouts } from "../../constants/timeouts";

test.describe("warm start", () => {
  test("reuses the install instead of downloading and setting up again", async () => {
    const workspace = useShared();
    const version = workspace.state().active as string;
    const session = new CliSession(workspace, [], { preconfigured: true });

    try {
      await session.expect("Starting ThunderID", Timeouts.STARTUP);
      await session.expect(`ThunderID v${version} is ready`, Timeouts.STARTUP);
      await session.expect("ThunderID started on port", Timeouts.STARTUP);

      expect(await session.waitFor("installed to", 0), "a warm start must not re-download").toBe(false);
      expect(await session.waitFor("Setup complete", 0), "a warm start must not re-run setup").toBe(false);

      // Setup fixed the port when it seeded the console's redirect URIs, so a warm start has to
      // come back up on the same one.
      expect(session.startedPort()).toBe(DEFAULT_PORT);
      expect(await waitForReady(DEFAULT_PORT), `server should answer readiness\n\n${session.tail(40)}`).toBe(true);
    } finally {
      await session.dispose();
      await freePort(DEFAULT_PORT);
    }
  });
});
