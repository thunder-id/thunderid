// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Application User Access E2E Test
 *
 * Covers the Create Application wizard's default "allow all user types" path: the master
 * "Allow all user types to access this application" checkbox (see
 * frontend/apps/console/src/features/applications/components/create-application/UserAccessSection.tsx),
 * left unchecked by every other applications spec so those specs can't race with tests that
 * create/delete user types (see application-onboarding.spec.ts TC002/TC005).
 *
 * Runs in its own file, ordered after every other project via playwright.config.ts's
 * `${browser}-user-access` projects: it reads the full set of user types in the system, so it
 * must not run while anything else could be creating or deleting one.
 *
 * Required environment variables:
 * - BASE_URL: Console base URL
 * - ADMIN_USERNAME: Admin credentials for authentication
 * - ADMIN_PASSWORD: Admin password for authentication
 */

import { test, expect, ApplicationsApi } from "../../fixtures/console";
import { TestDataFactory } from "../../utils/test-data";

test.describe("Application User Access", () => {
  const createdAppIds: string[] = [];

  test.afterAll(async ({ request }) => {
    const applicationsApi = new ApplicationsApi(request);
    for (const appId of createdAppIds) {
      const deleted = await applicationsApi.deleteById(appId);
      console.log(deleted ? `Cleaned up test app: ${appId}` : `Failed to clean up test app ${appId}`);
    }
  });

  /** TC013: Default "allow all user types" wizard path grants every existing user type */
  test("TC013: Created application allows every user type by default", async ({
    applicationsPage,
    applicationsApi,
    userTypesApi,
  }) => {
    const appData = TestDataFactory.createApplication({ name: `TestApp_ALLUSERTYPES_${Date.now()}` });
    let createdAppUrl: string;

    await test.step("Navigate to Applications page, select a template and open wizard", async () => {
      await applicationsPage.goto();
      await applicationsPage.verifyPageLoaded();
      await applicationsPage.clickAddApplication();
      await applicationsPage.selectTemplate("NEXTJS");
    });

    await test.step("Step 1 [configure-name]: Fill app name, verify allow-all is checked by default", async () => {
      await applicationsPage.waitForStep("application-configure-name");
      await applicationsPage.fillAppName(appData.name);
      await expect(applicationsPage.allowAllUserTypesCheckbox).toBeChecked();
      console.log("Allow all user types checkbox is checked by default — correct");
      await applicationsPage.clickNext();
      await applicationsPage.handleOptionalOuStep();
    });

    await test.step("Step 2 [configure-sign-in]: Skip and click Next", async () => {
      await applicationsPage.waitForStep("application-configure-sign-in");
      await applicationsPage.clickNext();
    });

    await test.step("Step 3 [configure-design]: Click Next", async () => {
      await applicationsPage.waitForStep("application-configure-design");
      await applicationsPage.clickNext();
    });

    await test.step("Step 4: Wait for wizard completion", async () => {
      createdAppUrl = await applicationsPage.completeWizardCreation();
      createdAppIds.push(createdAppUrl.split("/").pop()!);
      console.log("Application created, edit URL:", createdAppUrl);
    });

    await test.step("Verify every user type was granted via the application detail API", async () => {
      // Read after creation - this test is ordered last precisely so nothing else can create or
      // delete a user type between the wizard's own read and this one.
      const userTypeNames = (await userTypesApi.list()).map(userType => userType.name);
      const app = await applicationsApi.get(createdAppIds[createdAppIds.length - 1]);
      expect([...(app.allowedUserTypes ?? [])].sort()).toEqual([...userTypeNames].sort());
      console.log("Application allows every user type in the system — correct:", userTypeNames);
    });
  });
});
