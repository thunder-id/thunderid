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
 * Users Page Object Model
 *
 * Encapsulates all locators and actions for the User Management page.
 *
 * @example
 * const usersPage = new UsersPage(page, baseUrl);
 * await usersPage.goto();
 * await usersPage.createUser({ username: 'test', email: 'test@test.com' });
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
};

export class UsersPage extends BasePage {
  readonly baseUrl: string;

  // Page Locators
  readonly addUserButton: Locator;
  readonly userTable: Locator;
  readonly searchInput: Locator;

  // Wizard Locators (Step 1: Select User Type)
  readonly userTypeHeading: Locator;
  readonly organizationUnitHeading: Locator;
  readonly onboardingModeHeading: Locator;
  readonly userTypeSelect: Locator;
  readonly continueButton: Locator;
  readonly createUserActionButton: Locator;

  // Form Locators (Step 2: User Details)
  readonly usernameInput: Locator;
  readonly emailInput: Locator;
  readonly givenNameInput: Locator;
  readonly familyNameInput: Locator;
  readonly passwordInput: Locator;
  readonly submitButton: Locator;
  readonly cancelButton: Locator;
  readonly formHeading: Locator;

  // Messages
  readonly successMessage: Locator;
  readonly errorMessage: Locator;

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
    this.onboardingModeHeading = page.locator("h1, h2, h3, h4, h5, h6").filter({ hasText: /^add user$/i });

    // Wizard: User type dropdown
    this.userTypeSelect = page
      .locator('[data-testid="user-type-select"]')
      .or(page.locator("#user-type-select"))
      .or(page.getByRole("combobox"))
      .or(page.locator('[aria-haspopup="listbox"]'));

    // Wizard: Continue button
    this.continueButton = page.getByRole("button", { name: /continue/i });
    this.createUserActionButton = page.getByRole("button", { name: /^create user$/i });

    // User table
    this.userTable = page.locator('table, [role="table"], [data-testid*="user-list"]');

    // Search input
    this.searchInput = page.locator('input[placeholder*="search" i], input[type="search"]');

    // Form fields
    this.usernameInput = page.locator('input[name="username"]').or(page.getByLabel(/username/i));

    this.emailInput = page.locator('input[name="email"]').or(page.getByLabel(/email/i));

    this.givenNameInput = page.locator('input[name="given_name"]').or(page.getByLabel(/first.*name|given.*name/i));

    this.familyNameInput = page.locator('input[name="family_name"]').or(page.getByLabel(/last.*name|family.*name/i));
    this.passwordInput = page.locator('input[name="password"]').or(page.getByLabel(/^password$/i));

    // Form buttons
    this.submitButton = page.getByRole("button", { name: /create.*user|add.*user|submit|save/i });
    this.cancelButton = page.getByRole("button", { name: /cancel|close/i });

    // Form heading (Step 2: "Enter user details")
    this.formHeading = page
      .locator("h1, h2, h3, h4, h5, h6")
      .filter({ hasText: /enter.*user.*details|user.*details/i });

