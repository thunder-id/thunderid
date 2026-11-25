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

import { Page } from "@playwright/test";
import { ConsoleRoutes } from "../../configs/routes/console-routes";
import { Timeouts } from "../../constants/timeouts";
import { BasePage } from "../base.page";
import { AccountStatusControls } from "../components/account-status-controls";

export class AgentDetailsPage extends BasePage {
  readonly baseUrl: string;

  readonly accountStatus: AccountStatusControls;

  constructor(page: Page, baseUrl: string) {
    super(page);
    this.baseUrl = baseUrl;
    this.accountStatus = new AccountStatusControls(page);
  }

  /** Navigate straight to an agent's details page by id and wait for its tabs. */
  async goto(agentId: string): Promise<void> {
    await this.page.goto(`${this.baseUrl}${ConsoleRoutes.agentDetails(agentId)}`, {
      timeout: Timeouts.PAGE_LOAD,
    });
    await this.page.getByRole("tablist").first().waitFor({ state: "visible", timeout: Timeouts.PAGE_LOAD });
  }
}
