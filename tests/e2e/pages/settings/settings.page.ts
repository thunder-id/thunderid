// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Settings Page Object Model (CORS allowed origins)
 *
 * Encapsulates the Settings > CORS panel: adding/removing custom allowed origins (as exact origins
 * or as regex patterns) and saving.
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

// Marks each editable (custom) origin row. Read-only rows carry no test id.
const ROW_TEST_ID = "cors-origin-row";

export class SettingsPage extends BasePage {
  readonly baseUrl: string;

  readonly corsTab: Locator;
  readonly addOriginButton: Locator;
  // Editable (custom) origin rows. Each row owns its own value field, type selector, and remove
  // button, so controls are always resolved within a row rather than by position across the page.
  readonly originRows: Locator;
  readonly unsavedChangesBar: UnsavedChangesBar;

  constructor(page: Page, baseUrl: string) {
    super(page);
    this.baseUrl = baseUrl;

    this.corsTab = page.getByRole("tab", { name: /cors/i });
    this.addOriginButton = page.getByRole("button", { name: /add origin/i });
    this.originRows = page.locator(`[data-componentid="${ROW_TEST_ID}"]`);
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

  /**
   * The value field of the given editable row. Resolved by role rather than by position: the row's
   * type selector renders its own `aria-hidden` native input ahead of this one, so the first `input`
   * in the row is the selector's, not the field the test means to type into.
   */
  private valueField(row: Locator): Locator {
    return row.getByRole("textbox");
  }

  /**
   * The editable row holding the given value, whether it is an exact origin or a pattern, or
   * `undefined` when no row holds it. Values are compared as strings rather than interpolated into a
   * selector, so a pattern's backslashes stay literal instead of being read as selector escapes.
   */
  private async rowFor(value: string): Promise<Locator | undefined> {
    for (const row of await this.originRows.all()) {
      if ((await this.valueField(row).inputValue()) === value) {
        return row;
      }
    }
    return undefined;
  }

  /** Whether a custom (editable) entry with the given value is currently listed. */
  async hasCustomOrigin(value: string): Promise<boolean> {
    return (await this.rowFor(value)) !== undefined;
  }

  /** Add a custom allowed origin and persist it. */
  async addAllowedOrigin(origin: string) {
    await this.addOriginButton.click();
    const field = this.valueField(this.originRows.last());
    await field.fill(origin);
    await field.blur();
    await this.unsavedChangesBar.save();
  }

  /** Add a custom allowed origin as a regex pattern and persist it. */
  async addAllowedOriginRegex(pattern: string) {
    await this.addOriginButton.click();
    const row = this.originRows.last();
    await row.getByRole("combobox", { name: /entry type/i }).click();
    await this.page.getByRole("option", { name: /^regex$/i }).click();
    const field = this.valueField(row);
    await field.fill(pattern);
    await field.blur();
    await this.unsavedChangesBar.save();
  }

  /** Remove a custom allowed entry (no-op if absent) and persist. */
  async removeAllowedOrigin(value: string) {
    const row = await this.rowFor(value);
    if (row === undefined) {
      return;
    }
    await row.getByRole("button", { name: /remove origin/i }).click();
    await this.unsavedChangesBar.save();
  }
}
