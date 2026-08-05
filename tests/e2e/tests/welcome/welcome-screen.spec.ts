// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Welcome Screen E2E Tests
 *
 * Covers the console welcome screen's first-start vs subsequent-start behavior. The screen is
 * shown once per browser session (state lives in sessionStorage, not the server), so these tests
 * manipulate that sessionStorage flag directly rather than relying on a fresh server/database.
 *
 * Required environment variables:
 * - BASE_URL: Console base URL
 * - ADMIN_USERNAME: Admin credentials for authentication
 * - ADMIN_PASSWORD: Admin password for authentication
 */

import { test, expect, routes } from "../../fixtures/console";
import { TestTags } from "../../constants/test-tags";
import { Timeouts } from "../../constants/timeouts";

test.describe("Welcome Screen", { tag: [TestTags.WAYFINDER, TestTags.SMOKE] }, () => {
  /** TC001: First start redirects to the welcome screen */
  test("TC001: First start redirects to the welcome screen", async ({ welcomePage }) => {
    await test.step("Simulate a first application start (clear the dismissed flag)", async () => {
      await welcomePage.simulateFirstStart();
    });

    await test.step("Navigate to the console home", async () => {
      await welcomePage.page.goto(`${welcomePage.baseUrl}${routes.home}`, { timeout: Timeouts.PAGE_LOAD });
    });

    await test.step("Verify redirected to the welcome screen with the tryout sections visible", async () => {
      await welcomePage.verifyOnWelcomeScreen();
      await expect(welcomePage.tryoutSecuringApplicationRow).toBeVisible();
      await expect(welcomePage.tryoutAiAgentsRow).toBeVisible();
      await expect(welcomePage.tryoutMcpRow).toBeVisible();
      await welcomePage.screenshot("tc001-welcome-first-start");
    });

    await test.step("Verify the dismissed flag is now set", async () => {
      expect(await welcomePage.isWelcomeDismissed()).toBe(true);
    });
  });

  /** TC002: Subsequent start does not redirect to the welcome screen */
  test("TC002: Subsequent start does not redirect to the welcome screen", async ({ welcomePage }) => {
    await test.step("Navigate to the console home", async () => {
      await welcomePage.page.goto(`${welcomePage.baseUrl}${routes.home}`, { timeout: Timeouts.PAGE_LOAD });
    });

    await test.step("Verify the console rendered without redirecting to the welcome screen", async () => {
      // Assert the console actually mounted (user-menu trigger present), not just that the hero is
      // absent — otherwise a blank page would satisfy the negative check trivially.
      await expect(welcomePage.userMenuTrigger).toBeVisible();
      expect(welcomePage.page.url()).not.toContain(routes.welcome);
      await expect(welcomePage.heroHeading).not.toBeVisible();
      await welcomePage.screenshot("tc002-no-redirect-subsequent-start");
    });
  });

  /** TC003: Welcome screen is reopenable from the user menu */
  test("TC003: Welcome screen is reopenable from the user menu", async ({ welcomePage }) => {
    await test.step("Navigate to the console home", async () => {
      await welcomePage.page.goto(`${welcomePage.baseUrl}${routes.home}`, { timeout: Timeouts.PAGE_LOAD });
    });

    await test.step("Open the user menu and click Welcome", async () => {
      await welcomePage.reopenFromUserMenu();
    });

    await test.step("Verify the welcome screen is shown", async () => {
      await welcomePage.verifyOnWelcomeScreen();
      await welcomePage.screenshot("tc003-welcome-reopened-from-menu");
    });
  });
});
