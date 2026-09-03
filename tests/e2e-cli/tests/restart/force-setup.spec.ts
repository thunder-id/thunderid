// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/** --setup re-runs setup against an install that already exists. */

import { test, expect } from "@playwright/test";
import { CliSession } from "../../utils/cli-session";
import { useShared } from "../../fixtures/shared-install";
import { DEFAULT_PORT, freePort } from "../../utils/ports";
import { Timeouts } from "../../constants/timeouts";

test.describe("thunderid --setup", () => {
  test("re-runs setup without re-downloading", async () => {
    test.setTimeout(Timeouts.INSTALL_TEST);

    const session = new CliSession(useShared(), ["--setup"], { preconfigured: true });

    try {
      await session.expect("Setup requested", Timeouts.STARTUP);
      await session.expect("Setup complete", Timeouts.INSTALL);
      await session.expect("ThunderID started on port", Timeouts.STARTUP);

      expect(await session.waitFor("installed to", 0), "--setup must reuse the install").toBe(false);
    } finally {
      await session.dispose();
      await freePort(DEFAULT_PORT);
    }
  });
});
