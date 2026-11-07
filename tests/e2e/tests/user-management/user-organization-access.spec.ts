// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * User Management E2E Tests - Affiliations
 *
 * Covers sharing a user into other organization units from the user's Affiliations tab: granting
 * a child unit of the user's own unit (verified through the user sharing API and after a reload),
 * and stopping sharing (the policy is deleted). Users and units are created through the API, so
 * these stay independent of the create flows, and each test creates its own.
 *
 * Required environment variables:
 * - BASE_URL: Console base URL
 * - ADMIN_USERNAME / ADMIN_PASSWORD: admin credentials (console sign-in + API setup/teardown)
 *
 * Optional:
 * - SERVER_URL: backend base URL (default https://localhost:8090)
 */

import { expect, test, OrganizationUnitsApi, UsersApi, type ApiOrganizationUnit } from "../../fixtures/console";
import { Timeouts } from "../../constants/timeouts";
import { TestDataFactory } from "../../utils/test-data";

// The scope choices are titled after the user's own unit: "<unit> organization unit only" and "<unit> and chosen organization units".
const OWNER_ONLY = /^.+ (organization unit )?only\b/;
const CHOSEN_UNITS = /^.+ and chosen (organization )?units\b/;

test.describe("User Management - Affiliations", () => {
  // Generated at describe scope so afterAll can see the names, same as user-edit.spec.ts.
  const grantTestUser = TestDataFactory.createUser();
  const stopTestUser = TestDataFactory.createUser();
  // Parents are pushed before their children, so deleting in reverse empties each unit first.
  const createdUnitIds: string[] = [];

  test.afterAll(async ({ request }) => {
    // A user's sharing policies are deleted with the user, so the users go before the units.
    const usersApi = new UsersApi(request);
    for (const username of [grantTestUser.username, stopTestUser.username]) {
      const deleted = await usersApi.deleteByUsername(username);
      console.log(`Teardown: removed ${deleted} user(s) matching ${username}`);
    }
    const unitsApi = new OrganizationUnitsApi(request);
    for (const id of [...createdUnitIds].reverse()) {
      if (!(await unitsApi.deleteById(id))) console.warn(`Teardown: failed to delete organization unit ${id}`);
    }
  });

  /** Create a uniquely named unit under `parent`, recorded for teardown. */
  async function createUnit(unitsApi: OrganizationUnitsApi, parent: string): Promise<ApiOrganizationUnit> {
    const handle = TestDataFactory.generateUniqueId("e2e-ou");
    const unit = await unitsApi.create(handle, `E2E unit ${handle}`, parent);
    createdUnitIds.push(unit.id);
    return unit;
  }

  /** TC001: Grant a child unit of the user's own unit, save, and verify the policy and the reload. */
  test("TC001: Granting a child unit records a share", async ({ usersPage, usersApi, organizationUnitsApi }) => {
    const { user, child } = await test.step("Create a user and a child unit of its organization unit", async () => {
      const created = await usersApi.createUser(grantTestUser);
      return { user: created, child: await createUnit(organizationUnitsApi, created.ouId) };
    });
    const access = usersPage.organizationAccess;

    await test.step("Open the Affiliations tab", async () => {
      await usersPage.gotoUserDetails(user.id);
      await usersPage.clickTab("Affiliations");
      await expect(access.scopeChoice(OWNER_ONLY)).toBeChecked({ timeout: Timeouts.ELEMENT_VISIBILITY });
    });

    await test.step("Grant the child unit, this unit only", async () => {
      await access.chooseScope(CHOSEN_UNITS);
      await access.chooseChildUnits("Selected child organization units");
      await access.grantChildUnits([{ name: child.name, reach: "Only this OU" }]);
      await expect(access.grantedUnit(child.name)).toBeVisible({ timeout: Timeouts.ELEMENT_VISIBILITY });
    });

    await test.step("Save", async () => {
      await usersPage.clickSaveChanges();
    });

    await test.step("Verify the share through the user sharing API", async () => {
      const policies = await usersApi.listSharingPolicies(user.id);
      expect(policies).toHaveLength(1);
      expect(policies[0].stage).toBe("share");
      expect(policies[0].initiatingOuId).toBe(user.ouId);
      expect(policies[0].childUnits).toEqual({ mode: "selected", units: [{ ouId: child.id, reach: "unit" }] });
    });

    await test.step("Reload and verify the tab shows the saved share", async () => {
      await usersPage.page.reload({ timeout: Timeouts.PAGE_LOAD });
      await usersPage.clickTab("Affiliations");
      await expect(access.scopeChoice(CHOSEN_UNITS)).toBeChecked({
        timeout: Timeouts.ELEMENT_VISIBILITY,
      });
      await expect(access.childUnitsChoice("Selected child organization units")).toBeChecked();
      await expect(access.grantedUnit(child.name)).toBeVisible();
    });
  });

  /** TC002: Switch a shared user back to its own unit only, confirm, save, and verify the policy is gone. */
  test("TC002: Stopping sharing deletes the share", async ({ usersPage, usersApi, organizationUnitsApi }) => {
    const { user, child } = await test.step("Create a user shared into a child unit", async () => {
      const created = await usersApi.createUser(stopTestUser);
      const unit = await createUnit(organizationUnitsApi, created.ouId);
      await usersApi.share(created.id, { childUnits: { mode: "selected", units: [{ ouId: unit.id, reach: "unit" }] } });
      return { user: created, child: unit };
    });
    const access = usersPage.organizationAccess;

    await test.step("Open the Affiliations tab", async () => {
      await usersPage.gotoUserDetails(user.id);
      await usersPage.clickTab("Affiliations");
      await expect(access.grantedUnit(child.name)).toBeVisible({ timeout: Timeouts.ELEMENT_VISIBILITY });
    });

    await test.step("Switch to the user's own unit only and confirm", async () => {
      await access.chooseScope(OWNER_ONLY);
      await access.confirmSwitch("Remove affiliations");
      await expect(access.scopeChoice(OWNER_ONLY)).toBeChecked();
    });

    await test.step("Save", async () => {
      await usersPage.clickSaveChanges();
    });

    await test.step("Verify the share is gone through the user sharing API", async () => {
      expect(await usersApi.listSharingPolicies(user.id)).toEqual([]);
    });
  });
});
