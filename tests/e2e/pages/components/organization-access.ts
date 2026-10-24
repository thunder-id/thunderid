// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import { Page, Locator } from "@playwright/test";
import { Timeouts } from "../../constants/timeouts";

const escape = (text: string): string => text.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");

/**
 * The shared Organization Access screen (`OrganizationAccessSettings` from
 * `@thunderid/configure-resource-sharing`), which any shareable resource mounts as a tab: users
 * and applications so far. It has no test ids, so it is driven by its radio groups, buttons and
 * dialogs, the same accessible names a screen reader would use. Each choice card's radio is named
 * by its title followed by its description, so choices are matched on the leading title.
 */
export class OrganizationAccessSettings {
  readonly scopeGroup: Locator;
  readonly childUnitsGroup: Locator;
  readonly grantChildUnitsButton: Locator;
  readonly childUnitsDialog: Locator;

  constructor(readonly page: Page) {
    // Only the active tab panel is un-hidden, so this scopes every lookup to the open tab.
    const panel = page.locator('[role="tabpanel"]:not([hidden])');
    this.scopeGroup = panel.getByRole("radiogroup", { name: /^Who can use / });
    this.childUnitsGroup = panel.getByRole("radiogroup", { name: /^Child organization units of / });
    this.grantChildUnitsButton = panel.getByRole("button", { name: "Grant access", exact: true });
    this.childUnitsDialog = page.getByRole("dialog", { name: "Grant access to child units" });
  }

  /** A choice in the "Who can use" group, e.g. `Only Default` or `Chosen organization units`. */
  scopeChoice(title: string | RegExp): Locator {
    return this.scopeGroup.getByRole("radio", {
      name: typeof title === "string" ? new RegExp(`^${escape(title)}`) : title,
    });
  }

  /** A choice in the child units group: `None`, `All child units` or `Selected child units`. */
  childUnitsChoice(title: string): Locator {
    return this.childUnitsGroup.getByRole("radio", { name: new RegExp(`^${escape(title)}`) });
  }

  /** A granted unit's row, found by its revoke button, which only a granted unit has. */
  grantedUnit(name: string): Locator {
    return this.page.getByRole("button", { name: `Revoke access for ${name}`, exact: true });
  }

  // Choices are clicked, not checked: a choice that clears settings asks for confirmation first,
  // and its radio only changes once that is confirmed.
  async chooseScope(title: string | RegExp) {
    const choice = this.scopeChoice(title);
    await choice.waitFor({ state: "visible", timeout: Timeouts.ELEMENT_VISIBILITY });
    await choice.click();
  }

  async chooseChildUnits(title: string) {
    const choice = this.childUnitsChoice(title);
    await choice.waitFor({ state: "visible", timeout: Timeouts.ELEMENT_VISIBILITY });
    await choice.click();
  }

  /** Grant child units by name through the picker, setting each one's reach (the picker starts at "And below"). */
  async grantChildUnits(units: { name: string; reach: "This unit only" | "And below" }[]) {
    await this.grantChildUnitsButton.click();
    await this.childUnitsDialog.waitFor({ state: "visible", timeout: Timeouts.ELEMENT_VISIBILITY });
    for (const { name, reach } of units) {
      // Each picker row is a label holding the checkbox, the unit's name and its handle.
      await this.childUnitsDialog.getByRole("checkbox", { name: new RegExp(`^${escape(name)}`) }).check();
      await this.childUnitsDialog
        .getByRole("group", { name: `Reach for ${name}`, exact: true })
        .getByRole("button", { name: reach, exact: true })
        .click();
    }
    await this.childUnitsDialog.getByRole("button", { name: "Grant access", exact: true }).click();
    await this.childUnitsDialog.waitFor({ state: "hidden", timeout: Timeouts.ELEMENT_VISIBILITY });
  }

  /** Confirm a switch that clears settings, by the confirmation's action label, e.g. `Stop sharing`. */
  async confirmSwitch(actionLabel: string) {
    const dialog = this.page
      .getByRole("dialog")
      .filter({ has: this.page.getByRole("button", { name: actionLabel, exact: true }) });
    await dialog.waitFor({ state: "visible", timeout: Timeouts.ELEMENT_VISIBILITY });
    await dialog.getByRole("button", { name: actionLabel, exact: true }).click();
    await dialog.waitFor({ state: "hidden", timeout: Timeouts.ELEMENT_VISIBILITY });
  }
}
