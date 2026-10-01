// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Agent Onboarding E2E Tests
 *
 * Covers creating an agent from the Console. The page runs the administration flow named by
 * `flow.agentOnboardingFlow.defaultHandle` (`default-agent-onboarding-flow`, seeded in
 * backend/cmd/server/bootstrap/01-default-resources.yaml) and renders what the server returns,
 * so this is the only place the Console, the flow engine and the provisioning executor are
 * exercised together.
 *
 * Required environment variables:
 * - BASE_URL: Console base URL
 * - ADMIN_USERNAME / ADMIN_PASSWORD: admin credentials (console sign-in + API teardown)
 *
 * Optional:
 * - SERVER_URL: backend base URL (default https://localhost:8090)
 */

import { AgentsApi, ConsoleRoutes, expect, test } from "../../fixtures/console";
import { Timeouts } from "../../constants/timeouts";
import { TestDataFactory } from "../../utils/test-data";

test.describe("Agents - Onboarding Flow", () => {
  // Generated at describe scope so afterAll can see the names. Each browser project runs this
  // file in its own worker, so concurrent chromium/firefox/webkit runs get distinct names and
  // cannot collide.
  const createdAgent = TestDataFactory.generateUniqueId("zz_e2e_agent");
  const cancelledAgent = TestDataFactory.generateUniqueId("zz_e2e_agent_cancelled");

  test.afterAll(async ({ request }) => {
    // beforeAll/afterAll cannot take custom test-scoped fixtures, so construct the shared helper
    // directly here - same class the agentsApi fixture uses inside the tests below.
    const agentsApi = new AgentsApi(request);
    for (const name of [createdAgent, cancelledAgent]) {
      const deleted = await agentsApi.deleteByName(name);
      if (deleted) console.log(`Teardown: removed agent ${name}`);
    }
  });

  /** TC001: Verify an agent can be created by running the onboarding flow end to end */
  test("TC001: Create an agent through the onboarding flow", async ({ agentOnboardingPage, agentsApi }) => {
    await test.step("Open the onboarding page from the agent list", async () => {
      await agentOnboardingPage.open();
    });

    await test.step("Leave the owner unset", async () => {
      await agentOnboardingPage.submitOwnerStep();
    });

    await test.step("Name the agent", async () => {
      await agentOnboardingPage.submitNameStep(createdAgent);
    });

    await test.step("Fill in the schema-driven details", async () => {
      await agentOnboardingPage.submitDetailsStep({ model: "claude-opus-5", modelProvider: /^anthropic$/i });
    });

    await test.step("Read the generated credentials", async () => {
      await agentOnboardingPage.expectStep(/agent created successfully/i);

      for (const label of ["Agent ID", "Client ID"]) {
        await expect(agentOnboardingPage.credentialValue(label)).not.toBeEmpty({
          timeout: Timeouts.ELEMENT_VISIBILITY,
        });
      }

      // The client secret is returned on create and never again, so this screen is the only place
      // it can be read. It is masked rather than printed, so it is copyable without sitting on
      // screen in the clear.
      const secret = agentOnboardingPage.maskedCredential("Client Secret");

      await expect(secret).toHaveAttribute("type", "password", { timeout: Timeouts.ELEMENT_VISIBILITY });
      await expect(secret).not.toHaveValue("");

      await agentOnboardingPage.revealSecret();
      await expect(secret).toHaveAttribute("type", "text");
    });

    await test.step("Verify the agent, its client id and its attributes via the Agents API", async () => {
      const listed = await agentsApi.findByName(createdAgent);
      expect(listed, `agent ${createdAgent} should exist`).toBeDefined();

      const agent = await agentsApi.get(listed!.id);

      // A placeholder or stale value would still satisfy the not-empty check above.
      await expect(agentOnboardingPage.credentialValue("Client ID")).toHaveText(String(agent.clientId));

      // Proves the provisioning node wrote the attributes the schema-driven step collected.
      expect(agent.attributes).toMatchObject({ model: "claude-opus-5", modelProvider: "anthropic" });
    });
  });

  /**
   * TC002: Verify the schema's enum values reach the flow's own SELECT inputs
   *
   * The dropdowns are declared in the flow with `options: []`; the provisioning executor supplies
   * the values from the agent type's schema and the prompt node merges them in by identifier. A
   * broken merge leaves the step rendering and submitting normally, just with empty dropdowns.
   */
  test("TC002: Offer the agent type's enum values on the detail step", async ({ agentOnboardingPage }) => {
    await agentOnboardingPage.open();
    await agentOnboardingPage.submitOwnerStep();
    await agentOnboardingPage.submitNameStep(cancelledAgent);
    await agentOnboardingPage.expectStep(/agent details/i);

    const providers = await agentOnboardingPage.optionLabels(agentOnboardingPage.modelProviderSelect);

    expect(providers).toEqual(["openai", "anthropic", "gemini", "mistral", "custom"]);
    expect(await agentOnboardingPage.optionLabels(agentOnboardingPage.functionSelect)).not.toHaveLength(0);
  });

  /** TC003: Verify closing the wizard returns to the agent list without creating anything */
  test("TC003: Close the onboarding page without creating an agent", async ({ agentOnboardingPage, agentsApi }) => {
    const abandoned = TestDataFactory.generateUniqueId("zz_e2e_agent_abandoned");

    await agentOnboardingPage.open();
    await agentOnboardingPage.submitOwnerStep();
    await agentOnboardingPage.expectStep(/name your agent/i);
    await agentOnboardingPage.nameInput.fill(abandoned);

    await agentOnboardingPage.closeButton.click();
    await agentOnboardingPage.page.waitForURL(`**${ConsoleRoutes.agents}`, { timeout: Timeouts.PAGE_LOAD });

    // The provisioning node runs after the name step, so abandoning here must leave no record.
    expect(await agentsApi.findByName(abandoned)).toBeUndefined();
  });
});
