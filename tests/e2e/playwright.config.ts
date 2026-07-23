/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

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

const STORAGE_STATE = path.join(__dirname, "playwright/.auth/console-admin.json");

/** Configure number of workers. Defaults to 1; CI raises this via PLAYWRIGHT_WORKERS. */
const parsedWorkers = parseInt(process.env.PLAYWRIGHT_WORKERS ?? "", 10);
const WORKERS = Number.isFinite(parsedWorkers) && parsedWorkers > 0 ? parsedWorkers : 1;

/**
 * Spec files that mutate global server state and therefore must never run
 * concurrently with any other instance of themselves or each other:
 * - cors-allowed-origins mutates the server-wide CORS allowed-origins config
 *   and asserts on live response headers.
 * - sample-app-mfa-login reconfigures the shared Sample App's auth flows,
 *   creates fixed-name flows/sender/user, and binds a fixed mock SMS port.
 * They run in the chained single-spec projects below; the browser projects
 * exclude them so the remaining specs can fan out across workers.
 */
const SERIAL_SPECS = [
  { tag: "cors", file: "**/cors-allowed-origins.spec.ts" },
  { tag: "mfa", file: "**/sample-app-mfa-login.spec.ts" },
];

const BROWSERS = [
  { name: "chromium", device: devices["Desktop Chrome"] },
  { name: "firefox", device: devices["Desktop Firefox"] },
  { name: "webkit", device: devices["Desktop Safari"] },
];

/**
 * One project per (browser, serial spec), each depending on the previous so at
 * most one serial spec is executing at any moment regardless of worker count.
 * The chain must span browsers: these specs conflict through shared server
 * state, so e.g. chromium and firefox may not run the CORS spec concurrently.
 * Notes on Playwright dependency semantics:
 * - If a project in the chain fails, the ones after it are skipped for that
 *   run; the job still fails.
 * - Selecting a later chain project (e.g. `--project="firefox*"`) also runs
 *   the earlier browsers' serial projects as dependencies. Intentional; the
 *   per-browser scripts in package.json accept those few extra tests.
 */
const serialProjects = (() => {
  const projects = [];
  let previous = "setup";
  for (const { name, device } of BROWSERS) {
    for (const { tag, file } of SERIAL_SPECS) {
      const projectName = `${name}-serial-${tag}`;
      projects.push({
        name: projectName,
        testMatch: file,
        use: { ...device, storageState: STORAGE_STATE },
        dependencies: [previous],
      });
      previous = projectName;
    }
  }
  return projects;
})();

export default defineConfig({
  /** Directory containing test files */
  testDir: "./tests",

  /**
   * Tests within a file always run in order on one worker; with multiple
   * workers, different spec files run concurrently. Specs that mutate global
   * state are isolated in the serial project chain (see SERIAL_SPECS).
   */
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

  projects: [
    /** Setup project - only runs auth.setup.ts */
    {
      name: "setup",
      testMatch: "**/*.setup.ts",
      use: { ...devices["Desktop Chrome"], ignoreHTTPSErrors: true },
    },

    /** Main test projects - run parallel-safe .spec.ts files with authenticated session */
    ...BROWSERS.map(({ name, device }) => ({
      name,
      testMatch: "**/*.spec.ts",
      testIgnore: SERIAL_SPECS.map(({ file }) => file),
      use: {
        ...device,
        storageState: STORAGE_STATE,
      },
      dependencies: ["setup"],
    })),

    /** Serial chain for specs that mutate global server state */
    ...serialProjects,
  ],
});
