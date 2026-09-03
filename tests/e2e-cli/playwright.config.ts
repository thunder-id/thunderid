// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Playwright configuration for the ThunderID CLI e2e suite.
 *
 * This suite drives a terminal, so most projects define no browser. It also starts no web server:
 * the CLI is the thing that installs and starts ThunderID, and it has to find the default port
 * free to do it. That is why it cannot share tests/e2e's configuration, whose `webServer` boots a
 * backend on 8090 before the first test runs.
 *
 * @see https://playwright.dev/docs/test-configuration
 */

import { defineConfig, devices } from "@playwright/test";
import { Timeouts } from "./constants/timeouts";

export default defineConfig({
  testDir: "./tests",

  /**
   * Strictly serial. Every spec outside tests/commands binds the default port or deliberately
   * holds it, so two running at once would fight over it. This is a property of what is under
   * test, not a tuning knob.
   */
  fullyParallel: false,
  workers: 1,

  forbidOnly: !!process.env.CI,

  /**
   * No retries. A retry would re-run setup or re-download a distribution, and the failures this
   * suite catches (a hung prompt, a port left held) are exactly the ones a retry would mask.
   */
  retries: 0,

  reporter: [["list"], ["html", { open: "never" }], ["blob"], ["junit", { outputFile: "test-results/junit.xml" }]],

  /** Generous by default; the specs that install raise it further with test.setTimeout(). */
  timeout: Timeouts.STARTUP,

  expect: {
    timeout: Timeouts.SHORT,
  },

  globalSetup: require.resolve("./global-setup"),

  /**
   * A distribution is ~85MB extracted, so the suite installs one and reuses it. `install`
   * populates the shared workspace and everything downstream declares a dependency on it, which
   * is both the ordering mechanism and the reason those projects can warm-start in seconds.
   *
   * Only two projects download: `install`, and the first-run case in `port-conflict`, which needs
   * an install that setup has not yet pinned to a port.
   */
  projects: [
    {
      // No install and no network: these assert the argument surface that returns early.
      name: "commands",
      testMatch: "**/commands/*.spec.ts",
    },
    {
      name: "install",
      testMatch: "**/install/*.spec.ts",
    },
    {
      name: "repl",
      testMatch: "**/repl/*.spec.ts",
      dependencies: ["install"],
    },
    {
      name: "restart",
      testMatch: "**/restart/*.spec.ts",
      dependencies: ["install"],
    },
    {
      name: "port-conflict",
      testMatch: "**/port-conflict/*.spec.ts",
      dependencies: ["install"],
    },
    {
      /**
       * The only project with a browser, and the only destructive one: `try` rewrites the
       * install's declarative resources and restarts the server. It depends on every other
       * project that touches the shared install so it runs strictly after them.
       */
      name: "sample",
      testMatch: "**/sample/*.spec.ts",
      dependencies: ["install", "repl", "restart", "port-conflict"],
      use: {
        ...devices["Desktop Chrome"],
        // The gate serves TLS with a self-signed certificate, exactly as the CLI's own probes
        // assume.
        ignoreHTTPSErrors: true,
        // This spec is slow and depends on a lot of moving parts, so a failure has to carry
        // enough to diagnose it without reproducing the whole run.
        screenshot: "only-on-failure",
        trace: "retain-on-failure",
      },
    },
  ],
});
