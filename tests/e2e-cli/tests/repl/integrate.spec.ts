// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * /integrate-<platform> fetches that platform's guide from the docs site and renders it in the
 * REPL. This reaches thunderid.dev, the same host the release manifest comes from.
 */

import { test, expect } from "../../fixtures/cli";
import { Timeouts } from "../../constants/timeouts";

test.describe("REPL /integrate", () => {
  test("loads the React guide", async ({ repl }) => {
    await repl.session.runSlash("/integrate-react");

    await repl.session.expect("React", Timeouts.REMOTE);
    // The command reports a fetch failure inline rather than erroring out, so a broken docs URL
    // would otherwise render as a passing test.
    expect(repl.session.output, "the guide should have loaded").not.toContain("Could not load");
  });
});
