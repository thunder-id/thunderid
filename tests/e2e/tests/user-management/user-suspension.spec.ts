// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * User Management - Suspension E2E Tests
 *
 * Covers the lifecycle operations on a user's details page, run through the default administration
 * flows: suspending with an operator note, cancelling a suspension, unsuspending (which leaves the
 * user locked) followed by an unlock, and unsuspending and unlocking in one step. Each step checks
 * the confirmation dialog's copy, the status banner, the operations the Actions menu offers next,
 * and the status the Users API reports. Each user is created via the Users API, same as
 * user-edit.spec.ts.
 *
 * Required environment variables:
 * - BASE_URL: Console base URL
 * - ADMIN_USERNAME / ADMIN_PASSWORD: admin credentials (console sign-in + API user setup/teardown)
 */

import { expect, test, UsersApi, type UsersPage } from "../../fixtures/console";
import { TestTags } from "../../constants/test-tags";
import { Timeouts } from "../../constants/timeouts";
import { UIMessages } from "../../constants/ui-messages";
import { TestDataFactory } from "../../utils/test-data";

const { actions, user: copy } = UIMessages.accountStatus;

/** Suspend the user whose details page is open, and wait for the banner to show it. */
async function suspend(usersPage: UsersPage): Promise<void> {
  await usersPage.accountStatus.chooseAction(actions.suspend);
  await usersPage.accountStatus.confirm(actions.suspend);
  await expect(usersPage.accountStatus.statusBanner).toContainText(UIMessages.accountStatus.suspendedLabel);
}

