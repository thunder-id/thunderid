// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * User Management E2E Tests - Organization Access
 *
 * Covers sharing a user into other organization units from the user's Organization Access tab:
 * granting a child unit of the user's own unit (verified through the user sharing API and after a
 * reload), stopping sharing (the policy is deleted), and the read-only list of the shares other
 * units passed on. Users and units are created through the API, so these stay independent of the
 * create flows, and each test creates its own.
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

test.describe("User Management - Organization Access", () => {
  // Generated at describe scope so afterAll can see the names, same as user-edit.spec.ts.
  const grantTestUser = TestDataFactory.createUser();
  const stopTestUser = TestDataFactory.createUser();
  const reshareTestUser = TestDataFactory.createUser();
  // Parents are pushed before their children, so deleting in reverse empties each unit first.
  const createdUnitIds: string[] = [];

  test.afterAll(async ({ request }) => {
    // A user's sharing policies are deleted with the user, so the users go before the units.
    const usersApi = new UsersApi(request);
    for (const username of [grantTestUser.username, stopTestUser.username, reshareTestUser.username]) {
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

    await test.step("Open the Organization Access tab", async () => {
      await usersPage.gotoUserDetails(user.id);
      await usersPage.clickTab("Organization Access");
      await expect(access.scopeChoice(/^Only /)).toBeChecked({ timeout: Timeouts.ELEMENT_VISIBILITY });
    });

    await test.step("Grant the child unit, this unit only", async () => {
      await access.chooseScope("Chosen organization units");
      await access.chooseChildUnits("Selected child units");
      await access.grantChildUnits([{ name: child.name, reach: "This unit only" }]);
      await expect(access.grantedUnit(child.name)).toBeVisible({ timeout: Timeouts.ELEMENT_VISIBILITY });
    });

    await test.step("Save", async () => {
      await usersPage.clickSaveChanges();
      await expect(usersPage.saveChangesButton).toBeHidden({ timeout: Timeouts.FORM_LOAD });
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
      await usersPage.clickTab("Organization Access");
      await expect(access.scopeChoice("Chosen organization units")).toBeChecked({
        timeout: Timeouts.ELEMENT_VISIBILITY,
      });
      await expect(access.childUnitsChoice("Selected child units")).toBeChecked();
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

    await test.step("Open the Organization Access tab", async () => {
      await usersPage.gotoUserDetails(user.id);
      await usersPage.clickTab("Organization Access");
      await expect(access.grantedUnit(child.name)).toBeVisible({ timeout: Timeouts.ELEMENT_VISIBILITY });
    });

    await test.step("Switch to the user's own unit only and confirm", async () => {
      await access.chooseScope(/^Only /);
      await access.confirmSwitch("Stop sharing");
      await expect(access.scopeChoice(/^Only /)).toBeChecked();
    });

    await test.step("Save", async () => {
      await usersPage.clickSaveChanges();
      await expect(usersPage.saveChangesButton).toBeHidden({ timeout: Timeouts.FORM_LOAD });
    });

    await test.step("Verify the share is gone through the user sharing API", async () => {
      expect(await usersApi.listSharingPolicies(user.id)).toEqual([]);
    });
  });

  /** TC003: A share another unit passed on is listed, read-only, beneath the screen. */
  test("TC003: A unit passing the user on is listed", async ({ usersPage, usersApi, organizationUnitsApi }) => {
    const { user, child } = await test.step("Share a user into a child unit, which passes it on", async () => {
      const created = await usersApi.createUser(reshareTestUser);
      const unit = await createUnit(organizationUnitsApi, created.ouId);
      await createUnit(organizationUnitsApi, unit.id);
      await usersApi.share(created.id, { childUnits: { mode: "selected", units: [{ ouId: unit.id, reach: "unit" }] } });
      await usersApi.share(created.id, { initiatingOuId: unit.id, childUnits: { mode: "all" } });
      return { user: created, child: unit };
    });

    await test.step("Open the Organization Access tab and verify the re-share is listed", async () => {
      await usersPage.gotoUserDetails(user.id);
      await usersPage.clickTab("Organization Access");
      await expect(usersPage.passedOnCard).toBeVisible({ timeout: Timeouts.ELEMENT_VISIBILITY });
      await expect(usersPage.passedOnBy(child.name)).toBeVisible();
      await expect(
        usersPage.activeTabPanel.getByText(`All child units of ${child.name}`, { exact: true })
      ).toBeVisible();
    });
  });
});
