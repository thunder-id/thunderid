// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Sample App Login Page Object
 *
 * Page Object Model for the React SDK Sample App login functionality. Home-page chrome
 * (sign-in button, post-login landing) is specific to this app's SDK-rendered button; the login
 * form and logout flow are shared gate behavior, inherited from GateLoginPage.
 */

import { Page, expect } from "@playwright/test";
import { GateLoginPage } from "../gate-login.page";
import { Timeouts } from "../../constants/timeouts";

export class SampleAppLoginPage extends GateLoginPage {
  constructor(page: Page) {
    super(page);
  }

  /**
   * Navigate to the sample app
   * @param url - Sample app URL (default: https://localhost:3000)
   */
  async goto(url: string = "https://localhost:3000") {
    await this.page.goto(url, { waitUntil: "commit" });
  }

  /**
   * Verify the login page is loaded
   */
  async verifyHomePageLoaded() {
    // Wait for login form to be visible
    await this.page.waitForSelector('span.thunderid-button__content:has-text("Sign In")', {
      timeout: Timeouts.NETWORK_IDLE,
      state: "visible",
    });
  }

  async clickSignInButton() {
    const signInButton = this.page.locator('span.thunderid-button__content:has-text("Sign In")').first();
    await signInButton.waitFor({ state: "visible", timeout: Timeouts.DEFAULT_ACTION });
    await signInButton.click();
  }

  /**
   * Navigate to the sample app and click through to the sign-in form.
   * @param url - Sample app URL
   */
  async gotoLoginPage(url: string) {
    await this.goto(url);
    await this.verifyHomePageLoaded();
    await this.clickSignInButton();
    await this.verifyLoginPageLoaded();
  }

  /**
   * Verify specific user information is displayed
   * @param userInfo - Expected user information (e.g., username, email)
   */
  async verifyUserInfo(userInfo: string) {
    await expect(this.page.locator(`text=${userInfo}`)).toBeVisible({ timeout: Timeouts.DEFAULT_ACTION });
  }

  /**
   * Verify logout was successful
   * RP-initiated logout redirects back to the app's post-logout redirect URI (the home page),
   * not the login form.
   */
  async verifyLoggedOut() {
    await this.verifyHomePageLoaded();
  }

  /**
   * Verify OTP input page is loaded
   * Used for MFA authentication flows where OTP verification is required
   * Handles both single input and MUI separate digit inputs
   */
  async verifyOTPPageLoaded() {
    // Wait for either "Verify OTP" heading or OTP input fields
    await this.page.waitForSelector(
      'h3:has-text("Verify OTP"), input[aria-label*="OTP digit" i], input[name="otp"], input[placeholder*="otp" i]',
      {
        timeout: Timeouts.NETWORK_IDLE,
        state: "visible",
      }
    );
  }

  /**
   * Fill in the OTP input field
   * Handles both single input field and MUI separate digit inputs (6 boxes)
   * @param otp - OTP code to enter (e.g., "123456")
   */
  async fillOTP(otp: string) {
    // Check if MUI separate digit inputs exist (aria-label="OTP digit 1", etc.)
    const digitInputs = this.page.locator('input[aria-label*="OTP digit" i]');
    const digitCount = await digitInputs.count();

    if (digitCount > 0) {
      // MUI separate digit inputs - fill each digit individually
      console.log(`  Filling ${digitCount} separate OTP digit inputs...`);
      const digits = otp.split("");

      for (let i = 0; i < Math.min(digits.length, digitCount); i++) {
        const input = digitInputs.nth(i);
        await input.waitFor({ state: "visible", timeout: Timeouts.DEFAULT_ACTION });
        await input.fill(digits[i]);
        // Small delay to allow auto-focus to next field
        const hasNextDigit = i + 1 < Math.min(digits.length, digitCount);
        if (hasNextDigit) {
          const nextInput = digitInputs.nth(i + 1);
          await expect(nextInput).toBeFocused({ timeout: Timeouts.DEFAULT_ACTION });
        }
      }
    } else {
      // Single OTP input field
      const otpInput = this.page
        .locator('input[name="otp"], input[placeholder*="otp" i], input[placeholder*="code" i]')
        .first();

      await otpInput.waitFor({ state: "visible", timeout: Timeouts.DEFAULT_ACTION });
      await otpInput.fill(otp);
    }
  }

  /**
   * Click the submit/verify OTP button
   * Looks for "Verify" button specifically for MUI OTP form
   */
  async clickVerifyOTP() {
    // MUI OTP form has a "Verify" button (not type="submit")
    const verifyButton = this.page
      .locator(
        'button:has-text("Verify"):not(:has-text("Resend")), button:has-text("Submit"), button:has-text("Continue"), button[type="submit"]'
      )
      .first();

    await verifyButton.waitFor({ state: "visible", timeout: Timeouts.DEFAULT_ACTION });
    await verifyButton.click();
  }

  /**
   * Complete OTP verification step. Callers verify the result themselves (verifyLoggedIn(),
   * getOTPErrorMessage(), etc.), so there's no wait here beyond the click.
   * @param otp - OTP code to verify
   */
  async verifyOTP(otp: string) {
    await this.fillOTP(otp);
    await this.clickVerifyOTP();
  }

  /**
   * Returns the visible OTP error message text, or an empty string if none is present.
   */
  async getOTPErrorMessage(): Promise<string> {
    const text = await this.page
      .locator(".MuiAlert-message, .MuiAlert-colorError .MuiAlertTitle-root")
      .first()
      .textContent()
      .catch(() => "");
    return text?.trim() ?? "";
  }
}
