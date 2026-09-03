// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * A first run finding the default port taken can be moved, because setup has not yet seeded the
 * console's redirect URIs with a port.
 *
 * This is the one spec outside install/ that downloads a distribution: being movable is a
 * property of a *first* run, so it cannot reuse the shared install, which is already configured.
 */

import { test, expect } from "@playwright/test";
import { CliSession, Keys } from "../../utils/cli-session";
import { Workspace } from "../../utils/workspace";
import { DEFAULT_PORT, PortHolder, releasePortIfBound, requirePortFree, waitForReady } from "../../utils/ports";
import { Timeouts } from "../../constants/timeouts";

test.describe("port conflict on a first run", () => {
  test("moves to an alternate port and leaves the holder alone", async () => {
    test.setTimeout(Timeouts.INSTALL_TEST);

    await requirePortFree(DEFAULT_PORT);
    const workspace = Workspace.create();
    const holder = await PortHolder.start(DEFAULT_PORT);
    const session = new CliSession(workspace, [], { preconfigured: true });
    let port = 0;

    try {
      await session.expect(`Port ${DEFAULT_PORT} is already in use`, Timeouts.INSTALL);
      await session.expect("How would you like to proceed?");

      // The alternate is the second option, after "kill the process".
      await session.expect("instead");
      await session.send(Keys.DOWN, Keys.ENTER);

      await session.expect("Setup complete", Timeouts.INSTALL);
      await session.expect("ThunderID started on port", Timeouts.STARTUP);

      port = session.startedPort();
      expect(port, "choosing the alternate must not bind the contested port").not.toBe(DEFAULT_PORT);
      expect(holder.alive, "choosing the alternate must leave the holder running").toBe(true);
      expect(await waitForReady(port), `no readiness on the alternate port\n\n${session.tail(40)}`).toBe(true);
    } finally {
      await session.dispose();
      await releasePortIfBound(port);
      await holder.stop();
      workspace.remove();
    }
  });
});
