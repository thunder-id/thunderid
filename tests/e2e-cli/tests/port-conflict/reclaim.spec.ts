// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/** Reclaiming the port has to actually stop the holder before the server binds. */

import { test, expect } from "@playwright/test";
import { CliSession, Keys } from "../../utils/cli-session";
import { useShared } from "../../fixtures/shared-install";
import { DEFAULT_PORT, PortHolder, freePort, waitForReady } from "../../utils/ports";
import { Timeouts } from "../../constants/timeouts";

test.describe("reclaiming a contested port", () => {
  test("stops the holder and binds the port", async () => {
    const holder = await PortHolder.start(DEFAULT_PORT);
    const session = new CliSession(useShared(), [], { preconfigured: true });

    try {
      await session.expect("is already in use", Timeouts.STARTUP);
      await session.expect("and continue");

      // The kill option is preselected.
      await session.send(Keys.ENTER);

      await session.expect("ThunderID started on port", Timeouts.STARTUP);
      expect(session.startedPort()).toBe(DEFAULT_PORT);
      expect(holder.alive, "the holder should have been stopped").toBe(false);
      expect(await waitForReady(DEFAULT_PORT), `no readiness on the reclaimed port\n\n${session.tail(40)}`).toBe(true);
    } finally {
      await session.dispose();
      await freePort(DEFAULT_PORT);
      await holder.stop();
    }
  });
});
