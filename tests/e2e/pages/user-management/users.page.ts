// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Users Page Object Model
 *
 * Encapsulates all locators and actions for the User Management page.
 *
 * @example
 * const usersPage = new UsersPage(page, baseUrl);
 * await usersPage.openAddUserWizard("create");
 * await usersPage.fillUserForm({ username: 'test', email: 'test@test.com' });
 * await usersPage.submitForm();
 */

import { Page, Locator, expect } from "@playwright/test";
import { ConsoleRoutes } from "../../configs/routes/console-routes";
import { BasePage } from "../base.page";
import { Timeouts } from "../../constants/timeouts";

export type UserFormData = {
  username: string;
  email: string;
  given_name?: string;
  family_name?: string;
  password?: string;
};

export type AddUserMode = "create" | "invite";

export class UsersPage extends BasePage {
  readonly baseUrl: string;

  // Page Locators
  readonly addUserButton: Locator;

  // Wizard Locators (Step 1: Select User Type)
  readonly userTypeHeading: Locator;
  readonly organizationUnitHeading: Locator;
  readonly onboardingModeHeading: Locator;
  readonly userTypeSelect: Locator;
  readonly continueButton: Locator;
  readonly nextButton: Locator;

  // Invite flow locators
  readonly getInviteLinkButton: Locator;
  readonly copyInviteLinkButton: Locator;

  // The wizard's content column: holds both the current step's panel and that step's
  // action buttons. Scoping submitButton to this avoids the header
  // AppBreadcrumbs (which renders "Add User" / "Create User" crumbs before the content).
  readonly wizardContent: Locator;

  // Form Locators (Step 2: User Details)
  readonly usernameInput: Locator;
  readonly emailInput: Locator;
  readonly givenNameInput: Locator;
  readonly familyNameInput: Locator;
  readonly passwordInput: Locator;
  readonly submitButton: Locator;
  readonly closeButton: Locator;
  readonly formHeading: Locator;

  // Delete flow locators (User Details page: General tab, Danger Zone)
  readonly deleteUserButton: Locator;
  readonly deleteConfirmDialog: Locator;
  readonly deleteConfirmButton: Locator;
  readonly deleteCancelButton: Locator;

