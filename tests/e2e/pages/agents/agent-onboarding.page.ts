// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Agent Onboarding Page Object Model
 *
 * Encapsulates the Console's agent creation page, which has no built-in steps of its own: it runs
 * the administration flow named by `flow.agentOnboardingFlow.defaultHandle` and renders what that
 * flow returns. The shipped flow is owner -> name -> details -> credentials, which is what the
 * methods below step through.
 *
 * Field locators key off the input's `ref`, because the flow adapters set `id={ref}` on the
 * rendered control (see SelectAdapter/TextInputAdapter in frontend/packages/design). That stays
 * stable across relabelling in a way a visible label does not.
 *
 * @example
 * const onboarding = new AgentOnboardingPage(page, baseUrl);
 * await onboarding.open();
 * await onboarding.submitOwnerStep();
 * await onboarding.submitNameStep('zz_e2e_agent_1');
 * await onboarding.submitDetailsStep({ model: 'claude-opus-5' });
 */

import { expect, Locator, Page } from "@playwright/test";
import { ConsoleRoutes } from "../../configs/routes/console-routes";
import { Timeouts } from "../../constants/timeouts";
import { BasePage } from "../base.page";

export class AgentOnboardingPage extends BasePage {
  readonly baseUrl: string;

  // List page
  readonly addAgentButton: Locator;

  // Flow-rendered controls, one per input the shipped flow declares.
  readonly ownerSelect: Locator;
  readonly nameInput: Locator;
  readonly modelProviderSelect: Locator;
  readonly modelInput: Locator;
  readonly functionSelect: Locator;

  readonly closeButton: Locator;

  constructor(page: Page, baseUrl: string) {
    super(page);
    this.baseUrl = baseUrl;

    this.addAgentButton = page.getByTestId("agent-add-button");

    this.ownerSelect = page.locator("#owner");
    this.nameInput = page.locator("#name");
    this.modelProviderSelect = page.locator("#modelProvider");
    this.modelInput = page.locator("#model");
    this.functionSelect = page.locator("#function");

    this.closeButton = page.getByRole("button", { name: /^close$/i });
  }

  /** Navigate to the agent list and open the onboarding page from it. */
  async open() {
    await this.page.goto(`${this.baseUrl}${ConsoleRoutes.agents}`, { timeout: Timeouts.PAGE_LOAD });
    await this.addAgentButton.waitFor({ state: "visible", timeout: Timeouts.ELEMENT_VISIBILITY });
    await this.addAgentButton.click();
    await this.page.waitForURL(`**${ConsoleRoutes.agentCreate}`, { timeout: Timeouts.PAGE_LOAD });
  }

  /** The heading the current flow step renders, once it has replaced the previous one. */
  async expectStep(heading: RegExp) {
    await expect(this.page.getByRole("heading", { name: heading })).toBeVisible({
      timeout: Timeouts.FORM_LOAD,
    });
  }

  /** Click the step's single submit action, identified by its label. */
  async submitStep(label: RegExp) {
    await this.page.getByRole("button", { name: label }).click();
  }

  /**
   * Choose an option in one of the flow's SELECT inputs. MUI renders a listbox rather than a
   * native select, so the control is opened and the option clicked.
   */
  async chooseOption(select: Locator, option: RegExp) {
    await select.click();
    await this.page.getByRole("option", { name: option }).click();
  }

  /** The option labels a SELECT is offering, with the disabled placeholder row dropped. */
  async optionLabels(select: Locator): Promise<string[]> {
    await select.click();
    const listbox = this.page.getByRole("listbox");
    await listbox.waitFor({ state: "visible", timeout: Timeouts.ELEMENT_VISIBILITY });
    const labels = await listbox.getByRole("option").allInnerTexts();
    await this.page.keyboard.press("Escape");

    return labels.filter(label => !/^select /i.test(label.trim()));
  }

  /** Leave the owner unset, so the agent is owned by the signed-in administrator. */
  async submitOwnerStep() {
    await this.expectStep(/select an owner/i);
    await this.submitStep(/^continue$/i);
  }

  async submitNameStep(name: string) {
    await this.expectStep(/name your agent/i);
    await this.nameInput.fill(name);
    await this.submitStep(/^continue$/i);
  }

  /** Fill the schema-driven detail step. Every field on it is optional. */
  async submitDetailsStep({ modelProvider, model }: { modelProvider?: RegExp; model?: string }) {
    await this.expectStep(/agent details/i);
    if (modelProvider) await this.chooseOption(this.modelProviderSelect, modelProvider);
    if (model) await this.modelInput.fill(model);
    await this.submitStep(/^create agent$/i);
  }

  /**
   * The value shown next to a plain credential's label on the completion screen.
   *
   * COPYABLE_TEXT renders a label paragraph followed by a box holding the value, and carries no
   * test id, so the value is reached structurally from its label.
   */
  credentialValue(label: string): Locator {
    return this.page.locator(
      `xpath=//p[normalize-space(text())=${JSON.stringify(label)}]/following-sibling::div[1]//p[1]`
    );
  }

  /**
   * The input holding a credential the flow marks as masked.
   *
   * A masked credential is a read-only field rather than text, so it is copyable without being
   * readable. Its value lives on the input, and the type says whether it is currently revealed.
   */
  maskedCredential(label: string): Locator {
    return this.page.locator(
      `xpath=//*[normalize-space(text())=${JSON.stringify(label)}]/following-sibling::div[1]//input`
    );
  }

  /** Reveals a masked credential, the way an administrator would to read it on screen. */
  async revealSecret() {
    await this.page.getByRole("button", { name: /^show$/i }).click();
  }
}
