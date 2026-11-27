// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Agents - Owner Verification E2E Tests
 *
 * Covers an external owner's verification on the agent's details page: the status the Overview tab
 * shows, and the panel on the Advanced tab that sends the owner a link through the flow named by
 * `flow.adminAgentOwnerVerificationFlow.defaultHandle` (`default-agent-owner-verification-flow`). The
 * shipped flow's Send Email step names no email provider, so the send is refused, which the panel
 * reports, and the owner stays unverified.
 *
 * Required environment variables:
 * - BASE_URL: Console base URL
 * - ADMIN_USERNAME / ADMIN_PASSWORD: admin credentials (console sign-in + API agent setup/teardown)
 */

import { AgentsApi, expect, test } from "../../fixtures/console";
import { Timeouts } from "../../constants/timeouts";
import { UIMessages } from "../../constants/ui-messages";
import { TestDataFactory } from "../../utils/test-data";

test.describe("Agents - Owner Verification", () => {
  const agentName = TestDataFactory.generateUniqueId("zz_e2e_agent_owner");

  test.afterAll(async ({ request }) => {
    const deleted = await new AgentsApi(request).deleteByName(agentName);
    console.log(`Teardown: ${deleted ? "removed" : "found no"} agent ${agentName}`);
  });

  /** TC001: An external owner shows as not verified, and a refused send is reported in the panel */
  test("TC001: Send an external owner a verification link", async ({ agentDetailsPage, agentsApi }) => {
    const ownerEmail = `${agentName}@example.com`;
    const agent = await test.step("Create an agent with an external owner via the Agents API", async () => {
      return agentsApi.create(agentName, { id: ownerEmail, idType: "email", isLocal: false });
    });

    await test.step("The Overview tab shows the owner as not verified", async () => {
      await agentDetailsPage.goto(agent.id);
      await expect(agentDetailsPage.ownerNotVerified).toHaveText(UIMessages.agentOwnerVerification.notVerified, {
        timeout: Timeouts.ELEMENT_VISIBILITY,
      });
    });

    await test.step("The Advanced tab offers to send the owner a link", async () => {
      await agentDetailsPage.clickTab(/^advanced$/i);
      await expect(agentDetailsPage.ownerVerificationPanel).toContainText(ownerEmail, {
        timeout: Timeouts.ELEMENT_VISIBILITY,
      });
      await expect(agentDetailsPage.sendOwnerVerificationButton).toBeEnabled();
    });

    await test.step("A send the flow refuses is reported in the panel", async () => {
      await agentDetailsPage.sendOwnerVerificationButton.click();
      await expect(agentDetailsPage.ownerVerificationPanel.getByRole("alert")).toContainText(
        UIMessages.agentOwnerVerification.noEmailProvider,
        { timeout: Timeouts.FORM_LOAD }
      );
      await expect(agentDetailsPage.sendOwnerVerificationButton).toBeEnabled();
    });

    await test.step("The owner stays unverified", async () => {
      expect((await agentsApi.get(agent.id)).owner).toMatchObject({ id: ownerEmail, verified: false });
    });
  });
});