  constructor(page: Page, baseUrl: string) {
    super(page);
    this.baseUrl = baseUrl;

    // Add User button
    this.addUserButton = page
      .getByRole("button", { name: /add user/i })
      .or(page.locator('button:has-text("Add User")'))
      .or(page.locator('button:has-text("+ Add User")'))
      .or(page.locator('[data-testid*="add"][data-testid*="user"]'))
      .or(page.locator('a:has-text("Add User")'));

    // Wizard: Step 1 heading ("Select a user type")
    this.userTypeHeading = page.locator("h1, h2, h3, h4, h5, h6").filter({ hasText: /select.*user.*type/i });
    this.organizationUnitHeading = page
      .locator("h1, h2, h3, h4, h5, h6")
      .filter({ hasText: /select an organization unit/i });

    // Wizard: onboarding-mode heading ("Add User") - the step offering the Create/Invite cards
    this.onboardingModeHeading = page.locator("h1, h2, h3, h4, h5, h6").filter({ hasText: /^add user$/i });

    // Wizard: User type dropdown
    this.userTypeSelect = page.getByRole("combobox");

    // Wizard: Continue button
    this.continueButton = page.getByRole("button", { name: /continue/i });
    this.nextButton = page.getByRole("button", { name: /^next$/i });

    // Invite flow: link generation step
    this.getInviteLinkButton = page.getByRole("button", { name: /get invite link/i });
    this.copyInviteLinkButton = page.getByRole("button", { name: /^copy$/i });

    // The onboarding flow renders each interactive step as a <Box component="form">.
    this.wizardContent = page.locator("form");

    // Form fields - support both embedded flow (by id/label) and traditional form (by name)
    this.usernameInput = page
      .locator("input#username")
      .or(page.locator('input[name="username"]'))
      .or(page.getByLabel(/username/i));

    this.emailInput = page.locator("input#email").or(page.locator('input[name="email"]')).or(page.getByLabel(/email/i));

    this.givenNameInput = page
      .locator("input#given_name")
      .or(page.locator('input[name="given_name"]'))
      .or(page.getByLabel(/first.*name|given.*name/i));

    this.familyNameInput = page
      .locator("input#family_name")
      .or(page.locator('input[name="family_name"]'))
      .or(page.getByLabel(/last.*name|family.*name/i));
    this.passwordInput = page
      .locator("input#password")
      .or(page.locator('input[name="password"]'))
      .or(page.getByLabel(/^password$/i));

    // Form buttons. Scoped to wizardContent so this never matches the header breadcrumbs
    // or the invite completion screen's "Add Another User" button.
    this.submitButton = this.wizardContent.getByRole("button", { name: /create.*user|submit|save/i });

    // Both wizards render a header close IconButton; the invite completion screen renders a
    // second Close button. Both route back to the users list, so matching either is fine.
    this.closeButton = page.getByRole("button", { name: /^close$/i });

    // Form heading (Step 2: "Enter user details")
    this.formHeading = page
      .locator("h1, h2, h3, h4, h5, h6")
      .filter({ hasText: /enter.*user.*details|user.*details/i });

    // Danger Zone "Delete" button on the user details page. The details page renders it
    // alone until the dialog opens, so it's unambiguous without extra scoping.
    this.deleteUserButton = page.getByRole("button", { name: /^delete$/i });

    // Delete confirmation dialog, and its buttons scoped to it so they don't clash with
    // the Danger Zone button underneath.
    this.deleteConfirmDialog = page.getByRole("dialog");
    this.deleteConfirmButton = this.deleteConfirmDialog.getByRole("button", { name: /^delete$/i });
    this.deleteCancelButton = this.deleteConfirmDialog.getByRole("button", { name: /^cancel$/i });
  }

  /**
   * Navigate to users management page. Callers follow this with their own explicit visibility
   * wait (clickAddUser(), etc.), so there's no need to also wait for network idle here.
   */
  async goto() {
    await this.page.goto(`${this.baseUrl}${ConsoleRoutes.users}`, {
      timeout: Timeouts.PAGE_LOAD,
    });
  }

  /** Click the Add User button */
  async clickAddUser() {
    await this.addUserButton.first().waitFor({ state: "visible", timeout: Timeouts.ELEMENT_VISIBILITY });
    await this.addUserButton.first().scrollIntoViewIfNeeded();
    await this.addUserButton.first().click();
  }

  /**
   * Navigate to the users list, open the Add User wizard, and drive it up to the mode-specific
   * step: the details form for "create", the email prompt for "invite".
   *
   * The onboarding flow is a single wizard at /users/add: user type -> (org unit) ->
   * Create/Invite choice -> mode-specific steps. There is no separate chooser route any more.
   */
  async openAddUserWizard(mode: AddUserMode) {
    await this.goto();
    await this.clickAddUser();
    await this.page.waitForURL(`**${ConsoleRoutes.users}/add`, { timeout: Timeouts.PAGE_LOAD });
    await this.waitForWizardStep();

    await this.selectUserTypeAndContinue();
    await this.chooseOnboardingMode(mode);
    await this.waitForDetailsStep();
  }

  /** Click the "Create User" or "Invite User" card on the onboarding-mode step */
  async chooseOnboardingMode(mode: AddUserMode) {
    const card = this.page.getByRole("button", { name: mode === "create" ? /^create user/i : /^invite user/i });
    await card.first().waitFor({ state: "visible", timeout: Timeouts.ELEMENT_VISIBILITY });
    await card.first().click();

    await this.waitForStepTransition(this.onboardingModeHeading);
  }

