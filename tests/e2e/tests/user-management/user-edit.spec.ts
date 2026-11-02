// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * User Management E2E Tests
 *
 * Covers editing a user's attributes from its details page: Cancel discards the edits, and Save
 * persists them (verified after a full page reload and through the Users API). The user is created
 * via the Users API, same as user-deletion.spec.ts, so this stays green independent of the Add
 * people flow.
 *
 * Required environment variables:
 * - BASE_URL: Admin Console base URL
 * - ADMIN_USERNAME / ADMIN_PASSWORD: admin credentials (sign-in + API user setup/teardown)
 *
 * Optional:
 * - SERVER_URL: backend base URL (default https://localhost:8090)
 * - TEST_USER_PASSWORD: password for generated test users (default TestPassword@123)
 */

import { expect, test, UsersApi } from "../../fixtures/console";
import { Timeouts } from "../../constants/timeouts";
import { TestDataFactory } from "../../utils/test-data";

test.describe("User Management - Edit User", () => {
  // Generated at describe scope so afterAll can see the name, same as user-deletion.spec.ts.
  const editTestUser = TestDataFactory.createUser();
  let editedUsername = editTestUser.username;

  test.afterAll(async ({ request }) => {
    const usersApi = new UsersApi(request);
    const deleted = await usersApi.deleteByUsername(editedUsername);
    console.log(`Teardown: removed ${deleted} user(s) matching ${editedUsername}`);
  });

  /** TC001: Verify Cancel discards edited attributes, and Save persists them across a reload. */
  test("TC001: Cancel discards edited attributes, Save persists them", async ({ usersPage, usersApi }) => {
    const user = await test.step("Create the user via the Users API", async () => {
      return usersApi.createUser(editTestUser);
    });

    const originalValues = await test.step("Open the details page and read the attributes", async () => {
      await usersPage.gotoUserDetails(user.id);
      return usersPage.readAttributeValues();
    });

    await test.step("Edit every attribute, then cancel and verify nothing changed", async () => {
      await usersPage.clickEdit();
      await usersPage.fillAttributeFields("discarded");
      await usersPage.clickCancelEdit();
      expect(await usersPage.readAttributeValues()).toEqual(originalValues);
    });

    let editedValues: Record<string, string> = {};

    await test.step("Edit every attribute again and save", async () => {
      await usersPage.clickEdit();
      // Randomized so this saved value (username/email are unique across all users) can't collide
      // with another browser project's worker editing its own user concurrently.
      editedValues = await usersPage.fillAttributeFields(TestDataFactory.generateUniqueId("saved"));
      editedUsername = editedValues.username;
      await usersPage.clickSaveChanges();
      await expect.poll(() => usersPage.readAttributeValues(), { timeout: Timeouts.FORM_LOAD }).toEqual(editedValues);
    });

    await test.step("Reload the page and verify the edited values were persisted", async () => {
      await usersPage.page.reload({ timeout: Timeouts.PAGE_LOAD });
      await usersPage.editButton.waitFor({ state: "visible", timeout: Timeouts.PAGE_LOAD });
      expect(await usersPage.readAttributeValues()).toEqual(editedValues);
    });

    await test.step("Verify the new attribute values via the Users API", async () => {
      const apiUser = await usersApi.findByUsername(editedValues.username);
      expect(apiUser?.attributes.email).toBe(editedValues.email);
    });
  });
});
