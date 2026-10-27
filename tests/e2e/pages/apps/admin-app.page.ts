// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Admin App Page Object Model
 *
 * The Admin App the data plane serves at /admin. It is its own OAuth client, so opening it may
 * send the browser through the gate to sign in first.
 */

import { Page, Locator, expect } from "@playwright/test";
import { GateLoginPage } from "../gate-login.page";
import { Timeouts } from "../../constants/timeouts";

export class AdminAppPage extends GateLoginPage {
  readonly baseUrl: string;

  /** The Admin App's page navigation, in the top bar or beside the content. */
  readonly navigation: Locator;

  constructor(page: Page, baseUrl: string) {
    super(page);
    this.baseUrl = baseUrl;
    this.navigation = page.getByRole("navigation", { name: "Admin App" });
  }

  /** Open the Admin App, signing in through the gate when it asks. */
  async open(username: string, password: string) {
    await this.page.goto(`${this.baseUrl}/admin/`, { timeout: Timeouts.PAGE_LOAD });

    // The gate's username field. Search boxes are left out: the Admin App's user list has one for usernames.
    const usernameInput = this.page
      .locator('input[name="username"]:not([type="search"]), input[placeholder*="username" i]:not([type="search"])')
      .first();
    await expect(usernameInput.or(this.navigation).first()).toBeVisible({ timeout: Timeouts.PAGE_LOAD });
    if (await usernameInput.isVisible()) {
      await this.login(username, password);
    }

    await expect(this.navigation).toBeVisible({ timeout: Timeouts.PAGE_LOAD });
  }
}
