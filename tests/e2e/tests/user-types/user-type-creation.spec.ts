// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * User Type E2E Tests
 *
 * Covers the Console's three-step Create User Type wizard. The user type is created through
 * the UI, verified through the User Types API, and removed via that API afterwards.
 *
 * Required environment variables:
 * - BASE_URL: Console base URL
 * - ADMIN_USERNAME / ADMIN_PASSWORD: admin credentials (console sign-in + API teardown)
 *
 * Optional:
 * - SERVER_URL: backend base URL (default https://localhost:8090)
 */

import { ConsoleRoutes, expect, test, UserTypesApi } from "../../fixtures/console";
import { Timeouts } from "../../constants/timeouts";
import { TestDataFactory } from "../../utils/test-data";

test.describe("User Types - Create User Type", () => {
  // Generated at describe scope so afterAll can see the name. Each browser project runs this
  // file in its own worker, so concurrent runs get distinct names and cannot collide.
  //
  // The "zz" prefix is load bearing: GET /user-types sorts by name, and the Create User
  // wizard offers that order as-is while user-management's specs pick its first entry. A name
  // sorting before "Person" would make those specs fill a form for this schema instead.
  const userTypeName = `zz_e2e_user_type_${TestDataFactory.generateUniqueId()}`;

  test.afterAll(async ({ request }) => {
    // beforeAll/afterAll cannot take custom test-scoped fixtures, so construct the shared
    // helper directly here - same class the userTypesApi fixture uses inside the test below.
    const userTypesApi = new UserTypesApi(request);
    const deleted = await userTypesApi.deleteByName(userTypeName);
    console.log(`Teardown: removed ${deleted ? 1 : 0} user type(s) matching ${userTypeName}`);
  });

  /** TC001: Verify a user type can be created through the Console wizard */
  test("TC001: Create new user type with a schema property", async ({ userTypesPage, userTypesApi }) => {
    await test.step("Open the Create User Type wizard", async () => {
      await userTypesPage.openCreateWizard();
    });

    await test.step("Name the user type", async () => {
      // A single-organization-unit deployment auto-selects the only unit, so the wizard is
      // Details -> Properties with no organization-unit step in between.
      await userTypesPage.fillName(userTypeName);
      await userTypesPage.continueTo("properties");
    });

    await test.step("Add the Email property from the attribute library", async () => {
      await userTypesPage.addLibraryProperty("Email");
    });

    await test.step("Submit and land back on the user types list", async () => {
      await userTypesPage.submit();
      await userTypesPage.page.waitForURL(`**${ConsoleRoutes.userTypes}`, { timeout: Timeouts.PAGE_LOAD });
    });

    await test.step("Verify the new user type via the User Types API", async () => {
      const listed = await userTypesApi.findByName(userTypeName);
      expect(listed).toBeDefined();

      // The list endpoint omits `schema`, so read the type by id to assert on it.
      const userType = await userTypesApi.get(listed!.id);
      expect(userType.schema?.email).toMatchObject({ type: "string", required: true });
    });
  });
});
