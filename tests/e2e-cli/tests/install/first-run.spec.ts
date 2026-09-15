// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * The first run: fetch the manifest, download and extract a release, run setup.sh, start the
 * server.
 *
 * This is the only spec that downloads a distribution. It populates the shared workspace that the
 * repl, restart and sample projects then reuse, which is why they declare a dependency on this
 * project rather than installing their own copy.
 */

import { test, expect } from "@playwright/test";
import fs from "fs";
import path from "path";
import { CliSession } from "../../utils/cli-session";
import { createShared } from "../../fixtures/shared-install";
import { DEFAULT_PORT, freePort, requirePortFree, waitForReady } from "../../utils/ports";
import { Timeouts } from "../../constants/timeouts";

test.describe("first run", () => {
  test("installs, sets up and starts ThunderID", async () => {
    test.setTimeout(Timeouts.INSTALL_TEST);

    await requirePortFree(DEFAULT_PORT);
    const workspace = createShared();
    const session = new CliSession(workspace, [], { preconfigured: true });

    try {
      await session.expect("Latest ThunderID release: v", Timeouts.INSTALL);
      await session.expect("installed to", Timeouts.INSTALL);
      await session.expect("Setup complete", Timeouts.INSTALL);
      await session.expect("ThunderID started on port", Timeouts.STARTUP);

      const port = session.startedPort();
      expect(port, "a first run with a free default port should bind it").toBe(DEFAULT_PORT);
      expect(await waitForReady(port), `server should answer readiness\n\n${session.tail(40)}`).toBe(true);

      const state = workspace.state();
      expect(state.active, "state.json should record an active version").toBeTruthy();
      const version = state.active as string;

      const recorded = state.versions?.[version];
      expect(recorded?.setupComplete).toBe(true);
      expect(recorded?.installPath, "an install path should be recorded").toBeTruthy();
      expect(path.isAbsolute(recorded!.installPath!), "the install path should be absolute").toBe(true);

      // The recorded path has to be the one on disk, since later runs start what it points at
      // rather than re-deriving the location.
      for (const name of ["setup.sh", "start.sh", "deployment.yaml"]) {
        expect(fs.existsSync(path.join(recorded!.installPath!, name)), `${name} should be in the install`).toBe(true);
      }

      // Leave the port free for the projects that depend on this one. The install itself stays on
      // disk: that is the artifact they reuse.
      await session.shutdown(port);
    } finally {
      await session.dispose();
      await freePort(DEFAULT_PORT);
    }
  });
});
