// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/** /logs tails the server log inside the REPL until Esc. */

import { test } from "../../fixtures/cli";
import { Keys } from "../../utils/cli-session";

test.describe("REPL /logs", () => {
  test("follows the log and returns to the prompt on Esc", async ({ repl }) => {
    await repl.session.runSlash("/logs");
    await repl.session.expect("Following logs");

    repl.session.reset();
    await repl.session.send(Keys.ESC);

    // Leaving the follower restores the ordinary prompt rather than exiting the REPL.
    await repl.session.expect("Type / for commands");
  });
});
