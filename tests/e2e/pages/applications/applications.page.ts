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

import { Page, Locator, expect } from "@playwright/test";
import { ConsoleRoutes } from "../../configs/routes/console-routes";
import { BasePage } from "../base.page";
import { Timeouts } from "../../constants/timeouts";

export type ApplicationFormData = {
  name: string;
  redirectUri?: string;
};

export class ApplicationsPage extends BasePage {
  readonly baseUrl: string;

  // List page
  readonly addApplicationButton: Locator;
  readonly applicationsList: Locator;
  readonly searchInput: Locator;

  // Create wizard — shared
  readonly nextButton: Locator;
  readonly backButton: Locator;
  readonly errorAlert: Locator;

  // Create wizard — steps
  readonly configureNameStep: Locator;
  readonly appNameInput: Locator;
  readonly configureOrganizationUnitStep: Locator;
  readonly configureDesignStep: Locator;
  readonly configureSignInStep: Locator;
  readonly configureExperienceStep: Locator;
  readonly configureStackStep: Locator;
  readonly configureDetailsStep: Locator;
  readonly inbuiltExperienceCard: Locator;
  readonly embeddedExperienceCard: Locator;
  readonly showClientSecretStep: Locator;
  readonly clientSecretValue: Locator;
  readonly clientSecretContinue: Locator;

  // Edit page — shared
  readonly saveButton: Locator;

  // Edit page — General tab
  readonly applicationIdField: Locator;
  readonly clientIdField: Locator;
  readonly applicationUrlInput: Locator;
  readonly addUriButton: Locator;
  readonly deleteApplicationButton: Locator;

  constructor(page: Page, baseUrl: string) {
    super(page);
    this.baseUrl = baseUrl;

    // List page
    this.addApplicationButton = page.locator('[data-testid="application-add-button"]');

    this.applicationsList = page.locator('[data-testid="applications-list"]');
    this.searchInput = page
      .locator('input[placeholder*="Search" i]')
      .or(page.locator('[data-testid="applications-list"] input[type="text"]'));

    // Create wizard — shared
    this.nextButton = page
      .locator('[data-testid="application-wizard-next-button"]')
      .or(page.locator("button.MuiButton-contained").filter({ hasText: /continue/i }))
      .or(page.getByRole("button", { name: /^continue$/i }));

    this.backButton = page.getByRole("button", { name: /^back$/i }).or(page.locator('button:has-text("Back")'));

    this.errorAlert = page.locator(
      '[role="alert"].MuiAlert-standardError, [role="alert"].MuiAlert-filledError, [role="alert"].MuiAlert-outlinedError'
    );

    // Create wizard — steps
    this.configureNameStep = page.locator('[data-testid="application-configure-name"]');
    this.appNameInput = page.locator('[data-testid="app-name-input"]');
    this.configureOrganizationUnitStep = page.locator('[data-testid="application-configure-organization-unit"]');
    this.configureDesignStep = page.locator('[data-testid="application-configure-design"]');
    this.configureSignInStep = page.locator('[data-testid="application-configure-sign-in"]');
    this.configureExperienceStep = page.locator('[data-testid="application-configure-experience"]');
    this.configureStackStep = page.locator('[data-testid="application-configure-stack"]');
    this.configureDetailsStep = page.locator('[data-testid="application-configure-details"]');
    this.inbuiltExperienceCard = page.locator('div:has(input[value="INBUILT"])');
    this.embeddedExperienceCard = page.locator('div:has(input[value="EMBEDDED"])');
    this.showClientSecretStep = page.locator('[data-testid="application-show-client-secret"]');
    this.clientSecretValue = page.locator('[data-testid="application-client-secret-value"]');
    this.clientSecretContinue = page.locator('[data-testid="application-client-secret-continue"]');

    // Edit page — shared
    this.saveButton = page.getByRole("button", { name: /^save$/i }).or(page.locator('button:has-text("Save")'));

    // Edit page — General tab
    this.applicationIdField = page.locator("#application-id-input").or(page.getByLabel("Application ID"));
    this.clientIdField = page.getByLabel("Client ID");
    this.applicationUrlInput = page.locator("#application-url-input").or(page.getByLabel("Application URL"));
    this.addUriButton = page.getByRole("button", { name: /add uri/i }).or(page.locator('button:has-text("Add URI")'));
    this.deleteApplicationButton = page.locator('[data-testid="delete-application-button"]');
  }

