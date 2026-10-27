// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * App Builder Page Object Model
 *
 * Encapsulates the Console's Build > Admin App and Build > Account App builders: the page outline
 * with its on/off switches, the properties panel, the appearance settings, undo and save.
 *
 * @example
 * const builder = new AppBuilderPage(page, baseUrl);
 * await builder.goto("ADMIN");
 * await builder.setPageShown("Groups", false);
 * await builder.save();
 */

import { Page, Locator, expect } from "@playwright/test";
import { ConsoleRoutes } from "../../configs/routes/console-routes";
import { BasePage } from "../base.page";
import { Timeouts } from "../../constants/timeouts";

export type BuiltAppType = "ADMIN" | "ACCOUNT";

export class AppBuilderPage extends BasePage {
  readonly baseUrl: string;

  readonly saveButton: Locator;
  readonly undoButton: Locator;
  readonly preview: Locator;
  readonly appLayoutItem: Locator;

  constructor(page: Page, baseUrl: string) {
    super(page);
    this.baseUrl = baseUrl;

    this.saveButton = page.getByRole("button", { name: "Save", exact: true });
    this.undoButton = page.getByRole("button", { name: "Undo" });
    this.preview = page.getByRole("region", { name: /App preview/ });
    this.appLayoutItem = page.getByRole("button", { name: /^App layout/ });
  }

  /** Open the builder of the given app and wait for its outline to render. */
  async goto(type: BuiltAppType) {
    const route = type === "ADMIN" ? ConsoleRoutes.adminAppBuilder : ConsoleRoutes.accountAppBuilder;
    await this.page.goto(`${this.baseUrl}${route}`, { timeout: Timeouts.PAGE_LOAD });
    await expect(this.saveButton).toBeVisible({ timeout: Timeouts.ELEMENT_VISIBILITY });
  }

  /** The outline switch that shows or hides a page, by the page's label (e.g. "Groups"). */
  pageSwitch(pageLabel: string): Locator {
    return this.page.getByRole("checkbox", { name: `Show ${pageLabel} in the app` });
  }

  /** Turn a page on or off in the outline. A no-op when it is already in that state. */
  async setPageShown(pageLabel: string, shown: boolean) {
    const toggle = this.pageSwitch(pageLabel);
    if ((await toggle.isChecked()) !== shown) {
      await toggle.click();
    }
  }

  /** Select a page in the outline so the properties panel and preview show it. */
  async selectPage(pageLabel: string) {
    await this.page.getByRole("button", { name: new RegExp(`^${pageLabel}`) }).click();
  }

  /** Choose the navigation layout from the appearance settings. */
  async setNavigation(layout: "Top bar" | "Side bar") {
    await this.appLayoutItem.click();
    await this.page.getByRole("button", { name: layout, exact: true }).click();
  }

  /** Save when there are unsaved changes. */
  async saveIfChanged() {
    if (await this.saveButton.isEnabled()) {
      await this.save();
    }
  }

  /** Save, and wait for the server to accept the configuration. */
  async save() {
    const saved = this.page.waitForResponse(
      response => response.url().includes("/app-configs/") && response.request().method() === "PUT"
    );
    await this.saveButton.click();
    expect((await saved).status()).toBe(200);
    await expect(this.saveButton).toBeDisabled();
  }
}
