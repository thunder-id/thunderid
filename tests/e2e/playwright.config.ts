// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Playwright E2E Test Configuration
 *
 * This configuration sets up test projects for Chromium, Firefox, and Webkit.
 * All projects depend on the `setup` project for authentication.
 *
 * Reports are generated in both HTML and Blob format (for merging).
 *
 * @see https://playwright.dev/docs/test-configuration
 */

import { defineConfig, devices } from "@playwright/test";
import dotenv from "dotenv";
import path from "path";
import { Timeouts } from "./constants/timeouts";

const envPath = path.resolve(__dirname, ".env");
dotenv.config({ path: envPath });

/**
 * Configure number of workers. Workers parallelize test *files* across all projects (including the
 * three browser projects), so one shared pool fans out chromium/firefox/webkit files simultaneously.
 * Default 6 fits the standard GitHub `ubuntu-latest` runner (4 vCPU / 16GB RAM); tune via
 * PLAYWRIGHT_WORKERS if the workflow ever moves to a larger runner or hits memory pressure.
 */
const WORKERS = process.env.PLAYWRIGHT_WORKERS ? parseInt(process.env.PLAYWRIGHT_WORKERS, 10) : 6;

const BROWSERS = [
  { id: "chromium", device: devices["Desktop Chrome"] },
  { id: "firefox", device: devices["Desktop Firefox"] },
  { id: "webkit", device: devices["Desktop Safari"] },
];

/**
 * Specs that mutate global, non-partitionable server state: the CORS allowed-origins list and
 * the shared sample app's flow bindings and server-wide notification-sender setting. Running
 * them in more than one browser project at a time is racy, so they run on chromium only.
 *
 * They assert server behavior (flow COMPLETE, Access-Control-Allow-Origin, a scope update
 * surviving a reload), so one run is the whole coverage; the shared form and unsaved-changes UI is
 * exercised cross-browser by the applications and accessibility specs.
 */
const SERVER_STATE_SPECS = [
  "**/settings/cors-allowed-origins.spec.ts",
  "**/sample-app-authentication/sample-app-mfa-login.spec.ts",
];

/**
 * wayfinder-sample-setup.spec.ts imports the Wayfinder config bundle (replacing the server-wide
 * default resource server and CORS allowed-origins list, same non-partitionable-state reasoning
 * as SERVER_STATE_SPECS above); tests/wayfinder/** exercises the standalone Wayfinder sample app
 * against data that import creates (the seed user, the "Wayfinder" OAuth client). Both run
 * chromium-only via their own dedicated projects below rather than SERVER_STATE_SPECS, because
 * the tryout specs additionally need to run strictly *after* the setup spec finishes - a plain
 * testIgnore exclusion has no ordering mechanism, only a project `dependencies:` graph does.
 */
const WAYFINDER_SETUP_SPEC = "**/welcome/wayfinder-sample-setup.spec.ts";
const WAYFINDER_TRYOUT_SPECS = ["**/wayfinder/*.spec.ts"];

export default defineConfig({
  /** Directory containing test files */
  testDir: "./tests",

  /** Run tests sequentially to avoid auth conflicts */
  fullyParallel: false,

  /** Fail CI builds if test.only() is accidentally committed */
  forbidOnly: !!process.env.CI,

  /** Retry failed tests (more on CI) */
  retries: process.env.CI ? 2 : 1,

  /** Number of workers for parallel execution */
  workers: WORKERS,

  /** Generate HTML report, Console list, and Blob report for merging */
  reporter: [
    ["html"],
    ["list"],
    ["blob"],
    // Add JSON reporter for better CI integration
    ["json", { outputFile: "test-results/test-results.json" }],
    // Add JUnit reporter for CI systems
    ["junit", { outputFile: "test-results/junit.xml" }],
  ],

  /** Global test timeout */
  timeout: 90000,

  /** Expect timeout for assertions */
  expect: {
    timeout: 10000,
    toHaveScreenshot: {
      maxDiffPixels: 100,
      threshold: 0.2,
    },
  },

  /** Global setup and teardown */
  globalSetup: require.resolve("./global-setup"),
  globalTeardown: require.resolve("./global-teardown"),

  /**
   * Run local dev server before starting the tests.
   * This ensures the server is up before the setup project tries to authenticate.
   */
  webServer: {
    command:
      process.platform === "win32"
        ? "cd ..\\..  && pwsh -File .\\build.ps1 run_backend"
        : "cd ../.. && ./build.sh run_backend",
    url: "https://localhost:8090/health/liveness",
    reuseExistingServer: true,
    ignoreHTTPSErrors: true,
    timeout: 120 * 1000,
  },

  /** Shared settings for all projects */
  use: {
    trace: "retain-on-failure",
    ignoreHTTPSErrors: true,
    screenshot: "only-on-failure",
    video: "retain-on-failure",
    actionTimeout: Timeouts.DEFAULT_ACTION,
    baseURL: process.env.BASE_URL || "https://localhost:8090",
    // Add context options for better reliability
    viewport: { width: 1280, height: 720 },
    userAgent: "Playwright E2E Tests",
    // Collect console logs for debugging
    launchOptions: {
      slowMo: process.env.SLOW_MO ? parseInt(process.env.SLOW_MO) : 0,
    },
  },

  /**
   * Browser projects deliberately do NOT set `storageState`. The saved state contains localStorage
   * entries, and restoring those makes `browser.newContext()` navigate to the origin internally
   * (waiting for `load`) before a test starts, which is charged to the test timeout and is slow
   * against the console's large bundles. It is also redundant: the `authenticatedPage` fixture
   * restores cookies and injects both localStorage and sessionStorage itself. Tests that need an
   * authenticated session take `authenticatedPage`; tests that need a signed-out one take `page`.
   */
  projects: [
    /** Setup project - only runs auth.setup.ts */
    {
      name: "setup",
      testMatch: "**/*.setup.ts",
      use: { ...devices["Desktop Chrome"], ignoreHTTPSErrors: true },
    },

    /**
     * Every spec runs in one flat fan-out; the server-state specs (see SERVER_STATE_SPECS) are
     * excluded from the non-chromium projects instead of being duplicated and serialized. The
     * Wayfinder setup/tryout specs are excluded from every browser project unconditionally - they
     * run exclusively through the two dedicated projects below.
     */
    ...BROWSERS.map(browser => ({
      name: browser.id,
      testMatch: "**/*.spec.ts",
      testIgnore: [
        WAYFINDER_SETUP_SPEC,
        ...WAYFINDER_TRYOUT_SPECS,
        ...(browser.id === "chromium" ? [] : SERVER_STATE_SPECS),
      ],
      use: { ...browser.device },
      dependencies: ["setup"],
    })),

    /**
     * tests/wayfinder/** reads data (the seed user, the "Wayfinder" OAuth client) that only
     * exists once wayfinder-sample-setup.spec.ts's import has completed, so wayfinder-tryout must
     * not start until wayfinder-setup finishes
     */
    {
      name: "wayfinder-setup",
      testMatch: WAYFINDER_SETUP_SPEC,
      use: { ...devices["Desktop Chrome"] },
      dependencies: ["setup"],
    },
    {
      name: "wayfinder-tryout",
      testMatch: WAYFINDER_TRYOUT_SPECS,
      use: { ...devices["Desktop Chrome"] },
      dependencies: ["wayfinder-setup"],
    },
  ],
});
