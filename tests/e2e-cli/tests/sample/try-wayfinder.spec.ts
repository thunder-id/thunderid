// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * `try wayfinder` end to end: the CLI downloads the sample bundle, writes its declarative
 * resources into the install, restarts ThunderID, installs the sample's dependencies and starts
 * its services. Then a browser signs in to prove the whole chain actually produced a working app.
 *
 * This is the only spec that opens a browser, and the only one that reaches past the CLI into
 * what the CLI produced.
 *
 * It runs last and destructively: `try` rewrites <install>/config/resources/** and restarts the
 * server, so it must not run while the repl, restart or port-conflict specs are using the shared
 * install. The project dependency graph in playwright.config.ts is what orders it after them.
 */

import { test, expect } from "@playwright/test";
import { runCli } from "../../utils/cli-process";
import { useShared } from "../../fixtures/shared-install";
import { WayfinderPage, WAYFINDER_URL } from "../../pages/wayfinder.page";
import { SAMPLE_PORTS, stopSample } from "../../utils/sample-ports";
import { DEFAULT_PORT, isPortAccepting, waitForReady } from "../../utils/ports";
import { Timeouts } from "../../constants/timeouts";

/** Seeded in the sample bundle's thunderid-config.yaml, password equal to the username. */
const SEEDED_USER = "john.doe";

test.describe("thunderid try wayfinder", () => {
  test.beforeAll(async () => {
    // The CLI prompts before stopping anything holding the sample's fixed ports, and under a pty
    // that prompt is interactive with nobody to answer it. Clear them first.
    await stopSample();
  });

  test.afterAll(async () => {
    await stopSample();
  });

  test("launches the sample and a seeded user can sign in", async ({ page }) => {
    test.setTimeout(Timeouts.SAMPLE_TEST);

    // Deliberately not a pty session: `try` exits as soon as the services are up, and closing a
    // pty at that point SIGHUPs the ThunderID server it just backgrounded. See utils/cli-process.
    const result = await runCli(useShared(), ["try", "wayfinder"], Timeouts.SAMPLE);

    expect(result.exitCode, `try should succeed\n\n${result.output.slice(-2000)}`).toBe(0);
    expect(result.output).toContain("Wayfinder is running at");
    expect(result.output, "the summary should point at the sample's fixed origin").toContain(WAYFINDER_URL);

    // try restarts ThunderID, and the sample's login goes through it, so the gate has to be back
    // up before the browser starts.
    expect(await waitForReady(DEFAULT_PORT, Timeouts.SHORT), "the gate should be answering").toBe(true);

    // The API is a plain Express app with no readiness endpoint, so accepting a connection is the
    // most this can assert without guessing at a route.
    expect(await isPortAccepting(SAMPLE_PORTS.BACKEND), "the sample backend should be listening").toBe(true);

    const wayfinder = new WayfinderPage(page);
    await wayfinder.goto();
    await wayfinder.signIn(SEEDED_USER, SEEDED_USER);
    await wayfinder.expectSignedIn();
  });
});
