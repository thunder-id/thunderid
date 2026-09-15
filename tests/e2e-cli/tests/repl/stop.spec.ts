// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/** /stop takes the server down and ends the session. */

import { test, expect } from "../../fixtures/cli";
import { isPortInUse } from "../../utils/ports";
import { Timeouts } from "../../constants/timeouts";

test.describe("REPL /stop", () => {
  test("shuts the server down and exits", async ({ repl }) => {
    await repl.session.runSlash("/stop");
    await repl.session.expect("ThunderID stopped.", Timeouts.STARTUP);

    expect(await isPortInUse(repl.port), `port ${repl.port} should be released by /stop`).toBe(false);
    expect(await repl.session.exitRepl()).toBe(0);
  });
});
