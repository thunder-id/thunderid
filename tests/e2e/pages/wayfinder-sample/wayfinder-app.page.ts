// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Wayfinder Sample App Page Object
 *
 * Page Object Model for the standalone Wayfinder sample app's own home-page chrome (plain
 * hand-built sign-in button, not an SDK component). Login form and logout are shared gate
 * behavior, inherited from GateLoginPage.
 */

import { Page, Locator, expect } from "@playwright/test";
import { GateLoginPage } from "../gate-login.page";
import { Timeouts } from "../../constants/timeouts";

export class WayfinderAppPage extends GateLoginPage {
  readonly signInButton: Locator;
  readonly signupButton: Locator;
  readonly continueButton: Locator;
  readonly submitButton: Locator;
  readonly emailInput: Locator;
  readonly signupErrorAlert: Locator;
  readonly registrationCompleteHeading: Locator;
  readonly registrationCompleteLink: Locator;

  // Password Reset Locators
  readonly resetPasswordLink: Locator;
  readonly sendRecoveryLinkButton: Locator;
  readonly resetPasswordConfirmationHeading: Locator;
  readonly userNotFoundErrorAlert: Locator;
  readonly recoveryUsernameInput: Locator;
  readonly newPasswordInput: Locator;
  readonly resetPasswordSubmitButton: Locator;
  readonly passwordResetSuccessHeading: Locator;

  constructor(page: Page) {
    super(page);
    this.signInButton = page.getByRole("button", { name: /^sign in$/i });
    // The gate's self-registration flow is reached via a "Sign up" link on the login page (a
    // rich-text <a>, not a <button> - see the flow's action_signup component), and every one of
    // its own PROMPT steps submits via a button literally labeled "Continue".
    this.signupButton = page.getByRole("link", { name: /^sign up$/i });
    this.continueButton = page.getByRole("button", { name: /^continue$/i });
    this.submitButton = page.getByRole("button", { name: /^submit$/i });
    this.emailInput = page.locator('input[name="email"]');
    // Rendered above the "Create your account" form when the submitted username collides with
    // an existing account.
    this.signupErrorAlert = page.getByRole("alert");
    // The registration_complete node's confirmation screen (heading "Account Created" + a link
    // back to sign-in) - see the flow's registration_complete_heading/registration_complete_link
    // components. Registration itself does not sign the new user in.
    this.registrationCompleteHeading = page.getByRole("heading", { name: /account created/i });
    this.registrationCompleteLink = page.getByRole("link", { name: /go to wayfinder/i });

    // Password Reset
    this.resetPasswordLink = page.getByRole("link", { name: /reset/i });
    this.sendRecoveryLinkButton = page.getByRole("button", { name: /send recovery link/i });
    // await expect(page.getByRole('alert')).toBeVisible();
    // await expect(page.getByText('User not found')).toBeVisible();
    this.userNotFoundErrorAlert = page.getByRole("alert");
    this.resetPasswordConfirmationHeading = page.getByRole("heading", { name: /check your email/i });
    this.recoveryUsernameInput = page.locator('input[name="username"], input[name="email"]').first();
    this.newPasswordInput = page.locator('input[type="password"], input[name="password"]').first();
    this.resetPasswordSubmitButton = page.getByRole("button", { name: /reset password/i });
    this.passwordResetSuccessHeading = page.getByRole("heading", { name: /password reset successful|success/i });
  }

  /**
   * Navigate to the Wayfinder sample app
   * @param url - Wayfinder app URL (defaults to WAYFINDER_APP_URL, falling back to the app's
   *   own default dev port if unset)
   */
  async goto(url: string = process.env.WAYFINDER_APP_URL || "http://localhost:5173") {
    await this.page.goto(url, { waitUntil: "commit" });
  }

  /**
   * Verify the (unauthenticated) home page is loaded
   */
  async verifyUnAuthenticatedHomePageLoaded() {
    await this.signInButton.waitFor({ state: "visible", timeout: Timeouts.NETWORK_IDLE });
  }

  async clickSignInButton() {
    await this.signInButton.waitFor({ state: "visible", timeout: Timeouts.DEFAULT_ACTION });
    await this.signInButton.click();
  }

  /** From the login page, follow the "Don't have an account? Sign up" link into self-registration. */
  async clickSignupLink() {
    await this.signupButton.waitFor({ state: "visible", timeout: Timeouts.DEFAULT_ACTION });
    await this.signupButton.click();
  }

  /** Submit whichever step of the self-registration flow is currently on screen. */
  async clickContinueButton() {
    await this.continueButton.waitFor({ state: "visible", timeout: Timeouts.DEFAULT_ACTION });
    await this.continueButton.click();
  }

