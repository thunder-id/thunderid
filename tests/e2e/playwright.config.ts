// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Playwright E2E Test Configuration
 *
 * This configuration sets up test projects for Chromium, Firefox, and Webkit.
 * There is no dedicated login project: the `authenticatedPage` fixture lazily logs in on a
 * test's first use and caches the session per worker (see console-admin-auth-utils.ts) so
 * concurrent workers never share a single-use refresh token. globalSetup (below) clears any such
 * cached files left on disk by a previous run against a different server instance, so every
 * project - regardless of `--project`/`--grep` filtering - always logs in fresh against this run.
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
 * Specs that mutate global, non-partitionable server state: the shared sample app's flow bindings,
 * the server-wide notification-sender and identity_provider.<vendor>_base_url settings, and each
 * mock server's fixed port. Running them in more than one browser project at a time is not just
 * racy, it cannot work - the mock ports the backend is configured to call collide outright.
 *
 * They assert server behavior (flow COMPLETE, /user/emails called), so one run is the whole
 * coverage. They run on chromium only.
 */
const SERVER_STATE_SPECS = [
  "**/sample-app-authentication/sample-app-mfa-login.spec.ts",
  "**/sample-app-authentication/sample-app-social-login.spec.ts",
];

/**
 * The accessibility audits scan the Admin Console's pages, which are the same in every browser, so
 * one browser's run is the whole coverage. They run on chromium only.
 */
const ACCESSIBILITY_SPECS = ["**/accessibility/**/*.spec.ts"];

/**
 * wayfinder-bundle.spec.ts verifies the Wayfinder config bundle that run-e2e.sh imported (the
 * bundle replaces the server-wide default resource server and CORS allowed-origins list, so it
 * only ever runs against the dedicated Wayfinder server); tests/wayfinder/** exercises the
 * standalone Wayfinder sample app against data that import creates (the seed user, the "Wayfinder"
 * OAuth client). Both run chromium-only via their own dedicated projects below rather than
 * SERVER_STATE_SPECS, because the tryout specs additionally need to run strictly *after* the bundle
 * check finishes - a plain testIgnore exclusion has no ordering mechanism, only a project
 * `dependencies:` graph does.
 */
const WAYFINDER_SETUP_SPEC = "**/wayfinder-setup/wayfinder-bundle.spec.ts";
// `**` after wayfinder/ matches both direct children (mock-email-flows.spec.ts) and specs grouped
// into a subdirectory (b2c-tryout/authentication.spec.ts).
const WAYFINDER_TRYOUT_SPECS = ["**/wayfinder/**/*.spec.ts"];

/**
 * mock-email-flows.spec.ts reads from the shared mock SMTP inbox (see that file's header), which
 * has no per-recipient filtering - two of its tests running at once, even in different browsers,
 * can grab each other's email. It is excluded from WAYFINDER_TRYOUT_SPECS's fully-parallel,
 * three-browser projects below and instead runs `.serial` in its own single chromium-only project.
 */
const MOCK_EMAIL_TRYOUT_SPEC = "**/wayfinder/mock-email-flows.spec.ts";

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
   * This ensures the server is up before the first test's fixture tries to authenticate.
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
    /**
     * Every spec runs in one flat fan-out; the server-state and accessibility specs (see
     * SERVER_STATE_SPECS and ACCESSIBILITY_SPECS) are excluded from the non-chromium projects
     * instead of being duplicated and serialized. The
     * Wayfinder setup/tryout specs are excluded from every browser project unconditionally - they
     * run exclusively through the two dedicated projects below.
     */
    ...BROWSERS.map(browser => ({
      name: browser.id,
      testMatch: "**/*.spec.ts",
      testIgnore: [
        WAYFINDER_SETUP_SPEC,
        ...WAYFINDER_TRYOUT_SPECS,
        ...(browser.id === "chromium" ? [] : [...SERVER_STATE_SPECS, ...ACCESSIBILITY_SPECS]),
      ],
      use: { ...browser.device },
    })),

    /**
     * tests/wayfinder/** reads data (the seed user, the "Wayfinder" OAuth client) that only
     * exists once wayfinder-sample-setup.spec.ts's import has completed, so wayfinder-tryout must
     * not start until the bundle check finishes.
     */
    {
      name: "wayfinder-setup",
      testMatch: WAYFINDER_SETUP_SPEC,
      use: { ...devices["Desktop Chrome"] },
    },
    ...BROWSERS.map(browser => ({
      name: `${browser.id}-wayfinder-tryout`,
      testMatch: WAYFINDER_TRYOUT_SPECS,
      testIgnore: [MOCK_EMAIL_TRYOUT_SPEC],
      // Run the tryout specs in parallel across browsers, but only after wayfinder-setup has completed
      fullyParallel: true,
      use: { ...browser.device },
      dependencies: ["wayfinder-setup"],
    })),
    {
      name: "wayfinder-mock-email",
      testMatch: MOCK_EMAIL_TRYOUT_SPEC,
      use: { ...devices["Desktop Chrome"] },
      dependencies: ["wayfinder-setup"],
    },
  ],
});
