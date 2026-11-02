// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * A minimal page object for the Wayfinder sample and the ThunderID gate it logs in through.
 *
 * tests/e2e has richer page objects for the same screens, but this suite is a separate workspace
 * package and reaching across that boundary would couple two independently installed dependency
 * trees. The selectors here are deliberately the same ones, so the two stay recognisable:
 * see tests/e2e/pages/wayfinder-sample/wayfinder-app.page.ts and pages/gate-login.page.ts.
 */

import { Page, expect } from "@playwright/test";
import { Timeouts } from "../constants/timeouts";

/** Where the CLI serves the sample. Fixed: the bundle's redirect URIs hardcode this origin. */
export const WAYFINDER_URL = "http://localhost:5173";

export class WayfinderPage {
  constructor(private readonly page: Page) {}

  async goto(): Promise<void> {
    await this.page.goto(WAYFINDER_URL, { waitUntil: "domcontentloaded" });
  }

  /** Signs in through the gate and waits for the redirect back to the sample. */
  async signIn(username: string, password: string): Promise<void> {
    const signIn = this.page.getByRole("button", { name: /^sign in$/i });
    await signIn.waitFor({ state: "visible", timeout: Timeouts.BROWSER });

    // The sample is a freshly started Vite dev server, so the button is painted before React has
    // attached its handler and an early click is silently dropped. Retry until the navigation to
    // the gate actually happens rather than assuming the first click takes.
    await expect(async () => {
      await signIn.click();
      // "commit" rather than the default "load": the gate redirects again to add
      // showInsecureWarning, so the load event does not settle while the URL is already right.
      await this.page.waitForURL(/\/gate\/signin/, { timeout: 10_000, waitUntil: "commit" });
    }).toPass({ timeout: Timeouts.BROWSER });

    const usernameInput = this.page.locator('input[name="username"], input[placeholder*="username" i]').first();
    await usernameInput.waitFor({ state: "visible", timeout: Timeouts.BROWSER });
    await usernameInput.fill(username);

    const passwordInput = this.page.locator('input[name="password"], input[placeholder*="password" i]').first();
    await passwordInput.fill(password);

    const submit = this.page
      .locator('button[type="submit"], button:has-text("Sign In"), button:has-text("Sign in")')
      .first();

    // Wait for the redirect off the gate as part of the click, so a rejected login fails here
    // rather than later as a confusing missing-account-menu error.
    await Promise.all([
      this.page.waitForURL(url => url.origin === WAYFINDER_URL, {
        timeout: Timeouts.BROWSER,
        waitUntil: "commit",
      }),
      submit.click(),
    ]);
  }

  /** The account menu only renders for a signed-in session. */
  async expectSignedIn(): Promise<void> {
    const accountMenu = this.page.locator('button[aria-haspopup="menu"]').first();
    await expect(accountMenu).toBeVisible({ timeout: Timeouts.BROWSER });
  }
}
