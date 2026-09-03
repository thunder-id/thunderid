// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Fixtures for specs that run against the shared install.
 *
 * Each spec warm-starts its own CLI session rather than sharing one live process. A warm start
 * costs a few seconds because the install already exists, and it buys real isolation: a spec that
 * wedges the REPL cannot take the rest of the file down with it.
 */

import { test as base } from "@playwright/test";
import { CliSession } from "../utils/cli-session";
import { BrowserStub } from "../utils/browser-stub";
import { useShared } from "./shared-install";
import { freePort, waitForReady } from "../utils/ports";
import { Timeouts } from "../constants/timeouts";

export interface ReplFixture {
  /** A session that has reached the REPL, with the server up. */
  session: CliSession;
  /** The port that session's server is listening on. */
  port: number;
}

interface CliFixtures {
  repl: ReplFixture;
  browserStub: BrowserStub;
}

export const test = base.extend<CliFixtures>({
  browserStub: async ({}, use) => {
    const stub = BrowserStub.create();
    await use(stub);
    stub.remove();
  },

  repl: async ({}, use) => {
    const workspace = useShared();
    const session = new CliSession(workspace, [], { preconfigured: true });

    await session.expect("ThunderID started on port", Timeouts.STARTUP);
    const port = session.startedPort();
    if (!(await waitForReady(port, Timeouts.SHORT))) {
      throw new Error(`Server did not become ready on port ${port}`);
    }

    await use({ session, port });

    // The CLI backgrounds the server, so disposing the session is not enough to release the port
    // for the next spec.
    await session.dispose();
    await freePort(port);
  },
});

export { expect } from "@playwright/test";
