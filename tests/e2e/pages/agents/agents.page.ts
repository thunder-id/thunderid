// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Agents Page Object Model
 *
 * Encapsulates the Admin Console's agents list and its Register agent dialog, which creates the
 * agent through the Agents API and then opens it.
 *
 * @example
 * const agents = new AgentsPage(page, baseUrl);
 * await agents.openRegisterDialog();
 * await agents.nameInput.fill('zz_e2e_agent_1');
 * await agents.register();
 */

import { expect, Locator, Page } from "@playwright/test";
import { ConsoleRoutes } from "../../configs/routes/console-routes";
import { Timeouts } from "../../constants/timeouts";
import { BasePage } from "../base.page";

export class AgentsPage extends BasePage {
  readonly baseUrl: string;

  readonly registerAgentButton: Locator;
  readonly dialog: Locator;
  readonly nameInput: Locator;
  readonly agentTypeSelect: Locator;
  readonly descriptionInput: Locator;
  readonly submitButton: Locator;
  readonly cancelButton: Locator;

  constructor(page: Page, baseUrl: string) {
    super(page);
    this.baseUrl = baseUrl;

    this.registerAgentButton = page.getByRole("link", { name: /^register agent$/i });
    this.dialog = page.getByRole("dialog");
    this.nameInput = this.dialog.getByRole("textbox", { name: /^name/i });
    this.agentTypeSelect = this.dialog.getByRole("combobox", { name: /^agent type/i });
    this.descriptionInput = this.dialog.getByRole("textbox", { name: /^description/i });
    this.submitButton = this.dialog.getByRole("button", { name: /^register agent$/i });
    this.cancelButton = this.dialog.getByRole("button", { name: /^cancel$/i });
  }

  async goto(): Promise<void> {
    await this.page.goto(`${this.baseUrl}${ConsoleRoutes.agents}`, { timeout: Timeouts.PAGE_LOAD });
  }

  /** Open the Register agent dialog from the agents list. */
  async openRegisterDialog(): Promise<void> {
    await this.goto();
    await this.registerAgentButton.click();
    await this.page.waitForURL(`**${ConsoleRoutes.agentCreate}`, { timeout: Timeouts.PAGE_LOAD });
    await this.nameInput.waitFor({ state: "visible", timeout: Timeouts.FORM_LOAD });
  }

  /** Submit the dialog. Registering opens the new agent's details page. */
  async register(): Promise<void> {
    await expect(this.submitButton).toBeEnabled({ timeout: Timeouts.ELEMENT_VISIBILITY });
    await this.submitButton.click();
    await expect(this.dialog).toBeHidden({ timeout: Timeouts.FORM_LOAD });
  }
}
