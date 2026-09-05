/* eslint-disable playwright/require-top-level-describe */
// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Wayfinder B2C Tryout - Self-Service Profile E2E Test
 *
 * Exercises the redirect-based self-service profile flow documented in
 * docs/content/use-cases/b2c/try-it-out/profile-section.mdx: a signed-in Customer views the
 * attributes ThunderID stores about them (read from GET /users/me) from the Wayfinder sample
 * app's own Profile page, edits their last name (PUT /users/me), and changes their password
 * (POST /users/me/update-credentials) - then signs back in with the new password to confirm the
 * credential update actually took effect.
 *
 * Uses a dedicated Customer user created via the Users API rather than the shared seed user
 * (john.doe): this test mutates the user's attributes and password, and the
 * "${browser}-wayfinder-tryout" Playwright projects (see playwright.config.ts) run this file
 * fully parallel across chromium/firefox/webkit, so a shared user would race attribute writes
 * and password changes across workers and browsers.
 *
 * This spec must run after wayfinder-sample-setup.spec.ts has imported the Wayfinder bundle (the
 * Customer user type only exists afterward) - enforced by the wayfinder-setup -> wayfinder-tryout
 * project dependency in playwright.config.ts, not by any import step in this file.
 *
 * Skipped entirely if WAYFINDER_APP_URL is not provided (matches authentication.spec.ts's
 * WAYFINDER_APP_URL pattern).
 *
 * Required environment variables:
 * - WAYFINDER_APP_URL: URL of the running Wayfinder sample app (e.g. http://localhost:5173)
 */

import { test } from "@playwright/test";
import { WayfinderAppPage, WayfinderProfilePage } from "../../../pages/wayfinder-sample";
import { TestTags } from "../../../constants/test-tags";
import { TestDataFactory } from "../../../utils/test-data";
import { UsersApi, type ApiUser } from "../../../utils/users-api";

const wayfinderUrl = process.env.WAYFINDER_APP_URL;

// Skip tests if WAYFINDER_APP_URL is not provided
const describeOrSkip = wayfinderUrl ? test.describe : test.describe.skip;

describeOrSkip("Wayfinder B2C Tryout - Profile", { tag: [TestTags.WAYFINDER] }, () => {
  const customer = TestDataFactory.createUser();
  const newLastName = `${customer.family_name}_Updated`;
  const newPassword = `${customer.password}_New1`;
  let user: ApiUser | undefined;

  test.beforeAll(async ({ request }) => {
    // beforeAll cannot take custom test-scoped fixtures, so construct the shared helper directly -
    // same class the usersApi fixture uses inside test bodies elsewhere.
    user = await new UsersApi(request).createUser(customer, "Customer");
  });

  test.afterAll(async ({ request }) => {
    if (!user) return;
    const deleted = await new UsersApi(request).deleteById(user.id);
    if (!deleted) {
      console.warn(`Failed to delete test user ${user.id} (${customer.username})`);
    }
  });

  test("View, update attributes, and change password from the self-service Profile page", async ({ page }) => {
    const wayfinderPage = new WayfinderAppPage(page);
    const profilePage = new WayfinderProfilePage(page);

    await test.step("Sign in as the Customer", async () => {
      await wayfinderPage.goto(wayfinderUrl!);
      await wayfinderPage.verifyUnAuthenticatedHomePageLoaded();
      await wayfinderPage.clickSignInButton();
      await wayfinderPage.verifyLoginPageLoaded();
      await wayfinderPage.login(customer.username, customer.password);
      await wayfinderPage.verifyLoggedIn();
    });

    await test.step("Open Profile from the account menu and view the stored attributes", async () => {
      await wayfinderPage.openProfile();
      await profilePage.verifyProfileLoaded();
      await profilePage.verifyAttributeValue("given_name", customer.given_name);
      await profilePage.verifyAttributeValue("family_name", customer.family_name);
      await profilePage.verifyAttributeValue("email", customer.email);
    });

    await test.step("Edit the last name and save", async () => {
      await profilePage.updateAttribute("family_name", newLastName);
      await profilePage.verifyAttributeValue("family_name", newLastName);
    });

    await test.step("Change the password", async () => {
      await profilePage.changePassword(customer.password, newPassword);
    });

    await test.step("Sign back in with the new password to confirm the credential update took effect", async () => {
      await wayfinderPage.logout();
      await wayfinderPage.verifyLoggedOut();
      await wayfinderPage.clickSignInButton();
      await wayfinderPage.verifyLoginPageLoaded();
      await wayfinderPage.login(customer.username, newPassword);
      await wayfinderPage.verifyLoggedIn();
    });
  });
});
