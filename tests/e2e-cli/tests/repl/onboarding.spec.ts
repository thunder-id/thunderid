// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/** The first-run use-case picker the REPL opens on, and the command overlay it offers. */

import { test, expect } from "../../fixtures/cli";

test.describe("REPL onboarding", () => {
  // Both halves share one session on purpose: each spec in this project starts and stops a real
  // server, so splitting assertions that read the same screen would double that cost for nothing.
  //
  // The picker is gated on onboardingDone, which config only records when a use case is actually
  // selected (see ui.selectOnboarding). No spec here selects one, so it is still showing.
  test("opens on the use-case picker and offers the commands", async ({ repl }) => {
    await repl.session.expect("What would you like to try?");
    await repl.session.expect("Secured Web Application");
    await repl.session.expect("Secured AI Agent");

    await repl.session.openCommandOverlay();

    expect(repl.session.output).toContain("/try-consumer");
    expect(repl.session.output).toContain("/integrate-react");
    expect(repl.session.output).toContain("Follow recent server logs");
  });
});
