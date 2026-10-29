// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Agent Details Page Object Model
 *
 * Encapsulates an agent's details page: its lifecycle controls, through the shared
 * AccountStatusControls component, and its delete action.
 *
 * @example
 * const agentDetails = new AgentDetailsPage(page, baseUrl);
 * await agentDetails.goto(agentId);
 * await agentDetails.accountStatus.chooseAction("Suspend");
 */

import { Locator, Page } from "@playwright/test";
import { ConsoleRoutes } from "../../configs/routes/console-routes";
import { Timeouts } from "../../constants/timeouts";
import { UIMessages } from "../../constants/ui-messages";
import { BasePage } from "../base.page";
import { AccountStatusControls } from "../components/account-status-controls";

export class AgentDetailsPage extends BasePage {
  readonly baseUrl: string;

  readonly accountStatus: AccountStatusControls;
  readonly deleteAgentButton: Locator;
  /** The inline confirmation the delete action asks for. */
  readonly deleteConfirmation: Locator;

  constructor(page: Page, baseUrl: string) {
    super(page);
    this.baseUrl = baseUrl;
    this.accountStatus = new AccountStatusControls(page);
    const main = page.getByRole("main");
    this.deleteAgentButton = main.getByRole("button", { name: UIMessages.deletion.agent, exact: true });
    this.deleteConfirmation = main.getByRole("group").filter({ hasText: /^Delete .+\?/ });
  }

  async confirmDelete(): Promise<void> {
    await this.deleteConfirmation.getByRole("button", { name: UIMessages.deletion.confirm, exact: true }).click();
  }

  /** Navigate straight to an agent's details page by id and wait for it to load. */
  async goto(agentId: string): Promise<void> {
    await this.page.goto(`${this.baseUrl}${ConsoleRoutes.agentDetails(agentId)}`, {
      timeout: Timeouts.PAGE_LOAD,
    });
    await this.deleteAgentButton.waitFor({ state: "visible", timeout: Timeouts.PAGE_LOAD });
  }
}
