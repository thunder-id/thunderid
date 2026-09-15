// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * A configured install cannot be moved: setup seeded the console application's redirect URIs with
 * its port, so serving the console anywhere else would leave a login that cannot complete. The
 * only answers are to reclaim the port or to abort.
 */

import { test, expect } from "@playwright/test";
import { CliSession, Keys } from "../../utils/cli-session";
import { useShared } from "../../fixtures/shared-install";
import { DEFAULT_PORT, PortHolder } from "../../utils/ports";
import { Timeouts } from "../../constants/timeouts";

test.describe("port conflict on a configured install", () => {
  test("refuses to move and can be aborted", async () => {
    const holder = await PortHolder.start(DEFAULT_PORT);
    const session = new CliSession(useShared(), [], { preconfigured: true });

    try {
      await session.expect("is already in use", Timeouts.STARTUP);
      await session.expect("This install is already set up on port");
      await session.expect("cannot be moved");
      await session.expect("Abort");

      // With no alternate offered the options are kill and abort, so abort is second.
      await session.send(Keys.DOWN, Keys.ENTER);

      expect(await session.waitForExit(), "aborting is a choice, not a failure").toBe(0);
      expect(holder.alive, "aborting must not kill the holder").toBe(true);
      expect(await session.waitFor("ThunderID started on port", 0), "aborting must not start the server").toBe(false);
    } finally {
      await session.dispose();
      await holder.stop();
    }
  });
});
