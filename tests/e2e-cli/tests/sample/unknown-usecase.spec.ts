// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/** `try` validates the sample name against knownSamples once an install exists. */

import { test, expect } from "@playwright/test";
import { CliSession } from "../../utils/cli-session";
import { useShared } from "../../fixtures/shared-install";
import { Timeouts } from "../../constants/timeouts";

test.describe("thunderid try, unknown usecase", () => {
  // "consumer" is the REPL slash command (/try-consumer), not a sample name, so it is the
  // mistake a user is most likely to make on the command line. The error has to name what is
  // actually available rather than just refusing.
  test("names the available samples", async () => {
    const session = new CliSession(useShared(), ["try", "consumer"], { preconfigured: true });

    try {
      await session.expect(`unknown sample "consumer"`, Timeouts.SHORT);
      await session.expect("available: wayfinder");
      expect(await session.waitForExit()).toBe(1);
    } finally {
      await session.dispose();
    }
  });
});