  /**
   * Verify the registration flow's first step is loaded: same username/password fields as the
   * login page, but submitted via "Continue" rather than "Sign In".
   */
  async verifyFirstSignupPageLoaded() {
    await this.verifyLoginPageLoaded();
    await this.continueButton.waitFor({ state: "visible", timeout: Timeouts.ELEMENT_VISIBILITY });
  }

  /**
   * Verify the registration flow's second step is loaded: once the new username doesn't collide
   * with an existing account, it asks for the remaining required profile field (email) before
   * completing registration and signing the new user in.
   */
  async verifySecondSignupPageLoaded() {
    await this.emailInput.waitFor({ state: "visible", timeout: Timeouts.ELEMENT_VISIBILITY });
    await this.continueButton.waitFor({ state: "visible", timeout: Timeouts.ELEMENT_VISIBILITY });
  }

  /** Verify the self-registration flow rejected the submitted username because it already exists. */
  async verifyUserAlreadyExistsError() {
    await expect(this.signupErrorAlert).toBeVisible({ timeout: Timeouts.ELEMENT_VISIBILITY });
    await expect(this.signupErrorAlert).toContainText(/user already exists/i);
  }

  /**
   * Verify the registration flow's terminal confirmation screen is loaded. Self-registration
   * only creates the account - it does not sign the new user in - so this screen, not
   * verifyLoggedIn(), is the correct end state for TC003.
   */
  async verifyRegistrationCompleteScreenLoaded() {
    await expect(this.registrationCompleteHeading).toBeVisible({ timeout: Timeouts.ELEMENT_VISIBILITY });
    await expect(this.registrationCompleteLink).toBeVisible({ timeout: Timeouts.ELEMENT_VISIBILITY });
  }

  /** Follow the confirmation screen's link back to sign-in. */
  async clickRegistrationCompleteLink() {
    await this.registrationCompleteLink.click();
  }

  async clickSubmitButton() {
    await this.submitButton.waitFor({ state: "visible", timeout: Timeouts.DEFAULT_ACTION });
    await this.submitButton.click();
  }

  /**
   * Fill the registration step currently on screen: username + password on the first step, or
   * just the email on the second - called with one argument there.
   */
  async fillSignupForm(usernameOrEmail: string, password?: string) {
    if (password !== undefined) {
      await this.fillLoginForm(usernameOrEmail, password);
      return;
    }
    await this.emailInput.waitFor({ state: "visible", timeout: Timeouts.DEFAULT_ACTION });
    await this.emailInput.fill(usernameOrEmail);
  }

  /**
   * Verify logout was successful. RP-initiated logout redirects back to the app's home page.
   */
  async verifyLoggedOut() {
    await this.verifyUnAuthenticatedHomePageLoaded();
  }

  // Password Reset Methods

  async clickResetPasswordLink() {
    await this.resetPasswordLink.waitFor({ state: "visible", timeout: Timeouts.DEFAULT_ACTION });
    await this.resetPasswordLink.click();
  }

  async verifyResetPasswordPageLoaded() {
    await expect(this.sendRecoveryLinkButton).toBeVisible({ timeout: Timeouts.ELEMENT_VISIBILITY });
  }

  async fillResetPasswordForm(usernameOrEmail: string) {
    await this.recoveryUsernameInput.waitFor({ state: "visible", timeout: Timeouts.DEFAULT_ACTION });
    await this.recoveryUsernameInput.fill(usernameOrEmail);
  }

  async verifyResetPasswordConfirmationScreenLoaded() {
    await expect(this.resetPasswordConfirmationHeading).toBeVisible({ timeout: Timeouts.ELEMENT_VISIBILITY });
  }

  async clickSendRecoveryLinkButton() {
    await this.sendRecoveryLinkButton.waitFor({ state: "visible", timeout: Timeouts.DEFAULT_ACTION });
    await this.sendRecoveryLinkButton.click();
  }

  async verifyRecoverUserNotFoundError() {
    await expect(this.userNotFoundErrorAlert).toBeVisible({ timeout: Timeouts.ELEMENT_VISIBILITY });
    await expect(this.userNotFoundErrorAlert).toContainText(/user not found/i);
  }

  async verifyNewPasswordPageLoaded() {
    await expect(this.resetPasswordSubmitButton).toBeVisible({ timeout: Timeouts.ELEMENT_VISIBILITY });
  }

  async fillNewPasswordForm(password: string) {
    await this.newPasswordInput.waitFor({ state: "visible", timeout: Timeouts.DEFAULT_ACTION });
    await this.newPasswordInput.fill(password);
  }

  async clickResetPasswordSubmitButton() {
    await this.resetPasswordSubmitButton.waitFor({ state: "visible", timeout: Timeouts.DEFAULT_ACTION });
    await this.resetPasswordSubmitButton.click();
  }

  async verifyPasswordResetSuccessful() {
    await expect(this.passwordResetSuccessHeading).toBeVisible({ timeout: Timeouts.ELEMENT_VISIBILITY });
  }
}
