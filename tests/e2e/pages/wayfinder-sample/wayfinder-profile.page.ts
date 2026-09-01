// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Wayfinder Profile Page Object
 *
 * Page Object Model for the Wayfinder sample app's self-service Profile page (/profile): the
 * "Profile attributes" form (backed by GET/PUT /users/me) and the "Change password" form (backed
 * by POST /users/me/update-credentials). Each attribute row is rendered as a <label> wrapping its
 * <input>, so attributeInput() locates fields by their accessible (implicit-label) name rather
 * than a hardcoded selector per attribute.
 */

import { Page, Locator, expect } from "@playwright/test";
import { BasePage } from "../base.page";
import { Timeouts } from "../../constants/timeouts";

export class WayfinderProfilePage extends BasePage {
  readonly heading: Locator;
  readonly saveChangesButton: Locator;
  readonly currentPasswordInput: Locator;
  readonly newPasswordInput: Locator;
  readonly confirmPasswordInput: Locator;
  readonly updatePasswordButton: Locator;

  constructor(page: Page) {
    super(page);
    this.heading = page.getByRole("heading", { name: /^profile$/i });
    this.saveChangesButton = page.getByRole("button", { name: /save changes/i });
    this.currentPasswordInput = page.getByLabel("Current password", { exact: true });
    this.newPasswordInput = page.getByLabel("New password", { exact: true });
    this.confirmPasswordInput = page.getByLabel("Confirm new password");
    this.updatePasswordButton = page.getByRole("button", { name: /update password/i });
  }

  /** Locator for a profile attribute's input, keyed by its attribute name (e.g. "family_name"). */
  attributeInput(attributeName: string): Locator {
    return this.page.getByLabel(attributeName, { exact: true });
  }

  async verifyProfileLoaded() {
    await expect(this.heading).toBeVisible({ timeout: Timeouts.ELEMENT_VISIBILITY });
    await this.saveChangesButton.waitFor({ state: "visible", timeout: Timeouts.ELEMENT_VISIBILITY });
  }

  async verifyAttributeValue(attributeName: string, expectedValue: string) {
    await expect(this.attributeInput(attributeName)).toHaveValue(expectedValue, {
      timeout: Timeouts.ELEMENT_VISIBILITY,
    });
  }

  /** Edit one attribute field and save. Verifies the "Profile updated." confirmation. */
  async updateAttribute(attributeName: string, newValue: string) {
    await this.attributeInput(attributeName).fill(newValue);
    await this.saveChangesButton.click();
    await expect(this.page.getByText("Profile updated.")).toBeVisible({ timeout: Timeouts.DEFAULT_ACTION });
  }

  /**
   * Set a new password and save. Verifies the "Password updated." confirmation.
   *
   * The current password is required: POST /users/me/update-credentials verifies it before the
   * write, so a stolen access token alone cannot change the credential.
   */
  async changePassword(currentPassword: string, newPassword: string) {
    await this.currentPasswordInput.fill(currentPassword);
    await this.newPasswordInput.fill(newPassword);
    await this.confirmPasswordInput.fill(newPassword);
    await this.updatePasswordButton.click();
    await expect(this.page.getByText("Password updated.")).toBeVisible({ timeout: Timeouts.DEFAULT_ACTION });
  }
}
