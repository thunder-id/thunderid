// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Bookmarked Sign-in Page Tests
 *
 * A sign-in page URL that is bookmarked or shared outlives the flow it was opened for. Opening it later,
 * with no browser session, must lead back to a working sign-in rather than a dead end.
 *
 * Test Cases:
 * - TC-BOOKMARK-001: After the first step loads, the sign-in page URL keeps only the ref parameter
 * - TC-BOOKMARK-002: The bookmarked URL, opened in a new browser, goes back to the application, which
 *   starts a new sign-in
 * - TC-BOOKMARK-003: A sign-in page URL with no parameters asks the user to return to the application
 */

import { test, expect, ConsoleSigninPage } from "../../fixtures/console";
import { ConsoleRoutes } from "../../configs/routes/console-routes";
import { TestTags } from "../../constants/test-tags";
import { Timeouts } from "../../constants/timeouts";
import { UIMessages } from "../../constants/ui-messages";

test.describe("Bookmarked sign-in page", { tag: [TestTags.AUTHENTICATION] }, () => {
  test("TC-BOOKMARK-001: the sign-in page URL keeps only ref", async ({ signinPage, page }) => {
    await signinPage.gotoHome();
    await signinPage.waitForLoginForm();

    await expect
      .poll(() => [...new URL(page.url()).searchParams.keys()], { timeout: Timeouts.ELEMENT_VISIBILITY })
      .toEqual(["ref"]);
  });

  test("TC-BOOKMARK-002: a bookmarked URL goes back to the application", async ({ signinPage, page, browser }) => {
    await signinPage.gotoHome();
    await signinPage.waitForLoginForm();
    await expect.poll(() => new URL(page.url()).searchParams.get("executionId")).toBeNull();
    const bookmark = page.url();
    const ref = new URL(bookmark).searchParams.get("ref");
    expect(ref).toBeTruthy();

    // A new context has no cookies or session storage, as when a bookmark or a shared link is opened later.
    const context = await browser.newContext({ ignoreHTTPSErrors: true });
    const bookmarkPage = await context.newPage();
    try {
      const bookmarkSignin = new ConsoleSigninPage(bookmarkPage, new URL(bookmark).origin);
      // The way back comes from looking up this ref, not from a server default.
      const refLookup = bookmarkPage.waitForResponse(
        response => {
          const url = new URL(response.url());
          return (
            url.pathname.endsWith("/flow/meta") &&
            url.searchParams.get("type") === "REF" &&
            url.searchParams.get("id") === ref &&
            response.ok()
          );
        },
        { timeout: Timeouts.REDIRECT }
      );
      const consoleUrl = new URL(`${signinPage.baseUrl}${ConsoleRoutes.home}`);
      const visitedApplication = bookmarkPage.waitForRequest(
        request => {
          const url = new URL(request.url());
          return (
            request.isNavigationRequest() &&
            url.origin === consoleUrl.origin &&
            url.pathname.startsWith(consoleUrl.pathname)
          );
        },
        { timeout: Timeouts.REDIRECT }
      );
      await Promise.all([refLookup, visitedApplication, bookmarkSignin.gotoUrl(bookmark)]);

      await bookmarkSignin.waitForLoginForm();
      await expect(bookmarkPage.getByText(UIMessages.signIn.returnToApplicationUnavailable)).toBeHidden();
    } finally {
      await context.close();
    }
  });

  test("TC-BOOKMARK-003: a URL with no parameters asks the user to return", async ({ signinPage, page, request }) => {
    // With a server default return URL, the page goes there instead of showing the message.
    const meta = await request.get("/flow/meta");
    expect(meta.ok()).toBeTruthy();
    test.skip(Boolean((await meta.json()).defaultReturnUrl), "The server has a default return URL");

    await signinPage.goto();

    await expect(page.getByText(UIMessages.signIn.returnToApplicationUnavailable)).toBeVisible({
      timeout: Timeouts.ELEMENT_VISIBILITY,
    });
  });
});
