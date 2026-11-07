// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import { Page, Locator } from "@playwright/test";
import { Timeouts } from "../../constants/timeouts";
import { UIMessages } from "../../constants/ui-messages";

/**
 * The lifecycle controls a user, agent or application details page shares: the header's Actions
 * menu (suspend, unsuspend, unsuspend and unlock, unlock), its confirmation dialog, the status
 * banner above the tabs, and the toast an operation ends with. Each details page object composes
 * one, the same way pages compose the UnsavedChangesBar.
 */
export class AccountStatusControls {
  /** Renders only once the lifecycle flows resolve, so it doubles as the page's readiness signal. */
  readonly actionsButton: Locator;
  /** Every operation the open Actions menu offers. */
  readonly menuItems: Locator;
  readonly dialog: Locator;
  readonly dialogTitle: Locator;
  readonly operatorNoteField: Locator;
  /** The banner a suspended or locked record shows; absent while the record is active. */
  readonly statusBanner: Locator;

  constructor(readonly page: Page) {
    this.actionsButton = page.getByTestId("account-status-actions");
    this.menuItems = page.getByRole("menuitem");
    this.dialog = page.getByRole("dialog");
    this.dialogTitle = this.dialog.locator("h2");
    this.operatorNoteField = this.dialog.getByLabel(UIMessages.accountStatus.operatorNoteLabel);
    this.statusBanner = page.getByTestId("access-status-banner");
  }

  /** Wait for the Actions menu, which appears once the record and its lifecycle flows have loaded. */
  async waitForReady(): Promise<void> {
    await this.actionsButton.waitFor({ state: "visible", timeout: Timeouts.PAGE_LOAD });
  }

  /**
   * The menu entry for an operation. Matched on its label alone, since each entry's accessible name
   * also carries its hint, and "Unsuspend" is a prefix of "Unsuspend and unlock".
   */
  menuItem(label: string): Locator {
    return this.menuItems.filter({ has: this.page.getByText(label, { exact: true }) });
  }

  async openActions(): Promise<void> {
    await this.actionsButton.click();
    await this.menuItems.first().waitFor({ state: "visible", timeout: Timeouts.ELEMENT_VISIBILITY });
  }

  /** Close the Actions menu without choosing anything. */
  async closeActions(): Promise<void> {
    await this.page.keyboard.press("Escape");
    await this.menuItems.first().waitFor({ state: "hidden", timeout: Timeouts.ELEMENT_VISIBILITY });
  }

  /** The visible labels of every operation the Actions menu offers, in menu order. */
  async offeredActions(): Promise<string[]> {
    await this.openActions();
    const labels: string[] = [];
    for (const item of await this.menuItems.all()) {
      // The label is the entry's primary text; the hint is its secondary text.
      labels.push(((await item.locator(".MuiListItemText-primary").textContent()) ?? "").trim());
    }
    await this.closeActions();
    return labels;
  }

  /** Open the Actions menu and choose an operation. Unlock runs at once; the others open a dialog. */
  async chooseAction(label: string): Promise<void> {
    await this.openActions();
    await this.menuItem(label).click();
  }

  /** Confirm the open dialog with its primary button. */
  async confirm(label: string): Promise<void> {
    await this.dialog.getByRole("button", { name: label, exact: true }).click();
  }

  async cancel(): Promise<void> {
    await this.dialog.getByRole("button", { name: UIMessages.accountStatus.cancel, exact: true }).click();
  }

  /**
   * The toast whose message starts with the given text. The flows' email nodes are best-effort, so
   * a deployment without an email sender toasts a warning that starts with the same words.
   */
  toast(startsWith: string): Locator {
    const escaped = startsWith.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
    return this.page.getByRole("alert").filter({ hasText: new RegExp(`^${escaped}`) });
  }
}
