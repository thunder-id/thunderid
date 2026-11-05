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
import { AgentOnboardingPage, AgentDetailsPage } from "../../pages/agents";

const baseUrl = process.env.BASE_URL || "";

type POMFixtures = {
  signinPage: ConsoleSigninPage;
  usersPage: UsersPage;
  agentOnboardingPage: AgentOnboardingPage;
  agentDetailsPage: AgentDetailsPage;
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

  // Agent onboarding page requires auth, uses authenticatedPage fixture
  agentOnboardingPage: async ({ authenticatedPage }, use) => {
    await use(new AgentOnboardingPage(authenticatedPage, baseUrl));
  },

  // Agent details page requires auth, uses authenticatedPage fixture
  agentDetailsPage: async ({ authenticatedPage }, use) => {
    await use(new AgentDetailsPage(authenticatedPage, baseUrl));
  },
});

export { expect } from "@playwright/test";
export { ConsoleSigninPage } from "../../pages/authentication";
export { UsersPage, type UserFormData } from "../../pages/user-management";
export { AgentOnboardingPage, AgentDetailsPage } from "../../pages/agents";
