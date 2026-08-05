// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Console Sign-in Page Object Model
 *
 * Encapsulates all locators and actions for the Console login page.
 *
 * @example
 * const signinPage = new ConsoleSigninPage(page, baseUrl);
 * await signinPage.goto();
 * await signinPage.login('admin', 'password');
 */

import { Page, Locator, expect } from "@playwright/test";
import { ConsoleRoutes } from "../../configs/routes/console-routes";
import { BasePage } from "../base.page";
import { Timeouts } from "../../constants/timeouts";

export class ConsoleSigninPage extends BasePage {
  readonly baseUrl: string;

  // Locators
  readonly usernameInput: Locator;
  readonly passwordInput: Locator;
  readonly signInButton: Locator;
  readonly errorMessage: Locator;

  constructor(page: Page, baseUrl: string) {
    super(page);
    this.baseUrl = baseUrl;

    // Username field
    this.usernameInput = page
      .locator('input[name="username"]')
      .or(page.locator('input[type="text"]'))
      .or(page.locator('input[id*="username"]'));

    // Password field
    this.passwordInput = page.locator('input[name="password"]').or(page.locator('input[type="password"]'));

    // Sign in button
    this.signInButton = page
      .locator('button[type="submit"]')
      .or(page.getByRole("button", { name: /sign in|login|submit/i }));

    // Error message
    this.errorMessage = page.locator('[class*="error"], [role="alert"], .error-message');
  }

  /**
   * Navigate to the login page. Callers follow this with their own explicit wait
   * (waitForLoginForm(), login(), etc.)
   */
  async goto() {
    await this.page.goto(`${this.baseUrl}${ConsoleRoutes.signin}`, {
      timeout: Timeouts.PAGE_LOAD,
    });
  }

  /**
   * Navigate to home page (redirects to login if not authenticated). */
  async gotoHome() {
    await this.page.goto(`${this.baseUrl}${ConsoleRoutes.home}`, {
      timeout: Timeouts.PAGE_LOAD,
    });
  }

  /** Check if currently on login page */
  async isOnLoginPage(): Promise<boolean> {
    const url = this.page.url();
    return url.includes(ConsoleRoutes.signin) || url.includes("/auth") || url.includes("/login");
  }

  /** Wait for login form to be visible */
  async waitForLoginForm() {
    await this.usernameInput.first().waitFor({ state: "visible", timeout: Timeouts.ELEMENT_VISIBILITY });
  }

  /** Fill username field */
  async fillUsername(username: string) {
    await this.usernameInput.first().fill(username);
  }

  /** Fill password field */
  async fillPassword(password: string) {
    await this.passwordInput.first().fill(password);
  }

  /** Click the sign in button */
  async clickSignIn() {
    await this.signInButton.first().click();
  }

  /** Perform complete login flow */
  async login(username: string, password: string) {
    await this.waitForLoginForm();
    await this.fillUsername(username);
    await this.fillPassword(password);
    await this.clickSignIn();
  }

  /** Wait for successful login */
  async waitForLoginSuccess() {
    await this.page.waitForURL(/\/console(\/|$)/, { timeout: Timeouts.PAGE_LOAD });
  }

  /** Verify login was successful */
  async verifyLoginSuccess() {
    const url = this.page.url();
    expect(url).toContain(ConsoleRoutes.home);
    expect(url).not.toContain(ConsoleRoutes.signin);
  }
}