  /**
   * Wait for the wizard to leave the step owned by `heading`: the heading going away is the
   * signal, with a network settle as fallback for steps whose heading text stays put. The
   * trailing pause lets the step transition animation finish.
   */
  private async waitForStepTransition(heading: Locator) {
    try {
      await heading.first().waitFor({ state: "hidden", timeout: Timeouts.FORM_LOAD });
    } catch {
      await this.page.waitForLoadState("networkidle", { timeout: Timeouts.FORM_LOAD }).catch(() => {});
    }
    await this.page.waitForTimeout(300);
  }

  /** Click Next to advance the invite flow past the details step */
  async clickNextButton() {
    await this.nextButton.first().waitFor({ state: "visible", timeout: Timeouts.ELEMENT_VISIBILITY });
    await this.nextButton.first().click();
  }

  /** Click "Get Invite Link" to generate the invite link on the final invite step */
  async clickGetInviteLink() {
    await this.getInviteLinkButton.first().waitFor({ state: "visible", timeout: Timeouts.ELEMENT_VISIBILITY });
    await this.getInviteLinkButton.first().click();
  }

  /** Read the generated invite link's value (rendered next to the Copy button) */
  async getInviteLink(): Promise<string> {
    await this.copyInviteLinkButton.first().waitFor({ state: "visible", timeout: Timeouts.ELEMENT_VISIBILITY });
    const linkValue = await this.copyInviteLinkButton.first().locator("xpath=..").locator("p").first().textContent();
    return (linkValue ?? "").trim();
  }

  /**
   * Complete a (possibly multi-step) embedded registration/accept-invite form,
   * submitting until no required fields remain or maxSteps is reached.
   */
  async completeRegistrationFlow(data: UserFormData, maxSteps: number = 10) {
    const submitButton = this.page.locator('form button[type="submit"]');
    // The flow disables its submit button for exactly as long as a step's POST is in flight
    // (SubmitButtonAdapter's `disabled={isLoading}`), so no disabled submit button means the step
    // settled - either as the next step, or as the completion screen, which has no submit at all.
    const busySubmitButton = this.page.locator('form button[type="submit"][disabled]');

    // The gate's `load` event fires before the SPA validates the invite token and renders the
    // flow's first step, so the loop's instant `isVisible()` gate below would break on iteration 0.
    await this.page.locator("input[required]").first().waitFor({ state: "visible", timeout: Timeouts.PAGE_LOAD });

    for (let step = 0; step < maxSteps; step += 1) {
      const hasMoreFields = await this.page
        .locator("input[required]")
        .first()
        .isVisible()
        .catch(() => false);
      if (!hasMoreFields) break;

      await this.fillUserForm(data);
      await submitButton.first().waitFor({ state: "visible", timeout: Timeouts.FORM_LOAD });
      // waitForLoadState("networkidle") does not work here: the lifecycle event is latched per
      // document, and this flow never navigates, so after the initial load every call returns
      // immediately and the loop races the step's XHR. Wait for the step's POST, then for the
      // re-render it triggers.
      await Promise.all([
        this.page.waitForResponse(
          response => response.url().includes("/flow/execute") && response.request().method() === "POST",
          { timeout: Timeouts.PAGE_LOAD }
        ),
        submitButton.first().click(),
      ]);
      await expect(busySubmitButton).toHaveCount(0, { timeout: Timeouts.FORM_LOAD });
    }
  }

