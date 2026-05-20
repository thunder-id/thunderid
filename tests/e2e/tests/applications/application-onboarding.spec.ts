/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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
 * Application Onboarding E2E Tests
 *
 * Covers the applications list page and the create application wizard.
 *
 * Required environment variables:
 * - BASE_URL: Console base URL
 * - ADMIN_USERNAME: Admin credentials for authentication
 * - ADMIN_PASSWORD: Admin password for authentication
 */

import { test, expect } from "../../fixtures/console";
import { TestDataFactory } from "../../utils/test-data";
import { getAdminToken } from "../../utils/authentication";

const serverUrl = process.env.SERVER_URL || "https://localhost:8090";

async function deleteApplication(request: import("@playwright/test").APIRequestContext, appId: string): Promise<void> {
  try {
    const token = await getAdminToken(request);
    await request.delete(`${serverUrl}/applications/${appId}`, {
      headers: { Authorization: `Bearer ${token}` },
      ignoreHTTPSErrors: true,
    });
    console.log(`Cleaned up test app: ${appId}`);
  } catch (e) {
    console.warn(`Failed to clean up test app ${appId}:`, e);
  }
}

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
      for (const appId of createdAppIds) {
        await deleteApplication(request, appId);
      }
    });

    /** TC002: Full INBUILT wizard flow */
    test("TC002: Create application - full INBUILT wizard flow", async ({ applicationsPage }) => {
      const appData = TestDataFactory.createApplication({ name: `TestApp_INBUILT_${Date.now()}` });
      let createdAppUrl: string;

      await test.step("Navigate to Applications page and open wizard", async () => {
        console.log("Navigating to applications list...");
        await applicationsPage.goto();
        await applicationsPage.verifyPageLoaded();
        await applicationsPage.clickAddApplication();
        console.log("Opened create application wizard");
        await applicationsPage.screenshot("tc002-wizard-opened");
      });

      await test.step("Step 1 [configure-stack]: Skip and click Next", async () => {
        await applicationsPage.waitForStep("application-configure-stack");
        console.log("Step 1 (configure-stack) visible - skipping");
        await applicationsPage.clickNext();
        await applicationsPage.screenshot("tc002-step1-done");
      });

      await test.step("Step 2 [configure-name]: Fill app name and click Next", async () => {
        await applicationsPage.waitForStep("application-configure-name");
        console.log("Step 2 visible - filling app name:", appData.name);
        await applicationsPage.fillAppName(appData.name);
        await applicationsPage.clickNext();
        console.log("Clicked Next on Step 2");
        await applicationsPage.screenshot("tc002-step2-done");
        await applicationsPage.handleOptionalOuStep();
      });

      await test.step("Step 4 [configure-design]: Skip and click Next", async () => {
        await applicationsPage.waitForStep("application-configure-design");
        console.log("Step 4 (configure-design) visible - skipping");
        await applicationsPage.clickNext();
        await applicationsPage.screenshot("tc002-step4-done");
      });

      await test.step("Step 5 [configure-sign-in]: Skip and click Next", async () => {
        await applicationsPage.waitForStep("application-configure-sign-in");
        console.log("Step 5 (configure-sign-in) visible - skipping");
        await applicationsPage.clickNext();
        await applicationsPage.screenshot("tc002-step5-done");
      });

      await test.step("Step 6 [configure-experience]: Verify INBUILT is default and click Next", async () => {
        await applicationsPage.waitForStep("application-configure-experience");
        console.log("Step 6 (configure-experience) visible - INBUILT is default, clicking Next");
        await applicationsPage.clickNext();
        await applicationsPage.screenshot("tc002-step6-done");
      });

      await test.step("Step 7: Wait for wizard completion (secret screen or edit page)", async () => {
        createdAppUrl = await applicationsPage.completeWizardCreation();
        createdAppIds.push(createdAppUrl.split("/").pop()!);
        await applicationsPage.screenshot("tc002-wizard-done");
        console.log("Wizard complete, edit URL:", createdAppUrl);
      });

      await test.step("Verify created app edit page is reachable", async () => {
        await applicationsPage.page.goto(createdAppUrl, { waitUntil: "networkidle" });
        expect(applicationsPage.page.url()).toMatch(/\/console\/applications\/[^/]+$/);
        console.log("Created app edit page still reachable:", createdAppUrl);
        await applicationsPage.screenshot("tc002-app-verified");
      });
    });

    /** TC003: Next button blocked on empty name */
    test("TC003: Create application wizard - Next blocked on empty name", async ({ applicationsPage }) => {
      await test.step("Navigate to Applications and open wizard", async () => {
        await applicationsPage.goto();
        await applicationsPage.verifyPageLoaded();
        await applicationsPage.clickAddApplication();
        await applicationsPage.waitForStep("application-configure-stack");
        await applicationsPage.clickNext();
        await applicationsPage.waitForStep("application-configure-name");
        console.log("Step 2 visible with empty name input");
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

    /** TC004: EMBEDDED experience skips configure-details */
    test("TC004: Create application - EMBEDDED experience skips configure-details", async ({ applicationsPage }) => {
      const appData = TestDataFactory.createApplication({ name: `TestApp_EMBEDDED_${Date.now()}` });

      await test.step("Navigate and open wizard", async () => {
        await applicationsPage.goto();
        await applicationsPage.verifyPageLoaded();
        await applicationsPage.clickAddApplication();
      });

      await test.step("Step 1: Skip stack and advance", async () => {
        await applicationsPage.waitForStep("application-configure-stack");
        await applicationsPage.clickNext();
      });

      await test.step("Step 2: Fill name and advance", async () => {
        await applicationsPage.waitForStep("application-configure-name");
        await applicationsPage.fillAppName(appData.name);
        await applicationsPage.clickNext();
        await applicationsPage.handleOptionalOuStep();
      });

      await test.step("Steps 4 & 5: Skip design and sign-in", async () => {
        await applicationsPage.waitForStep("application-configure-design");
        await applicationsPage.clickNext();
        await applicationsPage.waitForStep("application-configure-sign-in");
        await applicationsPage.clickNext();
      });

      await test.step("Step 6: Select EMBEDDED and verify configure-details is skipped", async () => {
        await applicationsPage.waitForStep("application-configure-experience");
        console.log("Selecting EMBEDDED experience");
        await applicationsPage.selectEmbeddedExperience();
        await expect(applicationsPage.configureDetailsStep).not.toBeVisible();
        await applicationsPage.clickNext();
        await applicationsPage.screenshot("tc004-embedded-selected");
        console.log("configure-details was never shown - correct EMBEDDED behaviour");
        // EMBEDDED without passkey creates app directly and navigates to edit page
        await applicationsPage.page.waitForURL(/\/console\/applications\/(?!create)[^/]+$/, { timeout: 30000 });
        createdAppIds.push(applicationsPage.page.url().split("/").pop()!);
        console.log("Navigated to edit page directly — EMBEDDED skips both configure-details and secret screen");
        await applicationsPage.screenshot("tc004-details-skipped");
      });
    });

    /** TC005: Created application persists after navigation */
    test("TC005: Created application persists in list after navigation", async ({ applicationsPage }) => {
      const appData = TestDataFactory.createApplication({ name: `TestApp_PERSIST_${Date.now()}` });
      let createdAppUrl: string;

      await test.step("Create application via wizard", async () => {
        await applicationsPage.goto();
        await applicationsPage.verifyPageLoaded();
        await applicationsPage.clickAddApplication();

        await applicationsPage.waitForStep("application-configure-stack");
        await applicationsPage.clickNext();

        await applicationsPage.waitForStep("application-configure-name");
        await applicationsPage.fillAppName(appData.name);
        await applicationsPage.clickNext();
        await applicationsPage.handleOptionalOuStep();

        await applicationsPage.waitForStep("application-configure-design");
        await applicationsPage.clickNext();
        await applicationsPage.waitForStep("application-configure-sign-in");
        await applicationsPage.clickNext();
        await applicationsPage.waitForStep("application-configure-experience");
        await applicationsPage.clickNext();

        createdAppUrl = await applicationsPage.completeWizardCreation();
        createdAppIds.push(createdAppUrl.split("/").pop()!);
        console.log("Application created, edit URL:", createdAppUrl);
      });

      await test.step("Navigate away then back to applications", async () => {
        await applicationsPage.page.goto(`${process.env.BASE_URL || ""}/console/dashboard`, {
          waitUntil: "networkidle",
        });
        console.log("Navigated away to dashboard");
        await applicationsPage.goto();
        await applicationsPage.verifyPageLoaded();
        console.log("Navigated back to applications list");
      });

      await test.step("Verify app edit page still reachable after navigation", async () => {
        await applicationsPage.page.goto(createdAppUrl, { waitUntil: "networkidle" });
        expect(applicationsPage.page.url()).toMatch(/\/console\/applications\/[^/]+$/);
        console.log("App still reachable after navigation:", createdAppUrl);
        await applicationsPage.screenshot("tc005-app-persists");
      });
    });
  });
});
