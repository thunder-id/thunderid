/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

/**
 * Sample App Login Page Object
 *
 * Page Object Model for the React Vanilla Sample App login functionality.
 * Provides methods to interact with login form and verify authentication.
 */

import { Page, expect } from "@playwright/test";
import { BasePage } from "../base.page";
import { Timeouts } from "../../constants/timeouts";

export class SampleAppLoginPage extends BasePage {
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

  async verifyLoginPageLoaded() {
    await this.page.waitForSelector('input[name="username"], input[placeholder*="username" i]', {
      timeout: Timeouts.NETWORK_IDLE,
      state: "visible",
    });
  }

  /**
   * Fill in the login form
   * @param username - Username to enter
   * @param password - Password to enter
   */
  async fillLoginForm(username: string, password: string) {
    // Fill username
    const usernameInput = this.page.locator('input[name="username"], input[placeholder*="username" i]').first();
    await usernameInput.waitFor({ state: "visible", timeout: Timeouts.DEFAULT_ACTION });
    await usernameInput.fill(username);

    // Fill password
    const passwordInput = this.page.locator('input[name="password"], input[placeholder*="password" i]').first();
    await passwordInput.waitFor({ state: "visible", timeout: Timeouts.DEFAULT_ACTION });
    await passwordInput.fill(password);
  }

  /**
   * Click the login/sign in button
   */
  async clickLogin() {
    // Try multiple selector strategies for the login button
    const loginButton = this.page
      .locator(
        'button[type="submit"], button:has-text("Sign In"), button:has-text("Login"), button:has-text("Sign in")'
      )
      .first();

    await loginButton.waitFor({ state: "visible", timeout: Timeouts.DEFAULT_ACTION });
    await loginButton.click();
  }

  /**
   * Perform complete login flow
   * @param username - Username to login with
   * @param password - Password to login with
   */
  async login(username: string, password: string) {
    await this.fillLoginForm(username, password);
    await this.clickLogin();

    // Wait for navigation/response after login
    await this.page.waitForLoadState("networkidle");
  }

  /**
   * Verify user is logged in successfully
   * Checks for common indicators like avatar, profile information, or welcome message
   */
  async verifyLoggedIn() {
    // Wait for login to complete - look for logged-in state indicators
    await this.page.waitForLoadState("networkidle");

    // Check for common logged-in indicators (adjust selectors based on your app)
    const loggedInIndicators = [
      'button[aria-haspopup="true"]', // Avatar menu button
      'button:has(> div[class*="MuiAvatar"])', // Avatar button
      '[role="menuitem"]:has-text("Sign Out")', // May be visible if menu is open
      '[data-testid="user-profile"]',
      "text=/welcome|hello/i",
      ".user-profile",
      ".logged-in",
      ".token-container", // Token display container
    ];

    // Wait for at least one indicator to appear
    let found = false;
    for (const selector of loggedInIndicators) {
      const element = this.page.locator(selector).first();
      const count = await element.count();
      if (count > 0) {
        const isVisible = await element.isVisible().catch(() => false);
        if (isVisible) {
          found = true;
          break;
        }
      }
    }

    // If none of the indicators are found, check if we're no longer on login page
    if (!found) {
      // Verify login form is no longer visible
      const usernameInput = this.page.locator('input[name="username"], input[placeholder*="username" i]');
      const usernameCount = await usernameInput.count();

      if (usernameCount > 0) {
        const isLoginFormVisible = await usernameInput
          .first()
          .isVisible()
          .catch(() => false);
        expect(isLoginFormVisible).toBe(false);
      }
    }

    // Take a screenshot for verification
    await this.screenshot("logged-in-state");
  }

  /**
   * Verify specific user information is displayed
   * @param userInfo - Expected user information (e.g., username, email)
   */
  async verifyUserInfo(userInfo: string) {
    await expect(this.page.locator(`text=${userInfo}`)).toBeVisible({ timeout: Timeouts.DEFAULT_ACTION });
  }

  /**
   * Click logout button
   * The logout option is in a dropdown menu accessed via Avatar button
   */
  async logout() {
    // First, look for the avatar/menu button to open the menu
    const avatarButton = this.page
      .locator(
        'button[aria-haspopup="true"], button[aria-controls="account-menu"], button:has(> div[class*="MuiAvatar"])'
      )
      .first();

    // Check if avatar button exists (indicates logged in state with menu)
    const avatarCount = await avatarButton.count();

    if (avatarCount > 0) {
      // Click avatar to open menu
      await avatarButton.waitFor({ state: "visible", timeout: Timeouts.DEFAULT_ACTION });
      await avatarButton.click();

      // Wait for menu to appear
      await this.page.waitForSelector('#account-menu, [role="menu"]', {
        state: "visible",
        timeout: Timeouts.DEFAULT_ACTION,
      });

      // Click the Sign Out menu item
      const signOutMenuItem = this.page
        .locator('[role="menuitem"]:has-text("Sign Out"), [role="menuitem"]:has-text("Logout")')
        .first();

      await signOutMenuItem.waitFor({ state: "visible", timeout: Timeouts.DEFAULT_ACTION });
      await signOutMenuItem.click();
    } else {
      // Fallback: Try direct logout button (for other app implementations)
      const logoutButton = this.page
        .locator('button:has-text("Logout"), button:has-text("Sign Out"), [data-testid="logout-button"]')
        .first();

      await logoutButton.waitFor({ state: "visible", timeout: Timeouts.DEFAULT_ACTION });
      await logoutButton.click();
    }

    await this.page.waitForLoadState("networkidle");
  }

  /**
   * Verify logout was successful
   */
  async verifyLoggedOut() {
    await this.verifyLoginPageLoaded();
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
   * Complete OTP verification step
   * @param otp - OTP code to verify
   */
  async verifyOTP(otp: string) {
    await this.fillOTP(otp);
    await this.clickVerifyOTP();

    // Wait for navigation/response after OTP verification
    await this.page.waitForLoadState("networkidle");
  }
}
