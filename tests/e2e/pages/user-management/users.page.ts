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

  // Staff onboarding flow locators (email-based invite: "Which staff role?" -> "Invitee email")
  readonly sendInvitationButton: Locator;
  readonly invitationSentHeading: Locator;

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

  // Edit flow locators (User Details page: Attributes tab + page-level unsaved-changes bar)
  // Only one tabpanel is ever un-hidden at a time (MUI toggles a `hidden` attribute per panel),
  // so this always resolves to whichever tab is currently active.
  readonly activeTabPanel: Locator;
  readonly resetChangesButton: Locator;
  readonly saveChangesButton: Locator;

  // Credentials tab: "Reset Password" flow (User Details page: Credentials tab)
  readonly resetPasswordButton: Locator;
  readonly credentialResetDialog: Locator;
  readonly newCredentialValueInput: Locator;
  readonly confirmCredentialValueInput: Locator;
  readonly confirmResetPasswordButton: Locator;
  readonly cancelResetPasswordButton: Locator;

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

    // Staff onboarding flow: the "Invitee email" step submits via a "Send invitation" button
    // (rather than "Get Invite Link") and, once the invite email is sent, shows a confirmation
    // heading instead of a copyable link.
    this.sendInvitationButton = page.getByRole("button", { name: /send invitation/i });
    this.invitationSentHeading = page.locator("h1, h2, h3, h4, h5, h6").filter({ hasText: /invitation sent/i });

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

    // Form buttons - support multiple button naming conventions
    // The actual button label comes from the embedded flow, so we support common patterns.
    // Scoped to wizardContent: the unscoped regex also matches the header breadcrumb's
    // "Add User" crumb (which resets the wizard back to step 1), and that crumb sits earlier
    // in the DOM than the step's own submit button, so an unscoped .first() would click it.
    this.submitButton = this.wizardContent
      .getByRole("button", { name: /create.*user|add.*user|submit|save|finish|next|continue|confirm/i })
      .or(
        this.wizardContent
          .locator('button[size="large"]:not(:has-text("Cancel"))')
          .or(this.wizardContent.locator('button:not(:has-text("Cancel")):not(:has-text("Close"))').nth(-1))
      );
    this.closeButton = page.getByRole("button", { name: /cancel|close/i });

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

    this.activeTabPanel = page.locator('[role="tabpanel"]:not([hidden])');

    // The page-level bar only exists in the DOM while a field is dirty, so exact text avoids
    // matching the Credentials tab's per-field "Reset {label}" buttons (e.g. "Reset Password").
    this.resetChangesButton = page.getByRole("button", { name: "Reset", exact: true });
    this.saveChangesButton = page.getByRole("button", { name: "Save", exact: true });

    // Scoped to the active tab panel so it only matches the Credentials tab's trigger button,
    // not the reset dialog's own submit button underneath (both are named "Reset Password").
    this.resetPasswordButton = this.activeTabPanel.getByRole("button", { name: /^reset password$/i });
    this.credentialResetDialog = page.getByRole("dialog");
    this.newCredentialValueInput = this.credentialResetDialog.locator("#credential-new-password");
    this.confirmCredentialValueInput = this.credentialResetDialog.locator("#credential-confirm-password");
    this.confirmResetPasswordButton = this.credentialResetDialog.getByRole("button", { name: /^reset password$/i });
    this.cancelResetPasswordButton = this.credentialResetDialog.getByRole("button", { name: /^cancel$/i });
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
    // No DOM/network signal marks the transition animation finishing, so this is a deliberate exception.
    // eslint-disable-next-line playwright/no-wait-for-timeout
    await this.page.waitForTimeout(Timeouts.STEP_TRANSITION);
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

  /** Click one of the staff-onboarding flow's "Which staff role?" action buttons (e.g. "Support"). */
  async selectStaffRole(role: string) {
    const roleButton = this.page.getByRole("button", { name: role, exact: true });
    await roleButton.waitFor({ state: "visible", timeout: Timeouts.ELEMENT_VISIBILITY });
    await roleButton.click();
  }

  /** Fill the invitee's email on the staff-onboarding flow's "Invitee email" step. */
  async fillInviteeEmail(email: string) {
    await this.emailInput.first().waitFor({ state: "visible", timeout: Timeouts.DEFAULT_ACTION });
    await this.emailInput.first().fill(email);
  }

  /** Click "Send invitation" on the staff-onboarding flow's "Invitee email" step. */
  async clickSendInvitation() {
    await this.sendInvitationButton.first().waitFor({ state: "visible", timeout: Timeouts.DEFAULT_ACTION });
    await this.sendInvitationButton.first().click();
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

    const stillIncomplete = await this.page
      .locator("input[required]")
      .first()
      .isVisible()
      .catch(() => false);
    if (stillIncomplete) {
      throw new Error(`completeRegistrationFlow did not finish within ${maxSteps} steps`);
    }
  }

  /**
   * Select a user type and advance past the (optional) org unit step. Picks the first available
   * option when `userTypeName` is omitted; matches by exact visible text otherwise (e.g. "Staff",
   * for the Wayfinder staff-onboarding flow).
   */
  async selectUserTypeAndContinue(userTypeName?: string) {
    // Wait for user type select to be visible
    await this.userTypeSelect.first().waitFor({ state: "visible", timeout: Timeouts.FORM_LOAD });

    // Click the user type select dropdown
    await this.userTypeSelect.first().click();

    const option = userTypeName
      ? this.page.getByRole("option", { name: userTypeName, exact: true })
      : this.page.locator('[role="option"]:not([aria-disabled="true"])').first();
    await option.waitFor({ state: "visible", timeout: Timeouts.ELEMENT_VISIBILITY });
    await option.click();

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
    // an injected Confirm Password field). Scoped to wizardContent and to visible inputs only,
    // same reasoning as submitButton above - a hidden native input backing a component like MUI
    // Select can also carry `required`, and isn't something this loop should try to fill.
    const requiredInputs = this.wizardContent.locator('input[required]:not([type="checkbox"]):not([type="radio"])');
    const count = await requiredInputs.count();
    for (let i = 0; i < count; i++) {
      const input = requiredInputs.nth(i);
      if (!(await input.isVisible())) continue;

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
    const submitBtn = this.submitButton.first();

    // Wait for button to be visible first
    try {
      await submitBtn.waitFor({ state: "visible", timeout: Timeouts.ELEMENT_VISIBILITY });
    } catch (error) {
      // If button not found by standard selectors, try to find any large contained button
      const allButtons = this.page.locator('button[size="large"]');
      const count = await allButtons.count();

      if (count > 0) {
        // Try the last button (usually submit in wizards)
        await allButtons.last().waitFor({ state: "visible", timeout: Timeouts.ELEMENT_VISIBILITY });
        await allButtons.last().click();
        return;
      }

      throw new Error(`Could not find submit button. ${error}`);
    }

    // Check if enabled and click
    await expect(submitBtn).toBeEnabled({ timeout: Timeouts.ELEMENT_VISIBILITY });
    await submitBtn.click();
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

  /** Click a tab on the edit page by its visible label */
  async clickTab(tabName: string): Promise<void> {
    const tab = this.page.getByRole("tab", { name: new RegExp(tabName, "i") });
    await tab.waitFor({ state: "visible", timeout: Timeouts.ELEMENT_VISIBILITY });
    await tab.click();
  }

  /** Every editable schema attribute input on the currently active tab panel (Attributes tab). */
  private get attributeInputs(): Locator {
    return this.activeTabPanel.locator('input[type="text"], input[type="number"]');
  }

  /** Read the current value of every attribute field on the Attributes tab, keyed by field id. */
  async readAttributeFieldValues(): Promise<Record<string, string>> {
    const pairs = await this.attributeInputs.evaluateAll(inputs =>
      (inputs as HTMLInputElement[]).map(input => [input.id, input.value] as [string, string]).filter(([id]) => id)
    );
    return Object.fromEntries(pairs);
  }

  /**
   * Fill every attribute field on the Attributes tab with a unique value derived from `prefix`,
   * and return what was written (keyed by field id) for later comparison.
   */
  async fillAttributeFields(prefix: string): Promise<Record<string, string>> {
    const ids = await this.attributeInputs.evaluateAll(inputs =>
      (inputs as HTMLInputElement[]).map(input => input.id).filter(Boolean)
    );

    const values: Record<string, string> = {};
    for (const [index, id] of ids.entries()) {
      const input = this.activeTabPanel.locator(`#${id}`);

      // `picture` isn't free text: the Console renders it as an avatar spec (emoji:/avatar:/URL)
      // and falls back to displaying an unrecognized string as literal text on the user's avatar,
      // which then leaks into shared UI (e.g. the dashboard's recent-users widget). Leave it
      // untouched so this test can't pollute other pages with garbage-looking avatars.
      if (id === "picture") {
        values[id] = await input.inputValue();
        continue;
      }
      // The email field is validated against a regex, so a generic unique string won't pass.
      const value = id === "email" ? `${prefix}_${index}@example.com` : `${prefix}_${id}_${index}`;
      await input.fill(value);
      values[id] = value;
    }
    return values;
  }

  /** Click the page-level "Reset" button that discards unsaved attribute edits. */
  async clickResetChanges() {
    await this.resetChangesButton.waitFor({ state: "visible", timeout: Timeouts.ELEMENT_VISIBILITY });
    await this.resetChangesButton.click();
  }

  /** Click the page-level "Save" button that persists unsaved attribute edits. */
  async clickSaveChanges() {
    await this.saveChangesButton.waitFor({ state: "visible", timeout: Timeouts.ELEMENT_VISIBILITY });
    await this.saveChangesButton.click();
  }

  /** Click "Reset Password" (or another credential field's reset button) on the Credentials tab. */
  async clickResetPassword(fieldLabel: string = "Password") {
    const button = this.activeTabPanel.getByRole("button", { name: new RegExp(`^reset ${fieldLabel}$`, "i") });
    await button.waitFor({ state: "visible", timeout: Timeouts.ELEMENT_VISIBILITY });
    await button.click();
  }

  /** Fill the New/Confirm fields in the credential reset dialog. Defaults confirm to match new. */
  async fillCredentialResetForm(newValue: string, confirmValue: string = newValue) {
    await this.newCredentialValueInput.waitFor({ state: "visible", timeout: Timeouts.ELEMENT_VISIBILITY });
    await this.newCredentialValueInput.fill(newValue);
    await this.confirmCredentialValueInput.fill(confirmValue);
  }

  /** Confirm the credential reset dialog (e.g. "Reset Password"). */
  async confirmResetPassword() {
    await this.confirmResetPasswordButton.click();
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
