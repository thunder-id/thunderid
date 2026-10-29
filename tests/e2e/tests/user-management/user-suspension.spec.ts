// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * User Management - Suspension E2E Tests
 *
 * Covers the lifecycle actions on a user's details page, run through the default administration
 * flows: suspending with an operator note, cancelling a suspension, and unsuspending (which leaves
 * the user locked) followed by an unlock. Each step checks the confirmation's copy, the status the
 * page shows, the actions it offers next, and the status the Users API reports. Each user is created
 * via the Users API, same as user-edit.spec.ts.
 *
 * Required environment variables:
 * - BASE_URL: Admin Console base URL
 * - ADMIN_USERNAME / ADMIN_PASSWORD: admin credentials (sign-in + API user setup/teardown)
 */

import { expect, test, UsersApi } from "../../fixtures/console";
import { TestTags } from "../../constants/test-tags";
import { Timeouts } from "../../constants/timeouts";
import { UIMessages } from "../../constants/ui-messages";
import { TestDataFactory } from "../../utils/test-data";

const { actions, status } = UIMessages.accountStatus;

test.describe("User Management - Suspension", { tag: [TestTags.USER_MANAGEMENT, TestTags.ACCOUNT_STATUS] }, () => {
  // Generated at describe scope so afterAll can see the names, same as user-edit.spec.ts.
  const suspendUser = TestDataFactory.createUser();
  const cancelUser = TestDataFactory.createUser();
  const unsuspendUser = TestDataFactory.createUser();

  test.afterAll(async ({ request }) => {
    const usersApi = new UsersApi(request);
    for (const { username } of [suspendUser, cancelUser, unsuspendUser]) {
      const deleted = await usersApi.deleteByUsername(username);
      console.log(`Teardown: removed ${deleted} user(s) matching ${username}`);
    }
  });

  /** TC001: Suspend a user with an operator note */
  test("TC001: Suspend records the note and offers only Unsuspend", async ({ usersPage, usersApi }) => {
    const { accountStatus } = usersPage;
    const note = `Suspended by e2e ${TestDataFactory.generateUniqueId()}`;
    const user = await test.step("Create the user via the Users API", async () => {
      return usersApi.createUser(suspendUser);
    });

    await test.step("An active user offers Suspend", async () => {
      await usersPage.gotoUserDetails(user.id);
      await accountStatus.waitForReady();
      await expect(accountStatus.status).toHaveText(status.active);
      await expect(accountStatus.actionButtons).toHaveText([actions.suspend]);
    });

    await test.step("Suspend asks for confirmation and takes a note", async () => {
      await accountStatus.chooseAction(actions.suspend);
      await expect(accountStatus.confirmation).toContainText(
        UIMessages.accountStatus.suspendQuestion(suspendUser.username)
      );
      await accountStatus.operatorNoteField.fill(note);
    });

    await test.step("Confirm, and the user is suspended", async () => {
      await accountStatus.confirm(actions.suspend);
      await expect(accountStatus.confirmation).toBeHidden({ timeout: Timeouts.FORM_LOAD });
      await expect(accountStatus.status).toHaveText(status.suspended, { timeout: Timeouts.FORM_LOAD });
      await expect(accountStatus.actionButtons).toHaveText([actions.unsuspend]);
    });

    await test.step("The Users API reports the suspension and the note", async () => {
      const apiStatus = (await usersApi.get(user.id)).status;
      expect(apiStatus?.value).toBe("SUSPENDED");
      expect(apiStatus?.details?.operatorNote).toBe(note);
    });
  });

  /** TC002: Cancelling the confirmation changes nothing */
  test("TC002: Cancel leaves the user active", async ({ usersPage, usersApi }) => {
    const { accountStatus } = usersPage;
    const user = await usersApi.createUser(cancelUser);

    await usersPage.gotoUserDetails(user.id);
    await accountStatus.waitForReady();
    await accountStatus.chooseAction(actions.suspend);
    await accountStatus.cancel();

    await expect(accountStatus.confirmation).toBeHidden({ timeout: Timeouts.FORM_LOAD });
    await expect(accountStatus.status).toHaveText(status.active);
    await expect(accountStatus.actionButtons).toHaveText([actions.suspend]);
    expect((await usersApi.get(user.id)).status?.value).toBe("ACTIVE");
  });

  /** TC003: Unsuspend leaves the user locked, and Unlock restores access */
  test("TC003: Unsuspend leaves the user locked until an administrator unlocks", async ({ usersPage, usersApi }) => {
    const { accountStatus } = usersPage;
    const user = await test.step("Create and suspend the user", async () => {
      const created = await usersApi.createUser(unsuspendUser);
      await usersPage.gotoUserDetails(created.id);
      await accountStatus.waitForReady();
      await accountStatus.chooseAction(actions.suspend);
      await accountStatus.confirm(actions.suspend);
      await expect(accountStatus.status).toHaveText(status.suspended, { timeout: Timeouts.FORM_LOAD });
      return created;
    });

    await test.step("Unsuspend asks for confirmation and takes no note", async () => {
      await accountStatus.chooseAction(actions.unsuspend);
      await expect(accountStatus.confirmation).toContainText(
        UIMessages.accountStatus.unsuspendQuestion(unsuspendUser.username)
      );
      await expect(accountStatus.operatorNoteField).toBeHidden();
    });

    await test.step("Confirm, and the user is locked rather than active", async () => {
      await accountStatus.confirm(actions.unsuspend);
      await expect(accountStatus.status).toHaveText(status.locked, { timeout: Timeouts.FORM_LOAD });
      await expect(accountStatus.actionButtons).toHaveText([actions.unlock, actions.suspend]);
      expect((await usersApi.get(user.id)).status?.value).toBe("LOCKED");
    });

    await test.step("Unlock runs without confirmation and restores access", async () => {
      await accountStatus.chooseAction(actions.unlock);
      await expect(accountStatus.status).toHaveText(status.active, { timeout: Timeouts.FORM_LOAD });
      await expect(accountStatus.actionButtons).toHaveText([actions.suspend]);
      expect((await usersApi.get(user.id)).status?.value).toBe("ACTIVE");
    });
  });
});