  /** Select the first available user type and advance past the (optional) org unit step */
  private async selectUserTypeAndContinue() {
    // Wait for user type select to be visible
    await this.userTypeSelect.first().waitFor({ state: "visible", timeout: Timeouts.FORM_LOAD });

    // Click the user type select dropdown
    await this.userTypeSelect.first().click();

    // Select the first available option
    const firstOption = this.page.locator('[role="option"]:not([aria-disabled="true"])').first();
    await firstOption.waitFor({ state: "visible", timeout: Timeouts.ELEMENT_VISIBILITY });
    await firstOption.click();

    // Click Continue button
    await this.continueButton.first().waitFor({ state: "visible", timeout: Timeouts.ELEMENT_VISIBILITY });
    await this.clickContinueButton();

    await this.waitForStepTransition(this.userTypeHeading);

    // Handle Organization Unit step if it appears
    const hasOuStep = await this.organizationUnitHeading
      .first()
      .isVisible()
      .catch(() => false);
    if (hasOuStep) {
      await this.continueButton.first().waitFor({ state: "visible", timeout: Timeouts.ELEMENT_VISIBILITY });
      await this.clickContinueButton();

      await this.waitForStepTransition(this.organizationUnitHeading);
    }
  }

  /** Fill the user form (Step 2: User Details) */
  async fillUserForm(data: UserFormData) {
    // One password value per call. Computing it inside the fill loop below would give a
    // password + confirm-password pair two different values milliseconds apart.
    const password = data.password ?? `Test@${Date.now()}`;

    await this.fillIfVisible(this.usernameInput, data.username);
    await this.fillIfVisible(this.emailInput, data.email);
    await this.fillIfVisible(this.givenNameInput, data.given_name);
    await this.fillIfVisible(this.familyNameInput, data.family_name);
    await this.fillIfVisible(this.passwordInput, password);

    // Fill any remaining empty required text/password inputs with generated values
    // (dynamic schema fields that aren't covered by the known field locators, including
    // an injected Confirm Password field)
    const requiredInputs = this.page.locator('input[required]:not([type="checkbox"]):not([type="radio"])');
    const count = await requiredInputs.count();
    for (let i = 0; i < count; i++) {
      const input = requiredInputs.nth(i);
      const currentValue = await input.inputValue();
      if (currentValue) continue;

      const name = (await input.getAttribute("name")) ?? `field_${i}`;
      const type = await input.getAttribute("type");
      // `type` alone isn't enough: a password field with its show/hide toggle on renders as text.
      const isPassword = type === "password" || /password/i.test(name);
      await input.fill(isPassword ? password : `test_${name}_${Date.now()}`);
    }
  }

  /** Submit the form (clicks "Create User" on the last step) */
  async submitForm() {
    await expect(this.submitButton.first()).toBeEnabled({ timeout: Timeouts.ELEMENT_VISIBILITY });
    await this.submitButton.first().click();
  }

  /** Close the wizard. Both wizards route back to the users list from their close button. */
  async closeWizard() {
    await this.closeButton.first().click();
  }

  /**
   * Navigate directly to a user's details page by id.
   *
   * The users list sorts oldest-first and has no search box, so a freshly created test
   * user can land on the last page of the grid. Going straight to its details page by id
   * avoids relying on grid pagination.
   */
  async gotoUserDetails(userId: string) {
    await this.page.goto(`${this.baseUrl}${ConsoleRoutes.userDetails(userId)}`, {
      timeout: Timeouts.PAGE_LOAD,
    });
  }

  /** Open the delete confirmation dialog from the user details page's Danger Zone. */
  async clickDeleteUser() {
    await this.deleteUserButton.waitFor({ state: "visible", timeout: Timeouts.ELEMENT_VISIBILITY });
    await this.deleteUserButton.click();
  }

  /**
   * Confirm deletion in the dialog. The Delete button stays disabled until the usages
   * check (blocking-agent lookup) resolves, so wait for it to become enabled first.
   */
  async confirmDeleteUser() {
    await expect(this.deleteConfirmButton).toBeEnabled({ timeout: Timeouts.ELEMENT_VISIBILITY });
    await this.deleteConfirmButton.click();
  }

  /** Cancel deletion from the dialog, leaving the user intact. */
  async cancelDeleteUser() {
    await this.deleteCancelButton.click();
  }

