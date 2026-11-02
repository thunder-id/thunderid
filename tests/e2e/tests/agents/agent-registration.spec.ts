// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Agents - Registration E2E Tests
 *
 * Covers the Admin Console's Register agent dialog, which creates the agent through the Agents API
 * and opens it, cancelling the dialog, and deleting an agent from its details page.
 *
 * Required environment variables:
 * - BASE_URL: Admin Console base URL
 * - ADMIN_USERNAME / ADMIN_PASSWORD: admin credentials (sign-in + API teardown)
 */

import { AgentsApi, ConsoleRoutes, expect, test } from "../../fixtures/console";
import { Timeouts } from "../../constants/timeouts";
import { UIMessages } from "../../constants/ui-messages";
import { TestDataFactory } from "../../utils/test-data";

test.describe("Agents - Registration", () => {
  // Generated at describe scope so afterAll can see the names. Each browser project runs this file
  // in its own worker, so concurrent chromium/firefox/webkit runs get distinct names.
  const registeredAgent = TestDataFactory.generateUniqueId("zz_e2e_agent");
  const abandonedAgent = TestDataFactory.generateUniqueId("zz_e2e_agent_abandoned");
  const deletedAgent = TestDataFactory.generateUniqueId("zz_e2e_agent_deleted");

  test.afterAll(async ({ request }) => {
    const agentsApi = new AgentsApi(request);
    for (const name of [registeredAgent, abandonedAgent, deletedAgent]) {
      if (await agentsApi.deleteByName(name)) console.log(`Teardown: removed agent ${name}`);
    }
  });

  /** TC001: Register an agent and land on its details page */
  test("TC001: Register an agent", async ({ agentsPage, agentsApi }) => {
    await test.step("Open Register agent from the agents list", async () => {
      await agentsPage.openRegisterDialog();
    });

    await test.step("Name the agent and register it", async () => {
      await agentsPage.nameInput.fill(registeredAgent);
      await agentsPage.register();
    });

    await test.step("The Agents API has the agent, and its details page opens", async () => {
      const agent = await agentsApi.findByName(registeredAgent);
      expect(agent, `agent ${registeredAgent} should exist`).toBeDefined();
      await agentsPage.page.waitForURL(`**${ConsoleRoutes.agentDetails(agent!.id)}`, { timeout: Timeouts.PAGE_LOAD });
    });
  });

  /** TC002: Cancelling the dialog returns to the list without registering anything */
  test("TC002: Cancel registering an agent", async ({ agentsPage, agentsApi }) => {
    await agentsPage.openRegisterDialog();
    await agentsPage.nameInput.fill(abandonedAgent);
    await agentsPage.cancelButton.click();

    await agentsPage.page.waitForURL(`**${ConsoleRoutes.agents}`, { timeout: Timeouts.PAGE_LOAD });
    expect(await agentsApi.findByName(abandonedAgent)).toBeUndefined();
  });

  /** TC003: Delete an agent from its details page */
  test("TC003: Delete an agent from the details page", async ({ agentDetailsPage, agentsApi }) => {
    const agent = await agentsApi.create(deletedAgent);

    await agentDetailsPage.goto(agent.id);
    await agentDetailsPage.deleteAgentButton.click();
    await expect(agentDetailsPage.deleteConfirmation).toContainText(UIMessages.deletion.question(deletedAgent));
    await agentDetailsPage.confirmDelete();

    await agentDetailsPage.page.waitForURL(`**${ConsoleRoutes.agents}`, { timeout: Timeouts.PAGE_LOAD });
    expect(await agentsApi.findByName(deletedAgent)).toBeUndefined();
  });
});
