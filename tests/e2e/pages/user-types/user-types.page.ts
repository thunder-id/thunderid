// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * User Types Page Object Model
 *
 * Encapsulates the User Types list page and the Create User Type wizard. CreateUserTypePage
 * always starts on an Organization Unit step (see its ORGANIZATION_UNIT branch and useEffect in
 * frontend/packages/configure-user-types/src/pages/CreateUserTypePage.tsx):
 * - Single organization unit (this E2E environment's case): it auto-resolves the only unit and
 *   advances straight to Name, rendering nothing in between. The wizard is effectively two
 *   steps, Name -> Properties.
 * - Multiple organization units: it renders a real picker screen
 *   (OrganizationUnitPickerScreen, keyed by its tree's #organization-unit-picker-screen-tree)
 *   before Name. openCreateWizard() handles this defensively via handleOptionalOuStep(), the
 *   same pattern ApplicationsPage.handleOptionalOuStep() uses for its own optional OU step, even
 *   though it isn't exercised by this environment's single OU.
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
      'div:has(> [data-testid="configure-name"], > [data-testid="configure-properties"])'
    );

    this.nameInput = page.locator('[data-testid="user-type-name-input"]');

    // The attribute library panel. Each row's add button carries the attribute's display
    // name as its aria-label, so one scoped click seeds that schema property.
    this.attributeLibrary = page.getByRole("region", { name: /available properties/i });

    // Name renders "Continue", Properties renders "Create User Type" - the same single
    // contained button, relabelled. Scoping both to wizardContent means submitButton can
    // never resolve to the list page's own "Create User Type" action button.
    this.continueButton = this.wizardContent.getByRole("button", { name: /^continue$/i });
    this.submitButton = this.wizardContent.getByRole("button", { name: /^create user type$/i });
  }

  /**
   * Navigate to the user types list page. openCreateWizard() follows this with its own explicit
   * visibility wait, so there's no need to also wait for network idle here.
   */
  async goto() {
    await this.page.goto(`${this.baseUrl}${ConsoleRoutes.userTypes}`, {
      timeout: Timeouts.PAGE_LOAD,
    });
  }

  /** Navigate to the list and open the Create User Type wizard on its first step. */
  async openCreateWizard() {
    await this.goto();
    await this.createUserTypeButton.first().waitFor({ state: "visible", timeout: Timeouts.ELEMENT_VISIBILITY });
    await this.createUserTypeButton.first().click();
    await this.page.waitForURL(`**${ConsoleRoutes.userTypeCreate}`, { timeout: Timeouts.PAGE_LOAD });
    await this.handleOptionalOuStep();
    await this.stepPanel("name").waitFor({ state: "visible", timeout: Timeouts.FORM_LOAD });
  }

  /**
   * Handle the Organization Unit picker screen that precedes Name when the deployment has
   * multiple organization units. A single-OU deployment (this environment's case) never renders
   * it - CreateUserTypePage auto-resolves the only unit and advances straight past it.
   *
   * Races the picker against the Name panel rather than waiting for the picker alone to time
   * out: on a single-OU deployment the picker never appears, so a bare `waitFor` there would
   * burn its whole timeout on every call before falling through. Whichever renders first
   * settles it.
   */
  private async handleOptionalOuStep(): Promise<void> {
    const ouTree = this.page.locator("#organization-unit-picker-screen-tree");
    await ouTree.or(this.stepPanel("name")).first().waitFor({ state: "visible", timeout: Timeouts.FORM_LOAD });

    if (!(await ouTree.isVisible())) {
      return;
    }

    // OrganizationUnitTreePicker's autoSelectFirst prop selects the first unit itself once the
    // tree's data loads - nothing to click in the tree, just wait for that selection to enable
    // Continue. Unscoped (unlike this.continueButton): this screen renders before wizardContent's
    // step panels exist, so it carries no other "Continue" button to collide with.
    const ouContinueButton = this.page.getByRole("button", { name: /^continue$/i });
    await expect(ouContinueButton).toBeEnabled({ timeout: Timeouts.ELEMENT_VISIBILITY });
    await ouContinueButton.click();
  }

  /** Fill the user type name on step 1 */
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
