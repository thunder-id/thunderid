// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import { Page, Locator, expect } from "@playwright/test";
import { BasePage } from "../base.page";
import { Timeouts } from "../../constants/timeouts";

/**
 * Mock Email App Page Object
 *
 * Interacts with the local mock email service (e.g., Mailcrab or a custom SMTP inbox UI)
 * running on port 8788 for the Wayfinder sample app.
 */
export class MockEmailAppPage extends BasePage {
  constructor(page: Page) {
    super(page);
  }

  /**
   * Navigate to the mock email service
   * @param url - Mock email service URL
   */
  async goto(url: string = "http://localhost:8788/") {
    await this.page.goto(url, { waitUntil: "commit" });
  }

  /**
   * Wait for an email with the specified subject to appear in the inbox and click it.
   * @param subject - The subject line to search for (can be a string or regex)
   */
  async openEmailBySubject(subject: string | RegExp) {
    const emailItem = this.page.getByText(subject).first();
    await emailItem.waitFor({ state: "visible", timeout: Timeouts.DEFAULT_ACTION });
    await emailItem.click();
  }

  /**
   * Click a link in the email body containing the specified text.
   * Returns a promise that resolves to the new page opened by the link.
   * @param linkText - The text of the link to click (can be a string or regex)
   * @returns The new page that opens after clicking the link
   */
  async clickLinkInEmail(linkText: string | RegExp): Promise<Page> {
    const link = this.page.getByRole("link", { name: linkText }).first();
    await link.waitFor({ state: "visible", timeout: Timeouts.DEFAULT_ACTION });
    
    const pagePromise = this.page.context().waitForEvent("page");
    await link.click();
    return await pagePromise;
  }
}