  /** Navigate to the applications list page */
  async goto(): Promise<void> {
    await this.page.goto(`${this.baseUrl}${ConsoleRoutes.applications}`, {
      waitUntil: "networkidle",
      timeout: Timeouts.PAGE_LOAD,
    });
  }

  /** Navigate directly to an application's edit page */
  async gotoEdit(appId: string): Promise<void> {
    await this.page.goto(`${this.baseUrl}${ConsoleRoutes.applicationDetails(appId)}`, {
      waitUntil: "networkidle",
      timeout: Timeouts.PAGE_LOAD,
    });
  }

  /** Verify the applications list page is loaded */
  async verifyPageLoaded(): Promise<void> {
    const url = this.page.url();
    if (url.includes(ConsoleRoutes.signin)) {
      throw new Error("Authentication failed: Redirected to signin page");
    }
    expect(url).toContain(ConsoleRoutes.applications);
    await expect(this.applicationsList).toBeVisible({ timeout: Timeouts.ELEMENT_VISIBILITY });
    await this.page.waitForLoadState("networkidle");
  }

  /** Click the Add Application button to open the create wizard */
  async clickAddApplication(): Promise<void> {
    await this.addApplicationButton.first().waitFor({ state: "visible", timeout: Timeouts.ELEMENT_VISIBILITY });
    await this.addApplicationButton.first().scrollIntoViewIfNeeded();
    await this.addApplicationButton.first().click();
  }

  /** Type in the search box to filter the applications list */
  async searchApplications(name: string): Promise<void> {
    await this.searchInput.first().waitFor({ state: "visible", timeout: Timeouts.ELEMENT_VISIBILITY });
    await this.searchInput.first().fill(name);
    await this.page.waitForLoadState("networkidle");
  }

  /** Get a locator for an application row by its name */
  getApplicationRowByName(name: string): Locator {
    return this.page
      .locator('[role="gridcell"]')
      .filter({ hasText: name })
      .or(this.page.locator(".MuiDataGrid-cell").filter({ hasText: name }));
  }

  /** Wait for a wizard step to be visible by its data-testid */
  async waitForStep(testid: string, timeout: number = Timeouts.FORM_LOAD): Promise<void> {
    await this.page.locator(`[data-testid="${testid}"]`).waitFor({ state: "visible", timeout });
  }

  /** Fill the application name on Step 1 */
  async fillAppName(name: string): Promise<void> {
    await this.appNameInput.waitFor({ state: "visible", timeout: Timeouts.ELEMENT_VISIBILITY });
    await this.appNameInput.fill(name);
  }

  /** Click the Next / Continue button in the wizard */
  async clickNext(): Promise<void> {
    const btn = this.nextButton.first();
    await btn.waitFor({ state: "visible", timeout: Timeouts.ELEMENT_VISIBILITY });
    await btn.scrollIntoViewIfNeeded();
    await expect(btn).toBeEnabled({ timeout: Timeouts.ELEMENT_VISIBILITY });
    await btn.click();
  }

  /**
   * Handle the optional Organization Unit selection step that appears when the server has multiple OUs.
   * If the step is not visible it returns immediately; otherwise selects the first OU and advances.
   */
  async handleOptionalOuStep(): Promise<void> {
    try {
      await this.configureOrganizationUnitStep.waitFor({ state: "visible", timeout: 5000 });
    } catch {
      return;
    }
    // Wait for the tree to finish loading
    await this.page
      .locator('[data-testid="application-configure-organization-unit"] [role="progressbar"]')
      .waitFor({ state: "detached", timeout: Timeouts.ELEMENT_VISIBILITY })
      .catch(() => {});
    // Select the first real tree item (not a placeholder)
    const firstItem = this.configureOrganizationUnitStep
      .locator('[role="treeitem"]:not([aria-disabled="true"])')
      .first();
    await firstItem.waitFor({ state: "visible", timeout: Timeouts.ELEMENT_VISIBILITY });
    await firstItem.click();
    await this.clickNext();
  }

  /** Select a stack/technology card on the stack step by its visible title (e.g. "React", "Next.js"). */
  async selectStack(title: string): Promise<void> {
    const card = this.configureStackStep
      .locator('[role="button"]')
      .filter({ has: this.page.getByText(title, { exact: true }) })
      .first();
    await card.waitFor({ state: "visible", timeout: Timeouts.ELEMENT_VISIBILITY });
    await card.click();
  }

