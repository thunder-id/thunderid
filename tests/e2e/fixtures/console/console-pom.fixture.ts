/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

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
