// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Settings Page Object Model (CORS allowed origins)
 *
 * Encapsulates the Settings > CORS panel: adding/removing custom allowed origins and saving.
 *
 * @example
 * const settingsPage = new SettingsPage(page, baseUrl);
 * await settingsPage.goto();
 * await settingsPage.addAllowedOrigin("https://app.example.com");
 */

import { Page, Locator } from "@playwright/test";
import { ConsoleRoutes } from "../../configs/routes/console-routes";
import { BasePage } from "../base.page";
import { UnsavedChangesBar } from "../components/unsaved-changes-bar";
import { Timeouts } from "../../constants/timeouts";

// Matches the placeholder rendered on each editable (custom) origin input.
const ORIGIN_PLACEHOLDER = "https://app.example.com";

export class SettingsPage extends BasePage {
  readonly baseUrl: string;

  readonly corsTab: Locator;
  readonly addOriginButton: Locator;
  // Editable (custom) origin inputs and their delete buttons render in the same row order,
  // so the Nth input aligns with the Nth remove button. Read-only rows carry neither.
  readonly originInputs: Locator;
  readonly removeButtons: Locator;
  readonly unsavedChangesBar: UnsavedChangesBar;

  constructor(page: Page, baseUrl: string) {
    super(page);
    this.baseUrl = baseUrl;

    this.corsTab = page.getByRole("tab", { name: /cors/i });
    this.addOriginButton = page.getByRole("button", { name: /add origin/i });
    this.originInputs = page.getByPlaceholder(ORIGIN_PLACEHOLDER);
    this.removeButtons = page.getByRole("button", { name: /remove origin/i });
    // CorsSection passes saveLabel={t('settings:cors.save', 'Save changes')} and
    // resetLabel={t('settings:cors.reset', 'Reset')}.
    this.unsavedChangesBar = new UnsavedChangesBar(page, "Save changes", "Reset");
  }

  /** Navigate to the Settings (CORS) page. The waitFor below is the real readiness gate, so the
   *  navigation itself doesn't need to also wait for network idle. */
  async goto() {
    await this.page.goto(`${this.baseUrl}${ConsoleRoutes.settings}`, {
      timeout: Timeouts.PAGE_LOAD,
    });
    await this.corsTab.first().waitFor({ state: "visible", timeout: Timeouts.ELEMENT_VISIBILITY });
  }

  /** Index of the editable row holding the given origin, or -1 if absent. */
  private async indexOfOrigin(origin: string): Promise<number> {
    const count = await this.originInputs.count();
    for (let i = 0; i < count; i++) {
      if ((await this.originInputs.nth(i).inputValue()) === origin) {
        return i;
      }
    }
    return -1;
  }

  /** Whether a custom (editable) origin with the given value is currently listed. */
  async hasCustomOrigin(origin: string): Promise<boolean> {
    return (await this.indexOfOrigin(origin)) !== -1;
  }

  /** Add a custom allowed origin and persist it. */
  async addAllowedOrigin(origin: string) {
    await this.addOriginButton.click();
    const input = this.originInputs.last();
    await input.fill(origin);
    await input.blur();
    await this.unsavedChangesBar.save();
  }

  /** Remove a custom allowed origin (no-op if absent) and persist. */
  async removeAllowedOrigin(origin: string) {
    const index = await this.indexOfOrigin(origin);
    if (index === -1) {
      return;
    }
    await this.removeButtons.nth(index).click();
    await this.unsavedChangesBar.save();
  }
}
