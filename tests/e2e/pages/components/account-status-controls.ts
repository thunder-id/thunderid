// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import { Page, Locator } from "@playwright/test";
import { Timeouts } from "../../constants/timeouts";
import { UIMessages } from "../../constants/ui-messages";

const { actions, status } = UIMessages.accountStatus;

/** Matches exactly one of the given labels. */
const oneOf = (labels: readonly string[]): RegExp => new RegExp(`^(${labels.join("|")})$`);

/**
 * The lifecycle controls a user or agent details page shares: a button per action the record's
 * status allows (Suspend, Unsuspend, Unlock), the inline confirmation Suspend and Unsuspend ask for,
 * and the status the page shows. Each details page object composes one.
 */
export class AccountStatusControls {
  readonly main: Locator;
  /** The lifecycle buttons the page offers, in page order. Only rendered once the flows resolve. */
  readonly actionButtons: Locator;
  /** The inline confirmation an action asks for; absent until one is chosen. */
  readonly confirmation: Locator;
  readonly operatorNoteField: Locator;
  /** The record's access status: Active, Locked or Suspended. */
  readonly status: Locator;

  constructor(readonly page: Page) {
    this.main = page.getByRole("main");
    this.actionButtons = this.main.getByRole("button", { name: oneOf(Object.values(actions)) });
    this.confirmation = this.main
      .getByRole("group")
      .filter({ has: page.getByRole("button", { name: UIMessages.accountStatus.cancel, exact: true }) });
    this.operatorNoteField = this.confirmation.getByLabel(UIMessages.accountStatus.operatorNoteLabel);
    this.status = this.main.getByText(oneOf(Object.values(status)), { exact: true });
  }

  /** Wait for the record and its lifecycle flows to load, which renders the first lifecycle button. */
  async waitForReady(): Promise<void> {
    await this.actionButtons.first().waitFor({ state: "visible", timeout: Timeouts.PAGE_LOAD });
  }

  /** The button for an action, outside its confirmation. */
  action(label: string): Locator {
    return this.actionButtons.filter({ hasText: oneOf([label]) });
  }

  /** Choose an action. Unlock runs at once; Suspend and Unsuspend ask for confirmation first. */
  async chooseAction(label: string): Promise<void> {
    await this.action(label).first().click();
  }

  /** Confirm the open confirmation with its action button. */
  async confirm(label: string): Promise<void> {
    await this.confirmation.getByRole("button", { name: label, exact: true }).click();
  }

  async cancel(): Promise<void> {
    await this.confirmation.getByRole("button", { name: UIMessages.accountStatus.cancel, exact: true }).click();
  }
}
