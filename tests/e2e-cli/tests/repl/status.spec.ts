// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/** /status reports whether the server this session started is answering. */

import { test, expect } from "../../fixtures/cli";

test.describe("REPL /status", () => {
  test("reports the running server and its console URL", async ({ repl }) => {
    await repl.session.runSlash("/status");

    await repl.session.expect("ThunderID is running at");
    await repl.session.expect("Console:");
    // The URL has to carry the port this session actually bound, since that is what a user would
    // click, and a stale port would send them to nothing.
    expect(repl.session.output).toContain(`:${repl.port}/console`);
  });
});
