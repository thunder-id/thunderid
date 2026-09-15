// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * User Management E2E Tests
 *
 * Covers editing a user from the details page: reverting unsaved attribute edits via Reset,
 * persisting edits via Save (verified after a tab switch and a full page reload), and resetting
 * a credential (Password) from the Credentials tab. Each user is created via the Users API,
 * same as user-deletion.spec.ts, so these stay green independent of the create-user UI flow.
 *
 * Required environment variables:
 * - BASE_URL: Console base URL
 * - ADMIN_USERNAME / ADMIN_PASSWORD: admin credentials (console sign-in + API user setup/teardown)
 *
 * Optional:
 * - SERVER_URL: backend base URL (default https://localhost:8090)
 * - TEST_USER_PASSWORD: password for generated test users (default TestPassword@123)
 */

import { expect, test, UsersApi } from "../../fixtures/console";
import { Timeouts } from "../../constants/timeouts";
import { TestDataFactory } from "../../utils/test-data";

test.describe("User Management - Edit User", () => {
  // Generated at describe scope so afterAll can see the names, same as user-deletion.spec.ts.
  const editTestUser = TestDataFactory.createUser();
  const credentialTestUser = TestDataFactory.createUser();

  test.afterAll(async ({ request }) => {
    const usersApi = new UsersApi(request);
    for (const username of [editTestUser.username, credentialTestUser.username]) {
      const deleted = await usersApi.deleteByUsername(username);
      console.log(`Teardown: removed ${deleted} user(s) matching ${username}`);
    }
  });

  /**
   * TC001: Verify Reset discards edited attributes (restoring the original values), and that a
   * subsequent Save persists edits across a tab switch and a full page reload.
   */
  test("TC001: Reset reverts edited attributes, Save persists them", async ({ usersPage, usersApi }) => {
    const user = await test.step("Create the user via the Users API", async () => {
      return usersApi.createUser(editTestUser);
    });

    const originalValues = await test.step("Open the Attributes tab and capture original values", async () => {
      await usersPage.gotoUserDetails(user.id);
      await usersPage.clickTab("Attributes");
      return usersPage.readAttributeFieldValues();
    });

    await test.step("Edit every attribute field", async () => {
      await usersPage.fillAttributeFields("reset");
      await expect(usersPage.saveChangesButton).toBeVisible({ timeout: Timeouts.ELEMENT_VISIBILITY });
    });

    await test.step("Hit Reset and verify the fields revert", async () => {
      await usersPage.clickResetChanges();
      await expect(usersPage.saveChangesButton).toBeHidden({ timeout: Timeouts.ELEMENT_VISIBILITY });
      const revertedValues = await usersPage.readAttributeFieldValues();
      expect(revertedValues).toEqual(originalValues);
    });

    let editedValues: Record<string, string> = {};

    await test.step("Edit every attribute field again", async () => {
      // Randomized so this saved value (username/email are unique across all users) can't
      // collide with another browser project's worker editing its own user concurrently.
      editedValues = await usersPage.fillAttributeFields(TestDataFactory.generateUniqueId("saved"));
    });

    await test.step("Hit Save", async () => {
      await usersPage.clickSaveChanges();
      await expect(usersPage.saveChangesButton).toBeHidden({ timeout: Timeouts.FORM_LOAD });
    });

    await test.step("Switch tabs and verify the edited values are still shown", async () => {
      await usersPage.clickTab("General");
      await usersPage.clickTab("Attributes");
      const afterTabSwitch = await usersPage.readAttributeFieldValues();
      expect(afterTabSwitch).toEqual(editedValues);
    });

    await test.step("Reload the page and verify the edited values were persisted", async () => {
      await usersPage.page.reload({ timeout: Timeouts.PAGE_LOAD });
      await usersPage.clickTab("Attributes");
      const afterReload = await usersPage.readAttributeFieldValues();
      expect(afterReload).toEqual(editedValues);
    });

    await test.step("Verify the new attribute values via the Users API", async () => {
      const apiUser = await usersApi.findByUsername(editedValues.username);
      expect(apiUser?.attributes.email).toBe(editedValues.email);
    });
  });

  /** TC002: Verify a user's password can be reset from the Credentials tab */
  test("TC002: Reset password from the Credentials tab", async ({ usersPage, usersApi }) => {
    const user = await test.step("Create the user via the Users API", async () => {
      return usersApi.createUser(credentialTestUser);
    });

    await test.step("Open the Credentials tab", async () => {
      await usersPage.gotoUserDetails(user.id);
      await usersPage.clickTab("Credentials");
    });

    await test.step("Reset the password", async () => {
      await usersPage.clickResetPassword("Password");
      await expect(usersPage.credentialResetDialog).toBeVisible({ timeout: Timeouts.ELEMENT_VISIBILITY });
      await usersPage.fillCredentialResetForm(`NewPass@${Date.now()}`);
      await usersPage.confirmResetPassword();
      await expect(usersPage.credentialResetDialog).toBeHidden({ timeout: Timeouts.FORM_LOAD });
    });
  });
});
