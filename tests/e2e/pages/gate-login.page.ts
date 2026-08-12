// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Gate Login Page Object
 *
 * The OAuth authorization_code redirect flow always lands on ThunderID's shared "gate" app, which
 * renders the login form and account-menu logout dynamically from the client application's auth
 * flow. That DOM is identical regardless of which client app (react-sdk-sample, Wayfinder, ...)
 * initiated the redirect, so the methods here are shared across every sample-app page object
 * rather than duplicated per app. Only the client app's own home-page chrome (its sign-in button,
 * its post-login landing content) is app-specific and lives in the subclass.
 */

import { Page, expect } from "@playwright/test";
import { BasePage } from "./base.page";
import { Timeouts } from "../constants/timeouts";

export class GateLoginPage extends BasePage {
  constructor(page: Page) {
    super(page);
  }

  async verifyLoginPageLoaded() {
    await this.page.waitForSelector('input[name="username"], input[placeholder*="username" i]', {
      timeout: Timeouts.ELEMENT_VISIBILITY,
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
   * Perform complete login flow. Callers verify the result with verifyLoggedIn(), whose own
   * assertion is the thing actually worth waiting on - so there's no wait here beyond the click.
   * @param username - Username to login with
   * @param password - Password to login with
   */
  async login(username: string, password: string) {
    await this.fillLoginForm(username, password);
    await this.clickLogin();
  }

  /**
   * Verify user is logged in successfully
   * Checks for common indicators like avatar, profile information, or welcome message
   */
  async verifyLoggedIn() {
    // Check for common logged-in indicators (adjust selectors based on your app)
    const loggedInIndicators = [
      this.page.locator('button[aria-haspopup="true"]'), // Avatar menu button
      this.page.locator('button[aria-haspopup="menu"]'), // Avatar menu button (menu-flavored haspopup)
      this.page.locator('button:has(> div[class*="MuiAvatar"])'), // Avatar button
      this.page.locator('[role="menuitem"]:has-text("Sign Out")'), // May be visible if menu is open
      this.page.locator('[data-testid="user-profile"]'),
      this.page.getByText(/welcome|hello/i),
      this.page.locator(".user-profile"),
      this.page.locator(".logged-in"),
      this.page.locator(".token-container"), // Token display container
    ];

    // Require at least one indicator to appear. This must assert rather than probe: a sign-in that
    // fails client-side (for example a blocked cross-origin token read) still leaves the app on a
    // page with no login form, so tolerating a missing indicator would report success for a broken
    // sign-in and push the failure into whatever assertion happens to run next.
    const anyLoggedInIndicator = loggedInIndicators.reduce((combined, locator) => combined.or(locator));
    await expect(anyLoggedInIndicator.first()).toBeVisible({ timeout: Timeouts.DEFAULT_ACTION });

    // Take a screenshot for verification
    await this.screenshot("logged-in-state");
  }

  /**
   * Click logout button
   * The logout option is in a dropdown menu accessed via Avatar button.
   * Callers verify the result with verifyLoggedOut(), so there's no wait here beyond the click.
   */
  async logout() {
    // First, look for the avatar/menu button to open the menu
    const avatarButton = this.page
      .locator(
        'button[aria-haspopup="true"], button[aria-haspopup="menu"], button[aria-controls="account-menu"], button:has(> div[class*="MuiAvatar"])'
      )
      .first();

    // Fallback: direct logout button (for other app implementations)
    const logoutButton = this.page
      .locator('button:has-text("Logout"), button:has-text("Sign Out"), [data-testid="logout-button"]')
      .first();

    // Wait (with auto-retry) for whichever logout indicator is visible first
    await avatarButton.or(logoutButton).first().waitFor({ state: "visible", timeout: Timeouts.DEFAULT_ACTION });

    if (await avatarButton.isVisible().catch(() => false)) {
      // Click avatar to open menu
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
      await signOutMenuItem.dispatchEvent("click");
    } else {
      await logoutButton.click();
    }

    // RP-initiated logout redirects to the gate's sign-out confirmation flow before the
    // post-logout redirect back to the app; confirm it if it appears.
    const confirmSignOutButton = this.page.getByRole("button", { name: "Sign out", exact: true });
    const confirmed = await confirmSignOutButton
      .waitFor({ state: "visible", timeout: Timeouts.DEFAULT_ACTION })
      .then(() => true)
      .catch(() => false);
    if (confirmed) {
      await confirmSignOutButton.click();
    }
  }
}