test.describe("User Management - Suspension", { tag: [TestTags.USER_MANAGEMENT, TestTags.ACCOUNT_STATUS] }, () => {
  // Generated at describe scope so afterAll can see the names, same as user-edit.spec.ts.
  const suspendUser = TestDataFactory.createUser();
  const cancelUser = TestDataFactory.createUser();
  const unsuspendUser = TestDataFactory.createUser();
  const unsuspendAndUnlockUser = TestDataFactory.createUser();

  test.afterAll(async ({ request }) => {
    const usersApi = new UsersApi(request);
    for (const { username } of [suspendUser, cancelUser, unsuspendUser, unsuspendAndUnlockUser]) {
      const deleted = await usersApi.deleteByUsername(username);
      console.log(`Teardown: removed ${deleted} user(s) matching ${username}`);
    }
  });

  /** TC001: Suspend a user with an operator note */
  test("TC001: Suspend shows the note on the banner and offers only the release operations", async ({
    usersPage,
    usersApi,
  }) => {
    const { accountStatus } = usersPage;
    const note = `Suspended by e2e ${TestDataFactory.generateUniqueId()}`;
    const user = await test.step("Create the user via the Users API", async () => {
      return usersApi.createUser(suspendUser);
    });

    await test.step("An active user shows no banner and offers Suspend", async () => {
      await usersPage.gotoUserDetails(user.id);
      await accountStatus.waitForReady();
      await expect(accountStatus.statusBanner).toBeHidden();
      expect(await accountStatus.offeredActions()).toEqual([actions.suspend]);
    });

    await test.step("The Suspend dialog states what suspension does and takes a note", async () => {
      await accountStatus.chooseAction(actions.suspend);
      await expect(accountStatus.dialogTitle).toHaveText(copy.suspendTitle);
      await expect(accountStatus.dialog).toContainText(copy.suspendBody);
      await accountStatus.operatorNoteField.fill(note);
    });

    await test.step("Confirm, and the banner shows the suspension and the note", async () => {
      await accountStatus.confirm(actions.suspend);
      await expect(accountStatus.dialog).toBeHidden({ timeout: Timeouts.FORM_LOAD });
      await expect(accountStatus.toast(copy.toasts.suspended)).toBeVisible();
      await expect(accountStatus.statusBanner).toContainText(UIMessages.accountStatus.suspendedLabel);
      await expect(accountStatus.statusBanner).toContainText(copy.suspendedHint);
      await expect(accountStatus.statusBanner).toContainText(UIMessages.accountStatus.operatorNote(note));
    });

    await test.step("A suspended user offers Unsuspend and Unsuspend and unlock", async () => {
      expect(await accountStatus.offeredActions()).toEqual([actions.unsuspend, actions.unsuspendAndUnlock]);
    });

    await test.step("The Users API reports the suspension and the note", async () => {
      const status = (await usersApi.get(user.id)).status;
      expect(status?.value).toBe("SUSPENDED");
      expect(status?.details?.operatorNote).toBe(note);
    });
  });

  /** TC002: Cancelling the Suspend dialog changes nothing */
  test("TC002: Cancel leaves the user active", async ({ usersPage, usersApi }) => {
    const { accountStatus } = usersPage;
    const user = await usersApi.createUser(cancelUser);

    await usersPage.gotoUserDetails(user.id);
    await accountStatus.chooseAction(actions.suspend);
    await accountStatus.cancel();

    await expect(accountStatus.dialog).toBeHidden({ timeout: Timeouts.FORM_LOAD });
    await expect(accountStatus.statusBanner).toBeHidden();
    expect((await usersApi.get(user.id)).status?.value).toBe("ACTIVE");
  });

  /** TC003: Unsuspend leaves the user locked, and Unlock restores access */
  test("TC003: Unsuspend leaves the user locked until an administrator unlocks", async ({ usersPage, usersApi }) => {
    const { accountStatus } = usersPage;
    const user = await test.step("Create and suspend the user", async () => {
      const created = await usersApi.createUser(unsuspendUser);
      await usersPage.gotoUserDetails(created.id);
      await suspend(usersPage);
      return created;
    });

    await test.step("The Unsuspend dialog asks for checks first and warns of the lock", async () => {
      await accountStatus.chooseAction(actions.unsuspend);
      await expect(accountStatus.dialogTitle).toHaveText(copy.unsuspendTitle);
      await expect(accountStatus.dialog).toContainText(copy.unsuspendGuidance);
      await expect(accountStatus.dialog).toContainText(copy.unsuspendBody);
      await expect(accountStatus.operatorNoteField).toBeHidden();
    });

    await test.step("Confirm, and the user is locked rather than active", async () => {
      await accountStatus.confirm(actions.unsuspend);
      await expect(accountStatus.dialog).toBeHidden({ timeout: Timeouts.FORM_LOAD });
      await expect(accountStatus.toast(copy.toasts.unsuspended)).toBeVisible();
      await expect(accountStatus.statusBanner).toContainText(UIMessages.accountStatus.lockedLabel);
      await expect(accountStatus.statusBanner).toContainText(copy.releasedHint);
      // The hint says how a held lock ends, so no lock row repeats it.
      await expect(accountStatus.statusBanner.getByRole("listitem")).toHaveCount(0);
      expect((await usersApi.get(user.id)).status?.value).toBe("LOCKED");
    });

    await test.step("A locked user offers Unlock and Suspend", async () => {
      expect(await accountStatus.offeredActions()).toEqual([actions.unlock, actions.suspend]);
    });

    await test.step("Unlock runs without a dialog and restores access", async () => {
      await accountStatus.chooseAction(actions.unlock);
      await expect(accountStatus.toast(copy.toasts.unlocked)).toBeVisible();
      await expect(accountStatus.statusBanner).toBeHidden({ timeout: Timeouts.FORM_LOAD });
      expect((await usersApi.get(user.id)).status?.value).toBe("ACTIVE");
      expect(await accountStatus.offeredActions()).toEqual([actions.suspend]);
    });
  });

  /** TC004: Unsuspend and unlock restores access in one step */
  test("TC004: Unsuspend and unlock leaves the user active", async ({ usersPage, usersApi }) => {
    const { accountStatus } = usersPage;
    const user = await test.step("Create and suspend the user", async () => {
      const created = await usersApi.createUser(unsuspendAndUnlockUser);
      await usersPage.gotoUserDetails(created.id);
      await suspend(usersPage);
      return created;
    });

    await test.step("The Unsuspend and unlock dialog asks for checks first", async () => {
      await accountStatus.chooseAction(actions.unsuspendAndUnlock);
      await expect(accountStatus.dialogTitle).toHaveText(copy.unsuspendAndUnlockTitle);
      await expect(accountStatus.dialog).toContainText(copy.unsuspendAndUnlockGuidance);
      await expect(accountStatus.dialog).toContainText(copy.unsuspendAndUnlockBody);
    });

    await test.step("Confirm, and the user is active with no lock left", async () => {
      await accountStatus.confirm(actions.unsuspendAndUnlock);
      await expect(accountStatus.dialog).toBeHidden({ timeout: Timeouts.FORM_LOAD });
      await expect(accountStatus.toast(copy.toasts.unsuspendedAndUnlocked)).toBeVisible();
      await expect(accountStatus.statusBanner).toBeHidden({ timeout: Timeouts.FORM_LOAD });
      expect((await usersApi.get(user.id)).status?.value).toBe("ACTIVE");
      expect(await accountStatus.offeredActions()).toEqual([actions.suspend]);
    });
  });
});
