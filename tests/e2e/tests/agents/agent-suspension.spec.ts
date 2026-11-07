// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Agents - Suspension E2E Tests
 *
 * Covers suspending and unsuspending an agent from its details page through the default
 * administration flows. Unlike a user, an agent is not left locked by an unsuspend, so it returns
 * straight to active and the menu never offers Unsuspend and unlock.
 *
 * Required environment variables:
 * - BASE_URL: Console base URL
 * - ADMIN_USERNAME / ADMIN_PASSWORD: admin credentials (console sign-in + API agent setup/teardown)
 */

import { AgentsApi, expect, test } from "../../fixtures/console";
import { TestTags } from "../../constants/test-tags";
import { Timeouts } from "../../constants/timeouts";
import { UIMessages } from "../../constants/ui-messages";
import { TestDataFactory } from "../../utils/test-data";

const { actions, agent: copy } = UIMessages.accountStatus;

test.describe("Agents - Suspension", { tag: [TestTags.ACCOUNT_STATUS] }, () => {
  const agentName = TestDataFactory.generateUniqueId("zz_e2e_agent");

  test.afterAll(async ({ request }) => {
    const deleted = await new AgentsApi(request).deleteByName(agentName);
    console.log(`Teardown: ${deleted ? "removed" : "found no"} agent ${agentName}`);
  });

  /** TC001: Suspend an agent with an operator note, then unsuspend it */
  test("TC001: Suspend and unsuspend an agent", async ({ agentDetailsPage, agentsApi }) => {
    const { accountStatus } = agentDetailsPage;
    const note = `Suspended by e2e ${TestDataFactory.generateUniqueId()}`;
    const agent = await test.step("Create the agent via the Agents API", async () => {
      return agentsApi.create(agentName);
    });

    await test.step("An active agent offers Suspend only", async () => {
      await agentDetailsPage.goto(agent.id);
      await accountStatus.waitForReady();
      await expect(accountStatus.statusBanner).toBeHidden();
      expect(await accountStatus.offeredActions()).toEqual([actions.suspend]);
    });

    await test.step("Suspend with a note", async () => {
      await accountStatus.chooseAction(actions.suspend);
      await expect(accountStatus.dialogTitle).toHaveText(copy.suspendTitle);
      await expect(accountStatus.dialog).toContainText(copy.suspendBody);
      await accountStatus.operatorNoteField.fill(note);
      await accountStatus.confirm(actions.suspend);
      await expect(accountStatus.dialog).toBeHidden({ timeout: Timeouts.FORM_LOAD });
      await expect(accountStatus.toast(copy.toasts.suspended)).toBeVisible();
    });

    await test.step("The banner shows the suspension and the note", async () => {
      await expect(accountStatus.statusBanner).toContainText(UIMessages.accountStatus.suspendedLabel);
      await expect(accountStatus.statusBanner).toContainText(copy.suspendedHint);
      await expect(accountStatus.statusBanner).toContainText(UIMessages.accountStatus.operatorNote(note));
      expect((await agentsApi.get(agent.id)).status?.value).toBe("SUSPENDED");
    });

    await test.step("A suspended agent offers Unsuspend only", async () => {
      expect(await accountStatus.offeredActions()).toEqual([actions.unsuspend]);
    });

    await test.step("The Unsuspend dialog asks for secret rotation first", async () => {
      await accountStatus.chooseAction(actions.unsuspend);
      await expect(accountStatus.dialogTitle).toHaveText(copy.unsuspendTitle);
      await expect(accountStatus.dialog).toContainText(copy.unsuspendGuidance);
      await expect(accountStatus.dialog).toContainText(copy.unsuspendBody);
    });

    await test.step("Confirm, and the agent is active again", async () => {
      await accountStatus.confirm(actions.unsuspend);
      await expect(accountStatus.dialog).toBeHidden({ timeout: Timeouts.FORM_LOAD });
      await expect(accountStatus.toast(copy.toasts.unsuspended)).toBeVisible();
      await expect(accountStatus.statusBanner).toBeHidden({ timeout: Timeouts.FORM_LOAD });
      expect((await agentsApi.get(agent.id)).status?.value).toBe("ACTIVE");
      expect(await accountStatus.offeredActions()).toEqual([actions.suspend]);
    });
  });
});
