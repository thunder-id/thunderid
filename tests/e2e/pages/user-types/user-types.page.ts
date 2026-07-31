// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * User Types Page Object Model
 *
 * Encapsulates the User Types list page and the Create User Type wizard
 * (Details -> Properties). A deployment with more than one organization unit prepends an
 * organization-unit step; the E2E deployment has a single unit, so the wizard starts on Details.
 *
 * @example
 * const userTypesPage = new UserTypesPage(page, baseUrl);
 * await userTypesPage.openCreateWizard();
 * await userTypesPage.fillName('zz_e2e_user_type_1');
 * await userTypesPage.continueTo('properties');
 * await userTypesPage.addLibraryProperty('Email');
 * await userTypesPage.submit();
 */

import { Page, Locator, expect } from "@playwright/test";
import { ConsoleRoutes } from "../../configs/routes/console-routes";
import { BasePage } from "../base.page";
import { Timeouts } from "../../constants/timeouts";

type WizardStep = "name" | "properties";

export class UserTypesPage extends BasePage {
  readonly baseUrl: string;

  // List page
  readonly createUserTypeButton: Locator;

  // The wizard's content column: holds the active step's panel and that step's action
  // buttons as siblings. Scoping the buttons to it keeps them off the header breadcrumbs
  // and, critically, off the list page's own "Create User Type" action button.
  readonly wizardContent: Locator;

  // Wizard locators
  readonly nameInput: Locator;
  readonly attributeLibrary: Locator;
  readonly continueButton: Locator;
  readonly submitButton: Locator;

  constructor(page: Page, baseUrl: string) {
    super(page);
    this.baseUrl = baseUrl;

    this.createUserTypeButton = page.getByRole("button", { name: /^create user type$/i });

    this.wizardContent = page.locator(
      'div:has(> [data-testid="configure-name"], > [data-testid="configure-properties"])',
    );

    this.nameInput = page.locator('[data-testid="user-type-name-input"]');

    // The attribute library panel. Each row's add button carries the attribute's display
    // name as its aria-label, so one scoped click seeds that schema property.
    this.attributeLibrary = page.getByRole("region", { name: /available properties/i });

    // The Details step renders "Continue", the Properties step renders "Create User Type" - the
    // same single contained button, relabelled. Scoping both to wizardContent means submitButton
    // can never resolve to the list page's own "Create User Type" action button.
    this.continueButton = this.wizardContent.getByRole("button", { name: /^continue$/i });
    this.submitButton = this.wizardContent.getByRole("button", { name: /^create user type$/i });
  }

  /** Navigate to the user types list page */
  async goto() {
    await this.page.goto(`${this.baseUrl}${ConsoleRoutes.userTypes}`, {
      waitUntil: "networkidle",
      timeout: Timeouts.PAGE_LOAD,
    });
  }

  /** Navigate to the list and open the Create User Type wizard on its first step. */
  async openCreateWizard() {
    await this.goto();
    await this.createUserTypeButton.first().waitFor({ state: "visible", timeout: Timeouts.ELEMENT_VISIBILITY });
    await this.createUserTypeButton.first().click();
    await this.page.waitForURL(`**${ConsoleRoutes.userTypeCreate}`, { timeout: Timeouts.PAGE_LOAD });
    await this.stepPanel("name").waitFor({ state: "visible", timeout: Timeouts.FORM_LOAD });
  }

  /** Fill the user type name on the Details step */
  async fillName(name: string) {
    await this.nameInput.waitFor({ state: "visible", timeout: Timeouts.ELEMENT_VISIBILITY });
    await this.nameInput.fill(name);
  }

  /**
   * Advance to the next wizard step.
   *
   * Waits for the destination panel rather than returning on the click: the Continue button
   * is one persistent element relabelled per step, so returning early risks clicking it again
   * before the next step's readiness effect has run.
   */
  async continueTo(step: Exclude<WizardStep, "name">) {
    await expect(this.continueButton).toBeEnabled({ timeout: Timeouts.ELEMENT_VISIBILITY });
    await this.continueButton.click();
    await this.stepPanel(step).waitFor({ state: "visible", timeout: Timeouts.FORM_LOAD });
  }

  /** Add a predefined property to the schema by its display name, e.g. "Email". */
  async addLibraryProperty(displayName: string) {
    const addButton = this.attributeLibrary.getByRole("button", { name: displayName, exact: true });
    await addButton.waitFor({ state: "visible", timeout: Timeouts.ELEMENT_VISIBILITY });
    await addButton.click();
  }

  /** Submit the wizard from the Properties step. */
  async submit() {
    await expect(this.submitButton).toBeEnabled({ timeout: Timeouts.ELEMENT_VISIBILITY });
    await this.submitButton.click();
  }

  private stepPanel(step: WizardStep): Locator {
    return this.page.locator(`[data-testid="configure-${step}"]`);
  }
}
