// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Users Page Object Model
 *
 * Encapsulates the Admin Console's users pages: the list, the Add people dialog (which runs the
 * user onboarding flow), and a user's details page. Also drives the flow forms the gate renders
 * when an invitee accepts an invitation.
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
import { UIMessages } from "../../constants/ui-messages";
import { AccountStatusControls } from "../components/account-status-controls";

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

  // List page
  readonly addUserButton: Locator;

  // Add people dialog. The onboarding flow renders each step inside it, and the dialog's own Close
  // button returns to the list from any step.
  readonly wizard: Locator;
  readonly onboardingModeHeading: Locator;
  readonly userTypeSelect: Locator;
  readonly continueButton: Locator;
  readonly nextButton: Locator;
  readonly closeButton: Locator;

  // Invite steps
  readonly getInviteLinkButton: Locator;
  readonly copyInviteLinkButton: Locator;

  // Staff onboarding flow (email-based invite: "Which staff role?" -> "Invitee email")
  readonly sendInvitationButton: Locator;
  readonly invitationSent: Locator;

  // Details page
  readonly main: Locator;
  readonly editButton: Locator;
  readonly saveChangesButton: Locator;
  readonly cancelEditButton: Locator;
  readonly deleteUserButton: Locator;
  /** The inline confirmation the delete action asks for. */
  readonly deleteConfirmation: Locator;

  readonly accountStatus: AccountStatusControls;

  constructor(page: Page, baseUrl: string) {
    super(page);
    this.baseUrl = baseUrl;
    this.accountStatus = new AccountStatusControls(page);

    this.addUserButton = page.getByRole("link", { name: /^add people$/i });

    this.wizard = page.getByRole("dialog");
    this.onboardingModeHeading = this.wizard.getByRole("heading", { name: /^add user$/i });
    this.userTypeSelect = this.wizard.locator('select[name="userType"]');
    this.continueButton = this.wizard.getByRole("button", { name: /^continue$/i });
    this.nextButton = this.wizard.getByRole("button", { name: /^next$/i });
    this.closeButton = this.wizard.getByRole("button", { name: /^close$/i });

    this.getInviteLinkButton = this.wizard.getByRole("button", { name: /get invite link/i });
    this.copyInviteLinkButton = this.wizard.getByRole("button", { name: /^copy$/i });

    this.sendInvitationButton = page.getByRole("button", { name: /send invitation/i });
    // The staff flow's completion step; the dialog shows its message, not its "Invitation sent" text.
    this.invitationSent = this.wizard.getByText(/can now accept from email/i);

    this.main = page.getByRole("main");
    this.editButton = this.main.getByRole("button", { name: "Edit", exact: true });
    this.saveChangesButton = this.main.getByRole("button", { name: "Save changes", exact: true });
    this.cancelEditButton = this.main.getByRole("button", { name: "Cancel", exact: true });
    this.deleteUserButton = this.main.getByRole("button", { name: UIMessages.deletion.user, exact: true });
    this.deleteConfirmation = this.main.getByRole("group").filter({ hasText: /^Delete .+\?/ });
  }

  /** Navigate to the users list. */
  async goto() {
    await this.page.goto(`${this.baseUrl}${ConsoleRoutes.users}`, {
      timeout: Timeouts.PAGE_LOAD,
    });
  }

  /** Click Add people, which opens the dialog over the list. */
  async clickAddUser() {
    await this.addUserButton.waitFor({ state: "visible", timeout: Timeouts.ELEMENT_VISIBILITY });
    await this.addUserButton.click();
    await this.page.waitForURL(`**${ConsoleRoutes.userCreate}`, { timeout: Timeouts.PAGE_LOAD });
    await this.wizard.waitFor({ state: "visible", timeout: Timeouts.ELEMENT_VISIBILITY });
  }

  /**
   * Open Add people and drive the onboarding flow up to the mode-specific step: the details form
   * for "create", the email prompt for "invite". The flow asks for a user type, and then an
   * organization unit, first only when there is more than one to choose from.
   */
  async openAddUserWizard(mode: AddUserMode) {
    await this.goto();
    await this.clickAddUser();

    const firstStep = this.onboardingModeHeading.or(this.userTypeSelect);
    await firstStep.first().waitFor({ state: "visible", timeout: Timeouts.FORM_LOAD });
    if (!(await this.onboardingModeHeading.isVisible())) {
      await this.selectUserTypeAndContinue();
    }

    // An organization unit step, when the flow asks for one, keeps the unit it preselects.
    await this.onboardingModeHeading.or(this.continueButton).first().waitFor({
      state: "visible",
      timeout: Timeouts.FORM_LOAD,
    });
    if (!(await this.onboardingModeHeading.isVisible())) {
      await this.continueButton.click();
    }

    await this.chooseOnboardingMode(mode);
    await this.wizard.getByRole("textbox").first().waitFor({ state: "visible", timeout: Timeouts.FORM_LOAD });
  }

  /** Click "Create User" or "Invite User" on the onboarding-mode step. */
  async chooseOnboardingMode(mode: AddUserMode) {
    const choice = this.wizard.getByRole("button", { name: mode === "create" ? /^create user$/i : /^invite user$/i });
    await choice.waitFor({ state: "visible", timeout: Timeouts.FORM_LOAD });
    await choice.click();
    await expect(this.onboardingModeHeading).toBeHidden({ timeout: Timeouts.FORM_LOAD });
  }

  /**
   * Choose a user type and continue. Picks the first type when `userTypeName` is omitted, and
   * otherwise the type whose name matches it regardless of case (e.g. "staff", for the Wayfinder
   * staff onboarding flow). The select's placeholder option is disabled, so it is never a choice.
   */
  async selectUserTypeAndContinue(userTypeName?: string) {
    await this.userTypeSelect.first().waitFor({ state: "visible", timeout: Timeouts.FORM_LOAD });
    const labels = (
      await this.userTypeSelect.first().locator('option:not([disabled]):not([value=""])').allTextContents()
    ).map(label => label.trim());
    const label = userTypeName
      ? labels.find(candidate => candidate.toLowerCase() === userTypeName.toLowerCase())
      : labels.find(Boolean);
    if (!label) throw new Error(`No user type ${userTypeName ?? ""} among: ${labels.join(", ")}`);

    await this.userTypeSelect.first().selectOption({ label });
    await this.continueButton.click();
    await expect(this.userTypeSelect).toHaveCount(0, { timeout: Timeouts.FORM_LOAD });
  }

  /** Click Next to advance the invite flow past the email step. */
  async clickNextButton() {
    await expect(this.nextButton).toBeEnabled({ timeout: Timeouts.ELEMENT_VISIBILITY });
    await this.nextButton.click();
  }

  /** Click "Get Invite Link" on the invite step. */
  async clickGetInviteLink() {
    await this.getInviteLinkButton.waitFor({ state: "visible", timeout: Timeouts.FORM_LOAD });
    await this.getInviteLinkButton.click();
  }

  /** The generated invite link, shown next to its Copy button. */
  async getInviteLink(): Promise<string> {
    await this.copyInviteLinkButton.waitFor({ state: "visible", timeout: Timeouts.FORM_LOAD });
    // Read the link's own element: the dialog's text runs the link straight into the Copy label.
    const link =
      (await this.wizard
        .getByText(/^https?:\/\//)
        .first()
        .textContent()) ?? "";
    return link.trim();
  }

  /** Click one of the staff onboarding flow's "Which staff role?" buttons (e.g. "Support"). */
  async selectStaffRole(role: string) {
    const roleButton = this.page.getByRole("button", { name: role, exact: true });
    await roleButton.waitFor({ state: "visible", timeout: Timeouts.FORM_LOAD });
    await roleButton.click();
  }

  /** Fill the invitee's email on the staff onboarding flow's "Invitee email" step. */
  async fillInviteeEmail(email: string) {
    const emailInput = this.page.getByRole("textbox", { name: /email/i });
    await emailInput.waitFor({ state: "visible", timeout: Timeouts.FORM_LOAD });
    await emailInput.fill(email);
  }

  /** Click "Send invitation" on the staff onboarding flow's "Invitee email" step. */
  async clickSendInvitation() {
    await expect(this.sendInvitationButton).toBeEnabled({ timeout: Timeouts.FORM_LOAD });
    await this.sendInvitationButton.click();
  }

  /**
   * The container the current flow step renders in: the Add people dialog in the Admin Console,
   * the page itself on the gate.
   */
  private async formScope(): Promise<Locator> {
    return (await this.wizard.count()) > 0 ? this.wizard : this.page.locator("body");
  }

  /**
   * Complete a (possibly multi-step) embedded registration/accept-invite form on the gate,
   * submitting until no required fields remain or maxSteps is reached.
   */
  async completeRegistrationFlow(data: UserFormData, maxSteps: number = 10) {
    const submitButton = this.page.locator('form button[type="submit"]');
    // The flow disables its submit button for exactly as long as a step's POST is in flight, so no
    // disabled submit button means the step settled - either as the next step, or as the completion
    // screen, which has no submit at all.
    const busySubmitButton = this.page.locator('form button[type="submit"][disabled]');

    // The gate's `load` event fires before the SPA validates the invite token and renders the
    // flow's first step, so the loop's instant `isVisible()` check below would break on iteration 0.
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
      // The flow never navigates, so waitForLoadState would return at once; wait for the step's POST
      // and then for the re-render it triggers.
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
   * Fill the current flow step's user fields. Known fields are matched by label; any other empty
   * required field gets a generated value.
   */
  async fillUserForm(data: UserFormData) {
    const scope = await this.formScope();
    // One password value per call, so a password and confirm-password pair match.
    const password = data.password ?? `Test@${Date.now()}`;

    await this.fillIfVisible(scope.getByLabel(/^username/i), data.username);
    await this.fillIfVisible(scope.getByLabel(/^email/i), data.email);
    await this.fillIfVisible(scope.getByLabel(/^(first|given) ?name/i), data.given_name);
    await this.fillIfVisible(scope.getByLabel(/^(last|family) ?name/i), data.family_name);
    await this.fillIfVisible(scope.getByLabel(/^password/i), password);

    // Visible inputs only: a hidden native input backing a select can also carry `required`.
    const requiredInputs = scope.locator('input[required]:not([type="checkbox"]):not([type="radio"])');
    const count = await requiredInputs.count();
    for (let i = 0; i < count; i++) {
      const input = requiredInputs.nth(i);
      if (!(await input.isVisible())) continue;
      if (await input.inputValue()) continue;

      const name = (await input.getAttribute("name")) ?? `field_${i}`;
      const type = await input.getAttribute("type");
      // `type` alone isn't enough: a password field with its show/hide toggle on renders as text.
      const isPassword = type === "password" || /password/i.test(name);
      await input.fill(isPassword ? password : `test_${name}_${Date.now()}`);
    }
  }

  /** Submit the details step ("Create User"). */
  async submitForm() {
    const submit = this.wizard.getByRole("button", { name: /^create user$/i });
    await expect(submit).toBeEnabled({ timeout: Timeouts.ELEMENT_VISIBILITY });
    await submit.click();
  }

  /** Close the Add people dialog, which returns to the users list. */
  async closeWizard() {
    await this.closeButton.click();
  }

  /** Navigate straight to a user's details page by id and wait for it to load. */
  async gotoUserDetails(userId: string) {
    await this.page.goto(`${this.baseUrl}${ConsoleRoutes.userDetails(userId)}`, {
      timeout: Timeouts.PAGE_LOAD,
    });
    await this.editButton.waitFor({ state: "visible", timeout: Timeouts.PAGE_LOAD });
  }

  /** The user's attributes as the details page shows them, keyed by attribute name. */
  async readAttributeValues(): Promise<Record<string, string>> {
    const names = await this.main.getByRole("term").allTextContents();
    const values = await this.main.getByRole("definition").allTextContents();
    return Object.fromEntries(names.map((name, index) => [name.trim(), (values[index] ?? "").trim()]));
  }

  /** Start editing the user's attributes. */
  async clickEdit() {
    await this.editButton.click();
    await this.saveChangesButton.waitFor({ state: "visible", timeout: Timeouts.ELEMENT_VISIBILITY });
  }

  /**
   * Fill every attribute field with a unique value derived from `prefix`, and return what was
   * written, keyed by attribute name.
   */
  async fillAttributeFields(prefix: string): Promise<Record<string, string>> {
    const fields = this.main.getByRole("textbox");
    const values: Record<string, string> = {};

    for (const [index, field] of (await fields.all()).entries()) {
      const name = (await field.getAttribute("aria-label")) ?? (await field.getAttribute("name")) ?? `field_${index}`;
      // Some attributes are validated against a pattern, so a generic unique string won't pass.
      let value = `${prefix}_${name}_${index}`;
      if (name === "email") {
        value = `${prefix}_${index}@example.com`;
      } else if (name === "mobile_number") {
        value = `+1555${String(index).padStart(7, "0")}`;
      }
      await field.fill(value);
      values[name] = value;
    }
    return values;
  }

  /** Discard the attribute edits. */
  async clickCancelEdit() {
    await this.cancelEditButton.click();
    await this.editButton.waitFor({ state: "visible", timeout: Timeouts.ELEMENT_VISIBILITY });
  }

  /** Save the attribute edits and wait for the page to leave edit mode. */
  async clickSaveChanges() {
    await this.saveChangesButton.click();
    await this.editButton.waitFor({ state: "visible", timeout: Timeouts.FORM_LOAD });
  }

  /** Ask to delete the user, which opens the inline confirmation. */
  async clickDeleteUser() {
    await this.deleteUserButton.waitFor({ state: "visible", timeout: Timeouts.ELEMENT_VISIBILITY });
    await this.deleteUserButton.click();
  }

  async confirmDeleteUser() {
    await this.deleteConfirmation.getByRole("button", { name: UIMessages.deletion.confirm, exact: true }).click();
  }

  async cancelDeleteUser() {
    await this.deleteConfirmation.getByRole("button", { name: UIMessages.accountStatus.cancel, exact: true }).click();
  }

  private async fillIfVisible(locator: Locator, value?: string) {
    if (!value) return;
    const first = locator.first();
    // Absent fields are skipped: the Admin Console and the gate render different subsets.
    if (await first.isVisible().catch(() => false)) {
      await first.fill(value);
    }
  }
}
