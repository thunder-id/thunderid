// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Console Page Object Model Fixture
 *
 * Provides page object models as Playwright fixtures.
 *
 * - `signinPage`: Uses standard `page` (no auth required)
 * - `usersPage`: Uses `authenticatedPage` (enforces auth)
 */

import { test as base } from "./console-auth.fixture";
import { ConsoleSigninPage } from "../../pages/authentication";
import { UsersPage } from "../../pages/user-management";
import { ApplicationsPage } from "../../pages/applications";
import { SettingsPage } from "../../pages/settings";
import { WelcomePage } from "../../pages/welcome";

const baseUrl = process.env.BASE_URL || "";

type POMFixtures = {
  signinPage: ConsoleSigninPage;
  usersPage: UsersPage;
  applicationsPage: ApplicationsPage;
  settingsPage: SettingsPage;
  welcomePage: WelcomePage;
};

export const test = base.extend<POMFixtures>({
  // Signin page does NOT need auth, uses raw page
  signinPage: async ({ page }, use) => {
    await use(new ConsoleSigninPage(page, baseUrl));
  },

  // Users page requires auth, uses authenticatedPage fixture
  usersPage: async ({ authenticatedPage }, use) => {
    await use(new UsersPage(authenticatedPage, baseUrl));
  },

  // Applications page requires auth, uses authenticatedPage fixture
  applicationsPage: async ({ authenticatedPage }, use) => {
    await use(new ApplicationsPage(authenticatedPage, baseUrl));
  },

  // Settings page requires auth, uses authenticatedPage fixture
  settingsPage: async ({ authenticatedPage }, use) => {
    await use(new SettingsPage(authenticatedPage, baseUrl));
  },
  // Welcome page requires auth, uses authenticatedPage fixture
  welcomePage: async ({ authenticatedPage }, use) => {
    await use(new WelcomePage(authenticatedPage, baseUrl));
  },
});

export { expect } from "@playwright/test";
export { ConsoleSigninPage } from "../../pages/authentication";
export { UsersPage, type UserFormData } from "../../pages/user-management";
export { ApplicationsPage, type ApplicationFormData } from "../../pages/applications";
export { SettingsPage } from "../../pages/settings";
export { WelcomePage } from "../../pages/welcome";
