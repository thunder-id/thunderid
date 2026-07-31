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

/**
 * Specs that mutate global, non-partitionable server state and cannot run in parallel with themselves
 * across browsers:
 *   - CORS allowed-origins is a server-wide list; the console does read-modify-write on Save, so
 *     concurrent workers overwrite each other's list.
 *   - MFA suite reconfigures the shared sample-app's auth flow and depends on the one notification
 *     sender pointing at the one mock SMS server; parallel workers step on each other's OTPs.
 * These files are excluded from the fan-out projects below and re-added as a serial chain
 * (chromium -> firefox -> webkit) so at most one worker is executing them at any moment. The chain
 * also runs after the fan-out, not alongside it: the state these specs reshape is state other specs
 * read. The MFA suite repoints the shared sample-app application at its own auth flow and reverts it
 * on teardown, so a sample-app sign-in that starts inside that window resolves a flow that is being
 * swapped out and gets a 500 from /flow/execute.
 */
const SERIAL_SPECS = [
  "**/settings/cors-allowed-origins.spec.ts",
  "**/sample-app-authentication/sample-app-mfa-login.spec.ts",
];

/**
 * Specs that have possible collisions in the system, and so must not run while 
 * anything else is running. They are run in a separate project after all the 
 * other specs have finished.
 */
const ORDERED_LAST_SPECS = ["**/applications/application-user-access.spec.ts"];

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

    /** Main browser projects - run parallel-safe specs. Serial specs and ORDERED_LAST_SPECS are
     *  excluded here and re-added below in a per-browser chain / a tail project respectively. */
    {
      name: "chromium",
      testMatch: "**/*.spec.ts",
      testIgnore: [...SERIAL_SPECS, ...ORDERED_LAST_SPECS],
      use: {
        ...devices["Desktop Chrome"],
      },
      dependencies: ["setup"],
    },

    {
      name: "firefox",
      testMatch: "**/*.spec.ts",
      testIgnore: [...SERIAL_SPECS, ...ORDERED_LAST_SPECS],
      use: {
        ...devices["Desktop Firefox"],
      },
      dependencies: ["setup"],
    },

    {
      name: "webkit",
      testMatch: "**/*.spec.ts",
      testIgnore: [...SERIAL_SPECS, ...ORDERED_LAST_SPECS],
      use: {
        ...devices["Desktop Safari"],
      },
      dependencies: ["setup"],
    },

    /**
     * Serial chain for CORS + MFA specs. Each browser depends on the previous one so Playwright's
     * scheduler runs them one at a time even when 6 workers are otherwise fanning out. The head of
     * the chain waits for all three fan-out projects, keeping the shared sample-app reconfiguration
     * out of every other spec's way. If a project in the chain fails, later browsers in the chain
     * are skipped (dependency semantics); the job still fails and per-test retries still apply.
     */
    {
      name: "serial-chromium",
      testMatch: SERIAL_SPECS,
      use: {
        ...devices["Desktop Chrome"],
      },
      dependencies: ["chromium", "firefox", "webkit"],
    },
    {
      name: "serial-firefox",
      testMatch: SERIAL_SPECS,
      use: {
        ...devices["Desktop Firefox"],
      },
      dependencies: ["serial-chromium"],
    },
    {
      name: "serial-webkit",
      testMatch: SERIAL_SPECS,
      use: {
        ...devices["Desktop Safari"],
      },
      dependencies: ["serial-firefox"],
    },

    /**
     * Runs ORDERED_LAST_SPECS only once every fan-out browser project has finished, so nothing
     * else can still be mutating the user-type set they read. Depends directly on chromium/
     * firefox/webkit rather than the serial chain above: SERIAL_SPECS (CORS, MFA) never touch
     * user types, so there's nothing to wait on there.
     */
    {
      name: "chromium-user-access",
      testMatch: ORDERED_LAST_SPECS,
      use: {
        ...devices["Desktop Chrome"],
      },
      dependencies: ["chromium", "firefox", "webkit"],
    },
    {
      name: "firefox-user-access",
      testMatch: ORDERED_LAST_SPECS,
      use: {
        ...devices["Desktop Firefox"],
      },
      dependencies: ["chromium", "firefox", "webkit"],
    },
    {
      name: "webkit-user-access",
      testMatch: ORDERED_LAST_SPECS,
      use: {
        ...devices["Desktop Safari"],
      },
      dependencies: ["chromium", "firefox", "webkit"],
    },
  ],
});
