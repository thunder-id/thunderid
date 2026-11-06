// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Agents Page Object Model
 *
 * Encapsulates the Admin Console's agents list and its Register agent dialog, which runs the agent
 * onboarding flow: an agent type and organization unit step when the flow asks for one, then an
 * optional owner, then the details form whose Create Agent action provisions the agent.
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
  readonly continueButton: Locator;
  readonly submitButton: Locator;
  readonly closeButton: Locator;

  constructor(page: Page, baseUrl: string) {
    super(page);
    this.baseUrl = baseUrl;

    this.registerAgentButton = page.getByRole("link", { name: /^register agent$/i });
    this.dialog = page.getByRole("dialog");
    this.nameInput = this.dialog.getByRole("textbox", { name: /^name/i });
    this.agentTypeSelect = this.dialog.getByRole("combobox", { name: /^agent type/i });
    this.descriptionInput = this.dialog.getByRole("textbox", { name: /^description/i });
    this.continueButton = this.dialog.getByRole("button", { name: /^continue$/i });
    this.submitButton = this.dialog.getByRole("button", { name: /^create agent$/i });
    this.closeButton = this.dialog.getByRole("button", { name: /^close$/i });
  }

  async goto(): Promise<void> {
    await this.page.goto(`${this.baseUrl}${ConsoleRoutes.agents}`, { timeout: Timeouts.PAGE_LOAD });
  }

  /** Open the Register agent dialog from the agents list. */
  async openRegisterDialog(): Promise<void> {
    await this.goto();
    await this.registerAgentButton.click();
    await this.page.waitForURL(`**${ConsoleRoutes.agentCreate}`, { timeout: Timeouts.PAGE_LOAD });
    await this.advanceToDetails();
  }

  /**
   * Step through the flow's earlier prompts, which keep what they preselect, until the details
   * form shows. A step the flow can resolve on its own never renders.
   */
  async advanceToDetails(): Promise<void> {
    const maxSteps = 4;
    for (let step = 0; step < maxSteps; step++) {
      await this.nameInput.or(this.continueButton).first().waitFor({ state: "visible", timeout: Timeouts.FORM_LOAD });
      if (await this.nameInput.isVisible()) return;
      await expect(this.continueButton).toBeEnabled({ timeout: Timeouts.ELEMENT_VISIBILITY });
      await this.continueButton.click();
    }
    await this.nameInput.waitFor({ state: "visible", timeout: Timeouts.FORM_LOAD });
  }

  /** Submit the details form, then leave the completed flow, which returns to the agents list. */
  async register(): Promise<void> {
    await expect(this.submitButton).toBeEnabled({ timeout: Timeouts.ELEMENT_VISIBILITY });
    await this.submitButton.click();
    await this.closeButton.click({ timeout: Timeouts.FORM_LOAD });
    await expect(this.dialog).toBeHidden({ timeout: Timeouts.FORM_LOAD });
  }

  /** Leave the dialog without registering. The flow has no cancel control, so Escape closes it. */
  async cancel(): Promise<void> {
    await this.page.keyboard.press("Escape");
  }
}
