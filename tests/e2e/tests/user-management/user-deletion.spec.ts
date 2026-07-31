// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * User Management E2E Tests
 *
 * Covers deleting a user from the details page's Danger Zone. Each user is created via the
 * Users API (not the Create User wizard), so these tests stay green independent of the
 * create-user UI flow, and reached directly by id, since the users list has no search box
 * and sorts oldest-first.
 *
 * Required environment variables:
 * - BASE_URL: Console base URL
 * - ADMIN_USERNAME / ADMIN_PASSWORD: admin credentials (console sign-in + API user setup/teardown)
 *
 * Optional:
 * - SERVER_URL: backend base URL (default https://localhost:8090)
 * - TEST_USER_PASSWORD: password for generated test users (default TestPassword@123)
 */

import { ConsoleRoutes, expect, test, UsersApi } from "../../fixtures/console";
import { Timeouts } from "../../constants/timeouts";
import { TestDataFactory } from "../../utils/test-data";

test.describe("User Management - Delete User", () => {
  // Generated at describe scope so afterAll can see the names, same as user-creation.spec.ts.
  // TC002 cancels the delete, so its user still needs teardown; TC001's is gone by then, and
  // deleteByUsername is a no-op 404 in that case.
  const deletedUser = TestDataFactory.createUser();
  const keptUser = TestDataFactory.createUser();

  test.afterAll(async ({ request }) => {
    const usersApi = new UsersApi(request);
    for (const username of [deletedUser.username, keptUser.username]) {
      const deleted = await usersApi.deleteByUsername(username);
      console.log(`Teardown: removed ${deleted} user(s) matching ${username}`);
    }
  });

  /** TC001: Verify a user can be deleted from the details page's Danger Zone */
  test("TC001: Delete a user from the details page", async ({ usersPage, usersApi }) => {
    const user = await test.step("Create the user to delete via the Users API", async () => {
      return usersApi.createUser(deletedUser);
    });

    await test.step("Open the user's details page and delete it", async () => {
      await usersPage.gotoUserDetails(user.id);
      await usersPage.clickDeleteUser();
      await expect(usersPage.deleteConfirmDialog).toBeVisible({ timeout: Timeouts.ELEMENT_VISIBILITY });
      await usersPage.confirmDeleteUser();
      await usersPage.page.waitForURL(`**${ConsoleRoutes.users}`, { timeout: Timeouts.PAGE_LOAD });
    });

    await test.step("Verify the user is gone via the Users API", async () => {
      const remaining = await usersApi.findByUsername(deletedUser.username);
      expect(remaining).toBeUndefined();
    });
  });

  /** TC002: Verify cancelling the delete dialog leaves the user intact */
  test("TC002: Cancel deleting a user from the details page", async ({ usersPage, usersApi }) => {
    const user = await test.step("Create the user via the Users API", async () => {
      return usersApi.createUser(keptUser);
    });

    await test.step("Open the delete dialog and cancel it", async () => {
      await usersPage.gotoUserDetails(user.id);
      await usersPage.clickDeleteUser();
      await expect(usersPage.deleteConfirmDialog).toBeVisible({ timeout: Timeouts.ELEMENT_VISIBILITY });
      await usersPage.cancelDeleteUser();
      await expect(usersPage.deleteConfirmDialog).toBeHidden({ timeout: Timeouts.ELEMENT_VISIBILITY });
    });

    await test.step("Verify the user still exists via the Users API", async () => {
      const stillThere = await usersApi.findByUsername(keptUser.username);
      expect(stillThere?.attributes.email).toBe(keptUser.email);
    });
  });
});
