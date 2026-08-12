// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Console Support Fixtures
 *
 * An independent branch off the raw Playwright base, merged in `./index.ts`
 * (same shape as `console-routes.fixture.ts`).
 *
 * - `usersApi` / `applicationsApi`: the shared API helpers bound to the test's request context.
 *   `beforeAll`/`afterAll` cannot take test-scoped fixtures, so they construct `new UsersApi(request)`
 *   (etc.) themselves - same class, so no logic lives in two places.
 * - `isolatedPage`: a page in a throwaway browser context, closed on teardown. For
 *   flows that must not run inside the admin session (e.g. accepting an invite).
 */

import { test as base, type Page } from "@playwright/test";
import { UsersApi } from "../../utils/users-api";
import { ApplicationsApi } from "../../utils/applications-api";

type SupportFixtures = {
  usersApi: UsersApi;
  applicationsApi: ApplicationsApi;
  isolatedPage: Page;
};

export const test = base.extend<SupportFixtures>({
  usersApi: async ({ request }, use) => {
    await use(new UsersApi(request));
  },

  applicationsApi: async ({ request }, use) => {
    await use(new ApplicationsApi(request));
  },

  isolatedPage: async ({ browser }, use) => {
    // browser.newContext() does NOT inherit `use.ignoreHTTPSErrors` from playwright.config,
    // and the gate is served over HTTPS with a self-signed cert locally.
    const context = await browser.newContext({ ignoreHTTPSErrors: true });
    try {
      await use(await context.newPage());
    } finally {
      await context.close();
    }
  },
});
