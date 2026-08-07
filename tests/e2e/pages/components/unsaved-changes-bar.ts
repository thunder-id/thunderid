// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import { Page, Locator, expect } from "@playwright/test";
import { Timeouts } from "../../constants/timeouts";

/**
 * The frontend's UnsavedChangesBar, shown when a page's form has unsaved changes. Its save and
 * reset buttons' accessible names are per-page `saveLabel`/`resetLabel` props on the frontend
 * component, not fixed strings across the console (e.g. saveLabel is "Save changes" on
 * Settings/Connections but plain "Save" on User Types/Agent Types) - so each page composes this
 * with its own actual labels rather than every page guessing at one shared string.
 */
export class UnsavedChangesBar {
  readonly saveButton: Locator;
  readonly resetButton: Locator;

  constructor(page: Page, saveLabel: string, resetLabel: string) {
    this.saveButton = page.getByRole("button", { name: saveLabel, exact: true });
    this.resetButton = page.getByRole("button", { name: resetLabel, exact: true });
  }

  /** Click save and wait for the bar to clear (success). */
  async save(): Promise<void> {
    await expect(this.saveButton).toBeEnabled({ timeout: Timeouts.ELEMENT_VISIBILITY });
    await this.saveButton.click();
    await expect(this.saveButton).toBeHidden({ timeout: Timeouts.ELEMENT_VISIBILITY });
  }

  /** Click reset (discards the pending edits) and wait for the bar to clear. */
  async reset(): Promise<void> {
    await expect(this.resetButton).toBeEnabled({ timeout: Timeouts.ELEMENT_VISIBILITY });
    await this.resetButton.click();
    await expect(this.resetButton).toBeHidden({ timeout: Timeouts.ELEMENT_VISIBILITY });
  }
}
