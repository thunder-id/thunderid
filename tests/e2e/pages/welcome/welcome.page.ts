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

import { Page, Locator, expect, Download } from "@playwright/test";
import { ConsoleRoutes } from "../../configs/routes/console-routes";
import { BasePage } from "../base.page";
import { Timeouts } from "../../constants/timeouts";

const WAYFINDER_ASSET_PATTERN = /^sample-app-wayfinder-[0-9A-Za-z.+-]+\.zip$/i;

export class WelcomePage extends BasePage {
  readonly baseUrl: string;

  // Welcome landing
  readonly securedWebAppTile: Locator;

  // Tryout page - sample setup
  readonly sampleSetupHeader: Locator;
  readonly downloadLink: Locator;
  readonly importConfigButton: Locator;
  readonly importSuccessMessage: Locator;
  readonly importAlreadyDoneMessage: Locator;

  // Tryout page - scenario tabs
  readonly signInTab: Locator;
  readonly signUpTab: Locator;

  constructor(page: Page, baseUrl: string) {
    super(page);
    this.baseUrl = baseUrl;

    this.securedWebAppTile = page.getByRole("button", { name: /Secured Web Application/i });

    this.sampleSetupHeader = page.getByRole("button", { name: /Setup Wayfinder Sample/i });
    this.downloadLink = page.getByRole("link", { name: /^Download$/ });
    this.importConfigButton = page.getByRole("button", { name: /^Configure in\s+\S+/i });
    this.importSuccessMessage = page.getByText(/Wayfinder sample configured in .* successfully/i);
    this.importAlreadyDoneMessage = page.getByText(/Wayfinder sample already configured/i);

    this.signInTab = page.getByRole("tab", { name: /^Sign-In$/i });
    this.signUpTab = page.getByRole("tab", { name: /^Self Sign-Up$/i });
  }

  /**
   * Navigate directly to the welcome landing page.
   * Clears the dismissed-welcome flag so the page renders even after prior visits.
   */
  async goto(): Promise<void> {
    await this.page.goto(`${this.baseUrl}${ConsoleRoutes.home}`, {
      waitUntil: "domcontentloaded",
      timeout: Timeouts.PAGE_LOAD,
    });
    await this.page.evaluate(() => {
      const keys: string[] = [];
      for (let i = 0; i < sessionStorage.length; i++) {
        const k = sessionStorage.key(i);
        if (
          k &&
          (k.endsWith(":welcome:dismissed") ||
            k.endsWith(":wayfinder-config-imported") ||
            k.endsWith(":wayfinder-setup-expanded"))
        ) {
          keys.push(k);
        }
      }
      keys.forEach(k => sessionStorage.removeItem(k));
    });
    await this.page.goto(`${this.baseUrl}${ConsoleRoutes.welcome}`, {
      waitUntil: "networkidle",
      timeout: Timeouts.PAGE_LOAD,
    });
  }

  async verifyWelcomeLoaded(): Promise<void> {
    expect(this.page.url()).toContain(ConsoleRoutes.welcome);
    await this.securedWebAppTile.first().waitFor({ state: "visible", timeout: Timeouts.ELEMENT_VISIBILITY });
  }

  async clickSecuredWebAppTile(): Promise<void> {
    await this.securedWebAppTile.first().click();
    await this.page.waitForURL(new RegExp(`${ConsoleRoutes.welcomeTryoutApp}$`), { timeout: Timeouts.PAGE_LOAD });
  }

  async verifyTryoutPageLoaded(): Promise<void> {
    expect(this.page.url()).toContain(ConsoleRoutes.welcomeTryoutApp);
    await this.sampleSetupHeader.first().waitFor({ state: "visible", timeout: Timeouts.ELEMENT_VISIBILITY });
  }

  /**
   * Click the wayfinder sample download button and assert a download starts.
   * Returns the suggested filename so the caller can verify it matches the expected pattern.
   */
  async triggerDownload(): Promise<Download> {
    await this.downloadLink.first().waitFor({ state: "visible", timeout: Timeouts.ELEMENT_VISIBILITY });
    await expect(this.downloadLink.first()).toHaveAttribute("href", /sample-app-wayfinder-.+\.zip/i);

    // The download button is an anchor with target="_blank". Playwright fires the
    // download event on the browser context regardless of the target tab.
    const [download] = await Promise.all([
      this.page.context().waitForEvent("download", { timeout: 30000 }),
      this.downloadLink.first().click(),
    ]);
    expect(download.suggestedFilename()).toMatch(WAYFINDER_ASSET_PATTERN);
    return download;
  }

  /**
   * Trigger the Configure/Import action for the wayfinder bundle and wait for success.
   * Accepts either the "success" or "alreadyDone" terminal state.
   */
  async importWayfinderBundle(): Promise<void> {
    // If a previous run left the bundle configured, the button is replaced with the alreadyDone state.
    if (
      await this.importAlreadyDoneMessage
        .first()
        .isVisible()
        .catch(() => false)
    ) {
      return;
    }
    await this.importConfigButton.first().waitFor({ state: "visible", timeout: Timeouts.ELEMENT_VISIBILITY });
    await expect(this.importConfigButton.first()).toBeEnabled({ timeout: Timeouts.ELEMENT_VISIBILITY });
    await this.importConfigButton.first().click();
    await expect(this.importSuccessMessage.first().or(this.importAlreadyDoneMessage.first())).toBeVisible({
      timeout: 30000,
    });
  }

  async selectSignInTab(): Promise<void> {
    await this.signInTab.first().click();
  }

  async selectSignUpTab(): Promise<void> {
    await this.signUpTab.first().click();
  }
}
