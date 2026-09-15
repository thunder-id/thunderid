// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import { Page, Locator, expect } from "@playwright/test";
import { ConsoleRoutes } from "../../configs/routes/console-routes";
import { BasePage } from "../base.page";
import { UnsavedChangesBar } from "../components/unsaved-changes-bar";
import { Timeouts } from "../../constants/timeouts";

export type BrandedConnectionFormData = {
  clientId: string;
  clientSecret: string;
};

/**
 * Page Object Model for the Connections feature (console admin UI).
 *
 * Branded vendors (Google, GitHub, ...) are singletons configured via a one-step
 * "configure" wizard with a fixed name; the connection name field is not rendered for them
 * (see frontend/packages/configure-connections/src/config/connectionVendorMeta.tsx,
 * `presentation: 'branded'`). The redirect URI field is server-derived and read-only.
 */
export class ConnectionsPage extends BasePage {
  readonly baseUrl: string;

  // Configure/create wizard (branded vendor, e.g. google)
  readonly clientIdInput: Locator;
  readonly clientSecretInput: Locator;
  readonly redirectUriField: Locator;
  readonly scopesInput: Locator;
  readonly wizardCreateButton: Locator;
  readonly wizardCreateError: Locator;

  // Detail page
  readonly deleteButton: Locator;
  readonly deleteConfirmButton: Locator;
  readonly unsavedChangesBar: UnsavedChangesBar;

  constructor(page: Page, baseUrl: string) {
    super(page);
    this.baseUrl = baseUrl;

    this.clientIdInput = page.locator("#connection-field-clientId").or(page.getByLabel("Client ID"));
    this.clientSecretInput = page.locator("#connection-field-clientSecret").or(page.getByLabel(/client secret/i));
    this.redirectUriField = page.locator("#connection-field-redirectUri").or(page.getByLabel(/redirect uri/i));
    this.scopesInput = page.locator("#connection-field-scopes").or(page.getByLabel("Scopes"));
    this.wizardCreateButton = page.locator('[data-testid="wizard-create"]');
    // Not getByRole('alert'): ConnectionCreateHint renders an info Alert on the same page.
    this.wizardCreateError = page.locator('[data-testid="wizard-create-error"]');

    this.deleteButton = page.locator('[data-testid="connection-delete-button"]');
    this.deleteConfirmButton = page.locator('[data-testid="connection-delete-confirm"]');
    // ConnectionDetailPage passes saveLabel={t('detail.saveBar.save', 'Save changes')} and
    // resetLabel={t('detail.saveBar.reset', 'Reset')}.
    this.unsavedChangesBar = new UnsavedChangesBar(page, "Save changes", "Reset");
  }

  /**
   * Navigate to the connections list page. Callers follow every goto* here with their own
   * explicit visibility/value assertion, so there's no need to also wait for network idle.
   */
  async goto(): Promise<void> {
    await this.page.goto(`${this.baseUrl}${ConsoleRoutes.connections}`, {
      timeout: Timeouts.PAGE_LOAD,
    });
  }

  /** Click a tab on the edit page by its visible label */
  async clickTab(tabName: string): Promise<void> {
    const tab = this.page.getByRole("tab", { name: new RegExp(tabName, "i") });
    await tab.waitFor({ state: "visible", timeout: Timeouts.ELEMENT_VISIBILITY });
    await tab.click();
  }

  /** Navigate directly to a branded vendor's configure (create) wizard, e.g. type="google" */
  async gotoConfigure(type: string): Promise<void> {
    await this.page.goto(`${this.baseUrl}${ConsoleRoutes.connectionConfigure(type)}`, {
      timeout: Timeouts.PAGE_LOAD,
    });
  }

  /** Navigate directly to a connection's detail page */
  async gotoDetails(type: string, id: string): Promise<void> {
    await this.page.goto(`${this.baseUrl}${ConsoleRoutes.connectionDetails(type, id)}`, {
      timeout: Timeouts.PAGE_LOAD,
    });
  }

  /** Fill the branded-vendor configure form (clientId/clientSecret) */
  async fillOAuthForm(data: BrandedConnectionFormData): Promise<void> {
    await this.clientIdInput.waitFor({ state: "visible", timeout: Timeouts.DEFAULT_ACTION });
    await this.clientIdInput.fill(data.clientId);
    await this.clientSecretInput.fill(data.clientSecret);
  }

  /**
   * Submit the configure/create wizard and wait for navigation to the detail page.
   *
   * A failed create keeps the wizard on its own URL and only renders an inline error, so poll the
   * error and the URL together: waiting on the URL alone reports a bare navigation timeout that
   * says nothing about why the server rejected the request.
   *
   * The last path segment must not be "configure" - that also matches `[^/]+` and is the
   * wizard's own URL, so a bare pattern would resolve immediately without waiting for the
   * post-submit navigation.
   */
  async submitCreate(): Promise<void> {
    await this.wizardCreateButton.click();

    const detailUrl = new RegExp(`${ConsoleRoutes.connections}/[^/]+/(?!configure$)[^/?#]+`);
    await expect(async () => {
      if (await this.wizardCreateError.isVisible()) {
        throw new Error(`Connection create failed: ${(await this.wizardCreateError.innerText()).trim()}`);
      }
      expect(this.page.url()).toMatch(detailUrl);
    }).toPass({ timeout: Timeouts.DEFAULT_ACTION });
  }

  /** Read the connection id from the current detail page URL */
  getConnectionIdFromUrl(): string {
    const match = this.page.url().match(new RegExp(`${ConsoleRoutes.connections}/[^/]+/([^/?#]+)`));
    if (!match) {
      throw new Error(`Could not extract connection id from URL: ${this.page.url()}`);
    }
    return match[1];
  }

  /** Update the scopes field on the detail/edit page, then save */
  async updateScopes(scopes: string): Promise<void> {
    await this.scopesInput.waitFor({ state: "visible", timeout: Timeouts.DEFAULT_ACTION });
    await this.scopesInput.fill(scopes);
    await this.unsavedChangesBar.save();
  }

  /** Delete the connection via the danger-zone button and confirm dialog */
  async delete(): Promise<void> {
    await this.deleteButton.click();
    await this.deleteConfirmButton.waitFor({ state: "visible", timeout: Timeouts.DEFAULT_ACTION });
    await this.deleteConfirmButton.click();
    await expect(this.page).toHaveURL(new RegExp(`${ConsoleRoutes.connections}(?:\\?.*)?$`), {
      timeout: Timeouts.DEFAULT_ACTION,
    });
  }

  /**
   * Locator for a configured connection's card on the list page. The card id rendered by
   * buildConnectionCards.tsx is `${vendorKey}:${instanceId}`, not the bare connection id.
   */
  cardById(type: string, connectionId: string): Locator {
    return this.page.locator(`[data-testid="connection-card-${type}:${connectionId}"]`);
  }
}
