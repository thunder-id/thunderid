// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * /open-console hands the console URL to the system browser.
 *
 * utils.OpenBrowser shells out to `open` or `xdg-open` with no suppression hook, so the session
 * runs with a recording stub earlier on PATH. Nothing opens, and the URL the CLI chose becomes
 * assertable, which is the part of the command worth testing.
 */

import { test, expect } from "@playwright/test";
import { CliSession } from "../../utils/cli-session";
import { BrowserStub } from "../../utils/browser-stub";
import { useShared } from "../../fixtures/shared-install";
import { releasePortIfBound, waitForReady } from "../../utils/ports";
import { Timeouts } from "../../constants/timeouts";

test.describe("REPL /open-console", () => {
  test("opens the console URL for the port in use", async () => {
    const stub = BrowserStub.create();
    const session = new CliSession(useShared(), [], { preconfigured: true, pathPrefix: stub.binDir });
    let port = 0;

    try {
      await session.expect("ThunderID started on port", Timeouts.STARTUP);
      port = session.startedPort();
      if (!(await waitForReady(port, Timeouts.SHORT))) {
        throw new Error(`Server did not become ready on port ${port}`);
      }

      await session.runSlash("/open-console");
      await session.expect("Opening");

      // OpenBrowser uses cmd.Start() rather than Run, so the CLI does not wait for the opener
      // and neither can the assertion.
      await expect
        .poll(() => stub.openedUrls(), { timeout: Timeouts.SHORT })
        .toEqual([expect.stringContaining(`:${port}/console`)]);
    } finally {
      await session.dispose();
      await releasePortIfBound(port);
      stub.remove();
    }
  });
});
