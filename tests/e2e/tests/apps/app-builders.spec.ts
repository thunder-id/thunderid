// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Build — Admin App and Account App builders (end-to-end)
 *
 * Verifies that what the Console's builders save is kept, and that the Admin App the data plane
 * serves follows it: a page turned off in the builder is gone from the Admin App's navigation.
 *
 * The app configurations are server-wide, so this spec runs on one browser only (see
 * SERVER_STATE_SPECS in playwright.config.ts) and puts each configuration back when it ends.
 *
 * Required environment variables:
 *   - BASE_URL: ThunderID server URL (default https://localhost:8090).
 *   - ADMIN_USERNAME / ADMIN_PASSWORD: the admin the Console signs in with.
 */

import { test, expect } from "../../fixtures/console";
import { TestTags } from "../../constants/test-tags";
import { AdminAppPage } from "../../pages/apps";

const BASE_URL = process.env.BASE_URL || "https://localhost:8090";
const ADMIN_USERNAME = process.env.ADMIN_USERNAME || "admin";
const ADMIN_PASSWORD = process.env.ADMIN_PASSWORD || "admin";

test.describe("Build — app builders", { tag: [TestTags.SMOKE] }, () => {
  test.afterEach(async ({ appBuilderPage }) => {
    // Put back what the tests change, so other suites see the default apps.
    await appBuilderPage.goto("ADMIN");
    await appBuilderPage.setPageShown("Groups", true);
    await appBuilderPage.setNavigation("Top bar");
    await appBuilderPage.saveIfChanged();

    await appBuilderPage.goto("ACCOUNT");
    await appBuilderPage.setPageShown("Security", true);
    await appBuilderPage.saveIfChanged();
  });

  test("an Admin App page turned off in the builder stays off and leaves the Admin App", async ({ appBuilderPage }) => {
    await test.step("Turn Groups off, use a side bar, and save", async () => {
      await appBuilderPage.goto("ADMIN");
      await appBuilderPage.setPageShown("Groups", false);
      await appBuilderPage.setNavigation("Side bar");
      await appBuilderPage.save();
    });

    await test.step("The saved configuration survives a reload", async () => {
      await appBuilderPage.goto("ADMIN");
      await expect(appBuilderPage.pageSwitch("Groups")).not.toBeChecked();
      await expect(appBuilderPage.pageSwitch("Users")).toBeChecked();
    });

    await test.step("Undo puts an edit back and leaves nothing to save", async () => {
      await appBuilderPage.setPageShown("Roles", false);
      await expect(appBuilderPage.saveButton).toBeEnabled();
      await appBuilderPage.undoButton.click();
      await expect(appBuilderPage.pageSwitch("Roles")).toBeChecked();
      await expect(appBuilderPage.saveButton).toBeDisabled();
    });

    await test.step("The Admin App shows the pages that are on, in a side bar", async () => {
      const adminApp = new AdminAppPage(appBuilderPage.page, BASE_URL);
      await adminApp.open(ADMIN_USERNAME, ADMIN_PASSWORD);

      const { navigation } = adminApp;
      // The side bar sits beside the content rather than in the top bar.
      await expect(adminApp.page.locator("header").getByRole("navigation", { name: "Admin App" })).toHaveCount(0);
      await expect(navigation.getByRole("link", { name: "Users" })).toBeVisible();
      await expect(navigation.getByRole("link", { name: "Groups" })).toHaveCount(0);
    });
  });

  test("the Account App builder saves its own pages", async ({ appBuilderPage }) => {
    await appBuilderPage.goto("ACCOUNT");

    await expect(appBuilderPage.pageSwitch("Personal info")).toBeVisible();
    await expect(appBuilderPage.pageSwitch("Users")).toHaveCount(0);

    await appBuilderPage.setPageShown("Security", false);
    await appBuilderPage.save();

    await appBuilderPage.goto("ACCOUNT");
    await expect(appBuilderPage.pageSwitch("Security")).not.toBeChecked();
  });
});