  /** Select the INBUILT experience option on the experience step (INBUILT is the default, only call if switching) */
  async selectInbuiltExperience(): Promise<void> {
    const inbuiltCard = this.inbuiltExperienceCard.first();
    await inbuiltCard.waitFor({ state: "visible", timeout: Timeouts.ELEMENT_VISIBILITY });
    await inbuiltCard.click();
  }

  /** Read the client secret value from the final wizard step */
  async getClientSecretValue(): Promise<string> {
    await this.clientSecretValue.waitFor({ state: "visible", timeout: Timeouts.ELEMENT_VISIBILITY });
    const nestedInput = this.clientSecretValue.locator("input");
    const inputCount = await nestedInput.count();
    if (inputCount > 0) {
      return nestedInput.first().inputValue();
    }
    return this.clientSecretValue.innerText();
  }

  /** Click the Continue / Done button on the client secret step */
  async clickDoneAfterCreate(): Promise<void> {
    await this.clientSecretContinue.waitFor({ state: "visible", timeout: Timeouts.ELEMENT_VISIBILITY });
    await this.clientSecretContinue.click();
    await this.page.waitForLoadState("networkidle");
  }

  /**
   * Wait for wizard completion after the last step is submitted.
   * Some app types show a client-secret screen; others navigate directly to the edit page.
   * Returns the final edit page URL.
   */
  async completeWizardCreation(): Promise<string> {
    const editUrlPattern = /\/console\/applications\/(?!create)[^/]+$/;
    await Promise.race([
      this.showClientSecretStep.waitFor({ state: "visible", timeout: 30000 }),
      this.page.waitForURL(editUrlPattern, { timeout: 30000 }),
    ]);
    if (editUrlPattern.test(this.page.url())) {
      return this.page.url();
    }
    await this.clickDoneAfterCreate();
    await this.page.waitForURL(editUrlPattern, { timeout: 15000 });
    return this.page.url();
  }

  /** Click a tab on the edit page by its visible label */
  async clickTab(tabName: string): Promise<void> {
    const tab = this.page.getByRole("tab", { name: new RegExp(tabName, "i") });
    await tab.waitFor({ state: "visible", timeout: Timeouts.ELEMENT_VISIBILITY });
    await tab.click();
  }

  /** Add a redirect / callback URI on the General tab */
  async addRedirectUri(uri: string): Promise<void> {
    await this.addUriButton.first().waitFor({ state: "visible", timeout: Timeouts.ELEMENT_VISIBILITY });
    await this.addUriButton.first().click();
    const newInput = this.page
      .locator('input[placeholder*="callback" i], input[placeholder*="uri" i], input[placeholder*="redirect" i]')
      .last();
    await newInput.waitFor({ state: "visible", timeout: Timeouts.ELEMENT_VISIBILITY });
    await newInput.fill(uri);
    await newInput.blur();
  }

  /** Click Save on the edit page */
  async saveChanges(): Promise<void> {
    await this.saveButton.first().waitFor({ state: "visible", timeout: Timeouts.ELEMENT_VISIBILITY });
    await this.saveButton.first().click();
    await this.page.waitForLoadState("networkidle");
  }

  /** Click the Delete Application button in the Danger Zone */
  async clickDeleteApplication(): Promise<void> {
    await this.deleteApplicationButton.first().waitFor({ state: "visible", timeout: Timeouts.ELEMENT_VISIBILITY });
    await this.deleteApplicationButton.first().scrollIntoViewIfNeeded();
    await this.deleteApplicationButton.first().click();
  }

  /** Confirm the delete dialog */
  async confirmDelete(): Promise<void> {
    const dialog = this.page.getByRole("dialog");
    await dialog.waitFor({ state: "visible", timeout: Timeouts.ELEMENT_VISIBILITY });
    const confirmButton = dialog
      .getByRole("button", { name: /^delete$/i })
      .or(dialog.getByRole("button", { name: /confirm/i }));
    await confirmButton.first().waitFor({ state: "visible", timeout: Timeouts.ELEMENT_VISIBILITY });
    await confirmButton.first().click();
    await this.page.waitForLoadState("networkidle");
  }
}
