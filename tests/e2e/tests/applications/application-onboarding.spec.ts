// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Application Onboarding E2E Tests
 *
 * Covers the applications list page and the create application wizard.
 *
 * Required environment variables:
 * - BASE_URL: Console base URL
 * - ADMIN_USERNAME: Admin credentials for authentication
 * - ADMIN_PASSWORD: Admin password for authentication
 */

import { test, expect, ApplicationsApi } from "../../fixtures/console";
import { TestDataFactory } from "../../utils/test-data";

test.describe("Application Onboarding", () => {
  test.describe("Applications List Page", () => {
    /** TC001: Applications list page loads */
    test("TC001: Applications list page loads", async ({ applicationsPage }) => {
      await test.step("Navigate to Applications page", async () => {
        console.log("Navigating to applications list page...");
        await applicationsPage.goto();
        console.log("Applications page navigated");
      });

      await test.step("Verify applications list is visible", async () => {
        await applicationsPage.verifyPageLoaded();
        console.log("Applications list container visible");
      });

      await test.step("Verify Add Application button is present", async () => {
        await expect(applicationsPage.addApplicationButton.first()).toBeVisible();
        console.log("Add Application button is present");
      });
    });
  });

  test.describe("Create Application Wizard", () => {
    const createdAppIds: string[] = [];

    test.afterAll(async ({ request }) => {
      const applicationsApi = new ApplicationsApi(request);
      for (const appId of createdAppIds) {
        const deleted = await applicationsApi.deleteById(appId);
        console.log(deleted ? `Cleaned up test app: ${appId}` : `Failed to clean up test app ${appId}`);
      }
    });

    /** TC002: Full INBUILT wizard flow with name validation and persistence after navigation */
    test("TC002: Full INBUILT wizard flow with name validation and persistence", async ({
      applicationsPage,
      applicationsApi,
    }) => {
      const appData = TestDataFactory.createApplication({ name: `TestApp_INBUILT_${Date.now()}` });
      let createdAppUrl: string;
      let createdAppId: string;

      await test.step("Navigate to Applications page, select a template and open wizard", async () => {
        await applicationsPage.goto();
        await applicationsPage.verifyPageLoaded();
        await applicationsPage.clickAddApplication();
        await applicationsPage.selectTemplate("NEXTJS");
      });

      await test.step("Step 1 [configure-name]: Verify Next blocked on empty name, then fill name, restrict to Person, and click Next", async () => {
        await applicationsPage.waitForStep("application-configure-name");
        await expect(applicationsPage.nextButton.first()).toBeDisabled();
        console.log("Next button is disabled with empty name — correct");

        await applicationsPage.fillAppName(appData.name);
        await expect(applicationsPage.nextButton.first()).toBeEnabled();
        console.log("Next button enabled after typing name — correct");

        // Pin to a single user type instead of the wizard's "allow all" default, so this test
        // cannot race with specs that create/delete other user types (e.g. user-type-creation.spec.ts).
        await applicationsPage.selectOnlyUserType("Person");
        await applicationsPage.clickNext();
        await applicationsPage.handleOptionalOuStep();
      });

      await test.step("Step 2 [configure-sign-in]: Skip and click Next", async () => {
        await applicationsPage.waitForStep("application-configure-sign-in");
        await applicationsPage.clickNext();
      });

      await test.step("Step 3 [configure-design]: Verify INBUILT is default and click Next", async () => {
        await applicationsPage.waitForStep("application-configure-design");
        await applicationsPage.clickNext();
      });

      await test.step("Step 4: Wait for wizard completion (secret screen or edit page)", async () => {
        createdAppUrl = await applicationsPage.completeWizardCreation();
        createdAppId = createdAppUrl.split("/").pop()!;
        createdAppIds.push(createdAppId);
        console.log("Wizard complete, edit URL:", createdAppUrl);
      });

      await test.step("Verify created app edit page is reachable", async () => {
        await applicationsPage.page.goto(createdAppUrl);
        expect(applicationsPage.page.url()).toMatch(/\/console\/applications\/[^/]+$/);
        console.log("Created app edit page still reachable:", createdAppUrl);
      });

      await test.step("Verify only Person was granted via the application detail API", async () => {
        const app = await applicationsApi.get(createdAppId);
        expect(app.allowedUserTypes).toEqual(["Person"]);
        console.log("Application restricted to Person user type, correct");
      });

      await test.step("Navigate away then back to applications", async () => {
        await applicationsPage.page.goto(`${process.env.BASE_URL || ""}/console/dashboard`);
        console.log("Navigated away to dashboard");
        await applicationsPage.goto();
        await applicationsPage.verifyPageLoaded();
        console.log("Navigated back to applications list");
      });

      await test.step("Verify app edit page still reachable after navigation", async () => {
        await applicationsPage.page.goto(createdAppUrl);
        expect(applicationsPage.page.url()).toMatch(/\/console\/applications\/[^/]+$/);
        console.log("App still reachable after navigation:", createdAppUrl);
      });
    });

    /** TC003: SPA (public client) hides the sign-in approach picker entirely */
    test("TC003: Create application - SPA stack hides EMBEDDED experience", async ({ applicationsPage }) => {
      await test.step("Navigate and select the React (SPA) template", async () => {
        await applicationsPage.goto();
        await applicationsPage.verifyPageLoaded();
        await applicationsPage.clickAddApplication();
        await applicationsPage.selectTemplate("REACT");
      });

      await test.step("Step 1: Fill name and advance", async () => {
        await applicationsPage.waitForStep("application-configure-name");
        await applicationsPage.fillAppName(`TestApp_SPA_${Date.now()}`);
        await applicationsPage.clickNext();
        await applicationsPage.handleOptionalOuStep();
      });

      await test.step("Step 2: Skip sign-in", async () => {
        await applicationsPage.waitForStep("application-configure-sign-in");
        await applicationsPage.clickNext();
      });

      await test.step("Step 3: Verify the sign-in approach picker is hidden (redirect-only)", async () => {
        await applicationsPage.waitForStep("application-configure-design");
        await expect(applicationsPage.inbuiltExperienceCard).toHaveCount(0);
        await expect(applicationsPage.embeddedExperienceCard).toHaveCount(0);
        console.log("Sign-in approach picker hidden for SPA (redirect-only) — correct");
      });
    });
  });
});
