// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * User Management E2E Tests
 *
 * Covers the two Console onboarding wizards: Create User and Invite User.
 * Both users are created through the UI and removed via the Users API afterwards.
 *
 * Required environment variables:
 * - BASE_URL: Console base URL
 * - ADMIN_USERNAME / ADMIN_PASSWORD: admin credentials (console sign-in + API teardown)
 *
 * Optional:
 * - SERVER_URL: backend base URL (default https://localhost:8090)
 * - TEST_USER_PASSWORD: password for generated test users (default TestPassword@123)
 */

import { ConsoleRoutes, expect, test, UsersApi, UsersPage } from "../../fixtures/console";
import { Timeouts } from "../../constants/timeouts";
import { TestDataFactory } from "../../utils/test-data";

const baseUrl = process.env.BASE_URL as string;

test.describe("User Management - Create User", () => {
  // Generated at describe scope so afterAll can see the names. Each browser project runs
  // this file in its own worker, so concurrent chromium/firefox/webkit runs get distinct
  // usernames and cannot collide.
  const createdUser = TestDataFactory.createUser();
  const invitedUser = TestDataFactory.createUser();

  test.afterAll(async ({ request }) => {
    // beforeAll/afterAll cannot take custom test-scoped fixtures, so construct the shared
    // helper directly here - same class the usersApi fixture uses inside the tests below.
    const usersApi = new UsersApi(request);
    for (const username of [createdUser.username, invitedUser.username]) {
      const deleted = await usersApi.deleteByUsername(username);
      console.log(`Teardown: removed ${deleted} user(s) matching ${username}`);
    }
  });

  /** TC001: Verify user can be created with all required fields */
  test("TC001: Create new user with all required fields", async ({ usersPage, usersApi }) => {
    await test.step("Open the Create User wizard", async () => {
      await usersPage.openAddUserWizard("create");
    });

    await test.step("Fill in the user details", async () => {
      await usersPage.fillUserForm(createdUser);
    });

    await test.step("Submit and close the completion screen", async () => {
      await usersPage.submitForm();
      await expect(
        usersPage.page.locator("h1, h2, h3, h4, h5, h6").filter({ hasText: /user added successfully/i }).first(),
      ).toBeVisible({ timeout: Timeouts.FORM_LOAD });
      await usersPage.closeWizard();
      await usersPage.page.waitForURL(`**${ConsoleRoutes.users}`, { timeout: Timeouts.PAGE_LOAD });
    });

    await test.step("Verify the new user via the Users API", async () => {
      const user = await usersApi.findByUsername(createdUser.username);
      expect(user?.attributes.email).toBe(createdUser.email);
    });
  });

  /** TC002: Verify user can be invited with an invite link */
  test("TC002: Invite new user with an invite link", async ({ usersPage, usersApi, isolatedPage }) => {
    let inviteLink = "";

    await test.step("Open the Invite User wizard", async () => {
      await usersPage.openAddUserWizard("invite");
    });

    await test.step("Fill in the invitee's details", async () => {
      await usersPage.fillUserForm(invitedUser);
      await usersPage.clickNextButton();
    });

    await test.step("Generate the invite link", async () => {
      await usersPage.clickGetInviteLink();
      await expect(usersPage.copyInviteLinkButton.first()).toBeVisible({ timeout: Timeouts.ELEMENT_VISIBILITY });
      inviteLink = await usersPage.getInviteLink();
      expect(inviteLink).toMatch(/^https?:\/\//);
    });

    await test.step("Close the invite wizard", async () => {
      await usersPage.closeWizard();
      await usersPage.page.waitForURL(`**${ConsoleRoutes.users}`, { timeout: Timeouts.PAGE_LOAD });
    });

    await test.step("Follow the invite link and complete registration", async () => {
      // isolatedPage is a fresh browser context to avoid existing session cookies from main text context
      await isolatedPage.goto(inviteLink, { timeout: Timeouts.PAGE_LOAD });

      await new UsersPage(isolatedPage, baseUrl).completeRegistrationFlow(invitedUser);

      // The completion step renders only TEXT components: no form, no submit button.
      await expect(isolatedPage.locator('form button[type="submit"]')).toHaveCount(0, {
        timeout: Timeouts.FORM_LOAD,
      });
    });

    await test.step("Verify the registered user via the Users API", async () => {
      // Using the API as UI defaults to 10 results per page
      const user = await usersApi.findByUsername(invitedUser.username);
      expect(user?.attributes.email).toBe(invitedUser.email);
    });
  });
});
