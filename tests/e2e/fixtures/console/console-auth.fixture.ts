// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Console Authentication Fixture
 *
 * Provides an `authenticatedPage` fixture that automatically ensures
 * the test page is authenticated before the test runs.
 *
 * This replaces the manual `setupAuthentication` call in beforeEach.
 *
 * @example
 * import { test } from '../fixtures';
 *
 * test('authenticated test', async ({ authenticatedPage }) => {
 *   // authenticatedPage is already logged in
 *   await authenticatedPage.goto(routes.users);
 * });
 */

import { test as base, Page } from "@playwright/test";
import {
  invalidateAuthState,
  resyncStorageBeforeNavigation,
  saveAuthState,
  setupAuthentication,
} from "../../utils/authentication";

const baseUrl = process.env.BASE_URL as string;

type AuthFixtures = {
  authenticatedPage: Page;
};

export const test = base.extend<AuthFixtures>({
  authenticatedPage: async ({ page }, use) => {
    // Setup authentication before test usage
    const debugAuth = process.env.DEBUG_AUTH === "true";
    const authPath = await setupAuthentication(page, baseUrl, { debug: debugAuth });

    // Every subsequent goto/reload on this page re-runs the SAME init script setupAuthentication
    // just registered, unchanged - so if the SDK silently rotates the token mid-test, a later
    // navigation would otherwise revert to the stale pre-test snapshot (see
    // resyncStorageBeforeNavigation's doc comment). Wrapping here covers every page object built
    // on top of this fixture (UsersPage, ApplicationsPage, ...) without touching test call sites.
    // The returned handle is threaded through so each resync disposes the previous one instead of
    // piling up an extra registered script per navigation.
    let initScriptHandle: Awaited<ReturnType<typeof resyncStorageBeforeNavigation>>;

    const originalGoto = page.goto.bind(page);
    page.goto = (async (...args: Parameters<Page["goto"]>) => {
      initScriptHandle = await resyncStorageBeforeNavigation(page, baseUrl, initScriptHandle);
      return originalGoto(...args);
    }) as Page["goto"];

    const originalReload = page.reload.bind(page);
    page.reload = (async (...args: Parameters<Page["reload"]>) => {
      initScriptHandle = await resyncStorageBeforeNavigation(page, baseUrl, initScriptHandle);
      return originalReload(...args);
    }) as Page["reload"];

    // Provide the authenticated page to the test
    await use(page);

    // Re-save whatever session the page ends up holding: the SDK may have silently rotated the
    // access/refresh token pair mid-test (see saveAuthState's doc comment), and the next test in
    // this worker must not replay the stale pair this fixture injected at the top of this test.
    await saveAuthState(page, baseUrl, authPath, debugAuth).catch(error => {
      // The file on disk is now whatever the last successful save left there, which may itself be
      // an already-used refresh token if this test rotated one - invalidate rather than risk the
      // next test in this worker replaying it (see invalidateAuthState's doc comment).
      console.error("⚠️ Failed to re-save auth state after test, invalidating it instead:", error);
      invalidateAuthState(authPath);
    });
  },
});