  private async fillIfVisible(locator: Locator, value?: string) {
    if (!value) return;
    const first = locator.first();
    // Absent fields are skipped: the console wizards and the gate's flow share these helpers and
    // render different subsets. fill() waits for a field that is present to become editable.
    if (await first.isVisible().catch(() => false)) {
      await first.fill(value);
    }
  }

  private async clickContinueButton() {
    await this.continueButton.first().waitFor({ state: "visible", timeout: Timeouts.ELEMENT_VISIBILITY });
    await this.continueButton.first().click();
  }

  /** Wait until a wizard step has actually rendered. */
  private async waitForWizardStep() {
    await this.waitForAnyVisibleLocator(
      [this.userTypeHeading, this.organizationUnitHeading, this.wizardContent, this.formHeading],
      Timeouts.FORM_LOAD
    );
  }

  private async waitForDetailsStep() {
    // Strategy 1: Wait for specific typed text input fields to be visible (form is interactive)
    const typedInputs = this.page.locator(
      'input[type="text"], input[type="email"], input[type="password"], input[type="tel"], textarea'
    );

    try {
      await typedInputs.first().waitFor({ state: "visible", timeout: Timeouts.FORM_LOAD / 2 });
      return;
    } catch {
      // Strategy 2: Wait for ANY input element or form field
      const anyInputs = this.page.locator('input:not([type="hidden"]):not([type="checkbox"]):not([type="radio"])');
      const anyTextfields = this.page.locator('[role="textbox"]');
      const formControl = this.page.locator('[role="group"], .MuiFormControl-root');

      try {
        await Promise.race([
          anyInputs.first().waitFor({ state: "visible", timeout: Timeouts.FORM_LOAD / 2 }),
          anyTextfields.first().waitFor({ state: "visible", timeout: Timeouts.FORM_LOAD / 2 }),
          formControl.first().waitFor({ state: "visible", timeout: Timeouts.FORM_LOAD / 2 }),
        ]);
        return;
      } catch {
        // Strategy 3: Fallback with detailed error reporting
        await this.waitForAnyVisibleLocator(
          [
            this.formHeading,
            this.usernameInput,
            this.emailInput,
            this.givenNameInput,
            this.familyNameInput,
            this.passwordInput,
            anyInputs,
            anyTextfields,
          ],
          Timeouts.FORM_LOAD
        );
      }
    }
  }

  private async waitForAnyVisibleLocator(locators: Locator[], timeout: number) {
    try {
      await Promise.any(locators.map(locator => locator.first().waitFor({ state: "visible", timeout })));
    } catch (error) {
      // Provide debug information about what's actually on the page
      const pageContent = await this.page.content();
      const hasInputs = pageContent.includes("<input");
      const hasFormControl = pageContent.includes("FormControl");
      const hasTextField = pageContent.includes("TextField");

      // Try to find any inputs on the page and log their details
      const allInputs = await this.page.locator("input").all();
      const inputDetails = await Promise.all(
        allInputs.slice(0, 5).map(async input => {
          try {
            const type = await input.getAttribute("type");
            const id = await input.getAttribute("id");
            const name = await input.getAttribute("name");
            const visible = await input.isVisible().catch(() => false);
            return { type, id, name, visible };
          } catch {
            return null;
          }
        })
      );

      const headingContent = await this.page
        .locator("h1, h2, h3, h4, h5, h6")
        .first()
        .textContent()
        .catch(() => "");

      throw new Error(
        `Timed out after ${timeout}ms while waiting for the next visible user-creation step. ` +
          `Debug: hasInputs=${hasInputs}, hasFormControl=${hasFormControl}, hasTextField=${hasTextField}. ` +
          `Found ${allInputs.length} inputs: ${JSON.stringify(inputDetails.filter(Boolean))}. ` +
          `Heading: "${headingContent}". ` +
          `Error: ${error instanceof Error ? error.message : String(error)}`
      );
    }
  }
}