    // Messages
    this.successMessage = page.locator('[class*="success"], [role="status"]');
    this.errorMessage = page.locator('[class*="error"], [role="alert"]');
  }

  /** Navigate to users management page */
  async goto() {
    await this.page.goto(`${this.baseUrl}${ConsoleRoutes.users}`, {
      waitUntil: "networkidle",
      timeout: Timeouts.PAGE_LOAD,
    });
  }

  /** Check if currently on users page */
  async isOnUsersPage(): Promise<boolean> {
    const url = this.page.url();
    return url.includes(ConsoleRoutes.users) && !url.includes(ConsoleRoutes.signin);
  }

  /** Verify page loaded successfully */
  async verifyPageLoaded() {
    const url = this.page.url();
    if (url.includes(ConsoleRoutes.signin)) {
      throw new Error("Authentication failed: Redirected to signin page");
    }
    expect(url).toContain(ConsoleRoutes.users);
  }

  /** Click the Add User button */
  async clickAddUser() {
    await this.addUserButton.first().waitFor({ state: "visible", timeout: Timeouts.ELEMENT_VISIBILITY });
    await this.addUserButton.first().scrollIntoViewIfNeeded();
    await this.addUserButton.first().click();
  }

  /** Wait for the wizard to load (Step 1: Select User Type) */
  async waitForUserForm() {
    await this.waitForAnyVisibleLocator(
      [this.userTypeHeading, this.organizationUnitHeading, this.onboardingModeHeading, this.formHeading],
      Timeouts.FORM_LOAD,
    );
  }

  /** Select the first available user type and advance to Step 2 */
  async selectUserTypeAndContinue() {
    if (await this.isLocatorVisible(this.userTypeSelect)) {
      await this.userTypeSelect.first().click();

      const firstOption = this.page.locator('[role="option"]:not([aria-disabled="true"])').first();
      await firstOption.waitFor({ state: "visible", timeout: Timeouts.ELEMENT_VISIBILITY });
      await firstOption.click();
    }

    await this.clickContinueButton();

    if (await this.isLocatorVisible(this.organizationUnitHeading)) {
      await this.clickContinueButton();
    }

    if (await this.isLocatorVisible(this.createUserActionButton)) {
      await this.createUserActionButton.first().click();
    }

    await this.waitForDetailsStep();
  }

  /** Fill the user form (Step 2: User Details) */
  async fillUserForm(data: UserFormData) {
    // Fill known fields by name/label
    if (
      await this.usernameInput
        .first()
        .isVisible()
        .catch(() => false)
    ) {
      await this.usernameInput.first().fill(data.username);
    }
    if (
      await this.emailInput
        .first()
        .isVisible()
        .catch(() => false)
    ) {
      await this.emailInput.first().fill(data.email);
    }
    if (
      data.given_name &&
      (await this.givenNameInput
        .first()
        .isVisible()
        .catch(() => false))
    ) {
      await this.givenNameInput.first().fill(data.given_name);
    }
    if (
      data.family_name &&
      (await this.familyNameInput
        .first()
        .isVisible()
        .catch(() => false))
    ) {
      await this.familyNameInput.first().fill(data.family_name);
    }

    // Fill any remaining empty required text/password inputs with generated values
    // (dynamic schema fields that aren't covered by the known field locators)
    const requiredInputs = this.page.locator('input[required]:not([type="checkbox"]):not([type="radio"])');
    const count = await requiredInputs.count();
    for (let i = 0; i < count; i++) {
      const input = requiredInputs.nth(i);
      const currentValue = await input.inputValue();
      if (!currentValue) {
        const name = (await input.getAttribute("name")) ?? `field_${i}`;
        const type = await input.getAttribute("type");
        const value = type === "password" ? `Test@${Date.now()}` : `test_${name}_${Date.now()}`;
        await input.fill(value);
      }
    }
  }

  /** Submit the form (clicks "Create User" on the last step) */
  async submitForm() {
    await expect(this.submitButton.first()).toBeEnabled({ timeout: Timeouts.ELEMENT_VISIBILITY });
    await this.submitButton.first().click();
  }

  private async clickContinueButton() {
    await this.continueButton.first().waitFor({ state: "visible", timeout: Timeouts.ELEMENT_VISIBILITY });
    await this.continueButton.first().click();
  }

  private async waitForDetailsStep() {
    await this.waitForAnyVisibleLocator(
      [this.formHeading, this.usernameInput, this.emailInput, this.givenNameInput, this.familyNameInput, this.passwordInput],
      Timeouts.FORM_LOAD,
    );
  }

  private async isLocatorVisible(locator: Locator): Promise<boolean> {
    return locator.first().isVisible();
  }

  private async waitForAnyVisibleLocator(locators: Locator[], timeout: number) {
    try {
      await Promise.any(
        locators.map((locator) => locator.first().waitFor({ state: "visible", timeout })),
      );
    } catch {
      throw new Error(`Timed out after ${timeout}ms while waiting for the next visible user-creation step.`);
    }
  }

  /** Cancel the form */
  async cancelForm() {
    await this.cancelButton.first().click();
  }

  /** Create a new user (complete wizard flow) */
  async createUser(data: UserFormData) {
    await this.clickAddUser();
    await this.waitForUserForm();
    await this.selectUserTypeAndContinue();
    await this.fillUserForm(data);
    await this.submitForm();
  }

  /** Search for a user */
  async searchUser(query: string) {
    await this.searchInput.first().fill(query);
    // Using network idle after triggering search.
    // This is acceptable here because the users page is expected not to keep long-lived
    // connections (e.g., websockets) and search is the primary network activity.
    // If additional long-running requests are introduced, prefer a more targeted wait
    // such as page.waitForResponse() for the search API or waiting for the results
    // table locator to update instead of relying on 'networkidle'.
    await this.page.waitForLoadState("networkidle");
  }

  /** Get user count */
  async getUserCount(): Promise<number> {
    const rows = this.page.locator('table tbody tr, [role="row"]');
    return await rows.count();
  }
}
