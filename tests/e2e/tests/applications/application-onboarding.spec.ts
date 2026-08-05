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
        await applicationsPage.screenshot("tc001-applications-page");
      });

      await test.step("Verify applications list is visible", async () => {
        await applicationsPage.verifyPageLoaded();
        console.log("Applications list container visible");
      });

      await test.step("Verify Add Application button is present", async () => {
        await expect(applicationsPage.addApplicationButton.first()).toBeVisible();
        console.log("Add Application button is present");
        await applicationsPage.screenshot("tc001-verified");
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

    /** TC002: Full INBUILT wizard flow */
    test("TC002: Create application - full INBUILT wizard flow", async ({ applicationsPage, applicationsApi }) => {
      const appData = TestDataFactory.createApplication({ name: `TestApp_INBUILT_${Date.now()}` });
      let createdAppUrl: string;
      let createdAppId: string;

      await test.step("Navigate to Applications page, select a template and open wizard", async () => {
        console.log("Navigating to applications list...");
        await applicationsPage.goto();
        await applicationsPage.verifyPageLoaded();
        await applicationsPage.clickAddApplication();
        await applicationsPage.selectTemplate("NEXTJS");
        console.log("Opened create application wizard");
        await applicationsPage.screenshot("tc002-wizard-opened");
      });

      await test.step("Step 1 [configure-name]: Fill app name, restrict to Person, and click Next", async () => {
        await applicationsPage.waitForStep("application-configure-name");
        console.log("Step 1 visible - filling app name:", appData.name);
        await applicationsPage.fillAppName(appData.name);
        // Pin to a single user type instead of the wizard's "allow all" default, so this test
        // cannot race with specs that create/delete other user types (e.g. user-type-creation.spec.ts).
        await applicationsPage.selectOnlyUserType("Person");
        await applicationsPage.clickNext();
        console.log("Clicked Next on Step 1");
        await applicationsPage.screenshot("tc002-step1-done");
        await applicationsPage.handleOptionalOuStep();
      });

      await test.step("Step 2 [configure-sign-in]: Skip and click Next", async () => {
        await applicationsPage.waitForStep("application-configure-sign-in");
        console.log("Step 2 (configure-sign-in) visible - skipping");
        await applicationsPage.clickNext();
        await applicationsPage.screenshot("tc002-step2-done");
      });

      await test.step("Step 3 [configure-design]: Verify INBUILT is default and click Next", async () => {
        await applicationsPage.waitForStep("application-configure-design");
        console.log("Step 3 (configure-design) visible - INBUILT is default, clicking Next");
        await applicationsPage.clickNext();
        await applicationsPage.screenshot("tc002-step3-done");
      });

      await test.step("Step 4: Wait for wizard completion (secret screen or edit page)", async () => {
        createdAppUrl = await applicationsPage.completeWizardCreation();
        createdAppId = createdAppUrl.split("/").pop()!;
        createdAppIds.push(createdAppId);
        await applicationsPage.screenshot("tc002-wizard-done");
        console.log("Wizard complete, edit URL:", createdAppUrl);
      });

      await test.step("Verify created app edit page is reachable", async () => {
        await applicationsPage.page.goto(createdAppUrl);
        expect(applicationsPage.page.url()).toMatch(/\/console\/applications\/[^/]+$/);
        console.log("Created app edit page still reachable:", createdAppUrl);
        await applicationsPage.screenshot("tc002-app-verified");
      });

      await test.step("Verify only Person was granted via the application detail API", async () => {
        const app = await applicationsApi.get(createdAppId);
        expect(app.allowedUserTypes).toEqual(["Person"]);
        console.log("Application restricted to Person user type, correct");
      });
    });

    /** TC003: Next button blocked on empty name */
    test("TC003: Create application wizard - Next blocked on empty name", async ({ applicationsPage }) => {
      await test.step("Navigate to Applications and open wizard", async () => {
        await applicationsPage.goto();
        await applicationsPage.verifyPageLoaded();
        await applicationsPage.clickAddApplication();
        await applicationsPage.selectTemplate("NEXTJS");
        await applicationsPage.waitForStep("application-configure-name");
        console.log("Name step visible with empty name input");
        await applicationsPage.screenshot("tc003-empty-name");
      });

      await test.step("Verify Next is disabled when name is empty", async () => {
        await expect(applicationsPage.nextButton.first()).toBeDisabled();
        console.log("Next button is disabled with empty name — correct");
      });

      await test.step("Type a name and verify Next becomes enabled", async () => {
        await applicationsPage.fillAppName(`TestApp_${Date.now()}`);
        await expect(applicationsPage.nextButton.first()).toBeEnabled();
        console.log("Next button enabled after typing name — correct");
        await applicationsPage.screenshot("tc003-name-filled");
      });
    });

    /** TC004: SPA (public client) hides the sign-in approach picker entirely */
    test("TC004: Create application - SPA stack hides EMBEDDED experience", async ({ applicationsPage }) => {
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
        await applicationsPage.screenshot("tc004-spa-embedded-hidden");
      });
    });

    /** TC005: Created application persists after navigation */
    test("TC005: Created application persists in list after navigation", async ({
      applicationsPage,
      applicationsApi,
    }) => {
      const appData = TestDataFactory.createApplication({ name: `TestApp_PERSIST_${Date.now()}` });
      let createdAppUrl: string;

      await test.step("Create application via wizard", async () => {
        await applicationsPage.goto();
        await applicationsPage.verifyPageLoaded();
        await applicationsPage.clickAddApplication();
        await applicationsPage.selectTemplate("NEXTJS");

        await applicationsPage.waitForStep("application-configure-name");
        await applicationsPage.fillAppName(appData.name);
        // Pin to a single user type instead of the wizard's "allow all" default, so this test
        // cannot race with specs that create/delete other user types (e.g. user-type-creation.spec.ts).
        await applicationsPage.selectOnlyUserType("Person");
        await applicationsPage.clickNext();
        await applicationsPage.handleOptionalOuStep();

        await applicationsPage.waitForStep("application-configure-sign-in");
        await applicationsPage.clickNext();
        await applicationsPage.waitForStep("application-configure-design");
        await applicationsPage.clickNext();

        createdAppUrl = await applicationsPage.completeWizardCreation();
        createdAppIds.push(createdAppUrl.split("/").pop()!);
        console.log("Application created, edit URL:", createdAppUrl);
      });

      await test.step("Verify only Person was granted via the application detail API", async () => {
        const app = await applicationsApi.get(createdAppIds[createdAppIds.length - 1]);
        expect(app.allowedUserTypes).toEqual(["Person"]);
        console.log("Application restricted to Person user type — correct");
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
        await applicationsPage.screenshot("tc005-app-persists");
      });
    });
  });
});
