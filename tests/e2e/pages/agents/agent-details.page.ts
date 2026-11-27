// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Agent Details Page Object Model
 *
 * Encapsulates an agent's details page. Covers the header's lifecycle controls (the Actions menu,
 * its dialogs and the status banner) through the shared AccountStatusControls component.
 *
 * @example
 * const agentDetails = new AgentDetailsPage(page, baseUrl);
 * await agentDetails.goto(agentId);
 * await agentDetails.accountStatus.chooseAction("Suspend");
 */

import { Locator, Page } from "@playwright/test";
import { ConsoleRoutes } from "../../configs/routes/console-routes";
import { Timeouts } from "../../constants/timeouts";
import { BasePage } from "../base.page";
import { AccountStatusControls } from "../components/account-status-controls";

export class AgentDetailsPage extends BasePage {
  readonly baseUrl: string;

  readonly accountStatus: AccountStatusControls;

  // An external owner's verification: its status, and the panel on the Advanced tab that sends the owner a link.
  readonly ownerNotVerified: Locator;
  readonly ownerVerificationPanel: Locator;
  readonly sendOwnerVerificationButton: Locator;

  constructor(page: Page, baseUrl: string) {
    super(page);
    this.baseUrl = baseUrl;
    this.accountStatus = new AccountStatusControls(page);
    this.ownerNotVerified = page.getByTestId("agent-owner-not-verified");
    this.ownerVerificationPanel = page.getByTestId("agent-owner-verification");
    this.sendOwnerVerificationButton = page.getByTestId("agent-owner-send-verification");
  }

  /** Open one of the agent's tabs by its name. */
  async clickTab(name: RegExp): Promise<void> {
    await this.page.getByRole("tab", { name }).click();
  }

  /** Navigate straight to an agent's details page by id and wait for its tabs. */
  async goto(agentId: string): Promise<void> {
    await this.page.goto(`${this.baseUrl}${ConsoleRoutes.agentDetails(agentId)}`, {
      timeout: Timeouts.PAGE_LOAD,
    });
    await this.page.getByRole("tablist").first().waitFor({ state: "visible", timeout: Timeouts.PAGE_LOAD });
  }
}
