// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Agents - Suspension E2E Tests
 *
 * Covers suspending and unsuspending an agent from its details page through the default
 * administration flows. Unlike a user, an agent is not left locked by an unsuspend, so it returns
 * straight to active.
 *
 * Required environment variables:
 * - BASE_URL: Admin Console base URL
 * - ADMIN_USERNAME / ADMIN_PASSWORD: admin credentials (sign-in + API agent setup/teardown)
 */

import { AgentsApi, expect, test } from "../../fixtures/console";
import { TestTags } from "../../constants/test-tags";
import { Timeouts } from "../../constants/timeouts";
import { UIMessages } from "../../constants/ui-messages";
import { TestDataFactory } from "../../utils/test-data";

const { actions, status } = UIMessages.accountStatus;

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
      await expect(accountStatus.status).toHaveText(status.active);
      await expect(accountStatus.actionButtons).toHaveText([actions.suspend]);
    });

    await test.step("Suspend with a note", async () => {
      await accountStatus.chooseAction(actions.suspend);
      await accountStatus.operatorNoteField.fill(note);
      await accountStatus.confirm(actions.suspend);
      await expect(accountStatus.status).toHaveText(status.suspended, { timeout: Timeouts.FORM_LOAD });
    });

    await test.step("A suspended agent offers Unsuspend only, and the API records the note", async () => {
      await expect(accountStatus.actionButtons).toHaveText([actions.unsuspend]);
      const apiStatus = (await agentsApi.get(agent.id)).status;
      expect(apiStatus?.value).toBe("SUSPENDED");
      expect(apiStatus?.details?.operatorNote).toBe(note);
    });

    await test.step("Unsuspend, and the agent is active again", async () => {
      await accountStatus.chooseAction(actions.unsuspend);
      await accountStatus.confirm(actions.unsuspend);
      await expect(accountStatus.status).toHaveText(status.active, { timeout: Timeouts.FORM_LOAD });
      await expect(accountStatus.actionButtons).toHaveText([actions.suspend]);
      expect((await agentsApi.get(agent.id)).status?.value).toBe("ACTIVE");
    });
  });
});
