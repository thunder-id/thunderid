/* eslint-disable playwright/require-top-level-describe */
// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Wayfinder B2C Tryout - Sample App Authentication E2E Tests
 *
 * Exercises the standalone Wayfinder sample app itself (not the console): sign-in and sign-out
 * with the seed user shipped in the Wayfinder config bundle, self-registration, and the
 * password-reset error path for an unknown user.
 *
 * This spec must run after wayfinder-sample-setup.spec.ts has imported that bundle (the seed
 * user and the "Wayfinder" OAuth client only exist afterward) - enforced structurally by the
 * wayfinder-setup -> wayfinder-tryout project dependency in playwright.config.ts, not by any
 * import step in this file.
 *
 * Skipped entirely if WAYFINDER_APP_URL is not provided (matches sample-app-login.spec.ts's
 * SAMPLE_APP_URL pattern).
 *
 * The password-reset-with-existing-user case (and any other test that reads from the shared mock
 * SMTP inbox) lives in ../mock-email-flows.spec.ts instead: that inbox has no per-recipient
 * filtering, so tests reading from it must run one at a time in a single, chromium-only project -
 * this file's tests don't touch it and stay free to run fully parallel across all three browsers.
 *
 * Required environment variables:
 * - WAYFINDER_APP_URL: URL of the running Wayfinder sample app (e.g. http://localhost:5173)
 */

import { test } from "@playwright/test";
import { WayfinderAppPage } from "../../../pages/wayfinder-sample";
import { TestTags } from "../../../constants/test-tags";
import { UsersApi } from "../../../utils/users-api";

const wayfinderUrl = process.env.WAYFINDER_APP_URL;

// Seed user shipped with the Wayfinder bundle - see
// samples/apps/wayfinder-sample/thunderid-config/redirect/thunderid.env
const SEED_USERNAME = "john.doe";
const SEED_PASSWORD = "john.doe";

// Skip tests if WAYFINDER_APP_URL is not provided
const describeOrSkip = wayfinderUrl ? test.describe : test.describe.skip;

describeOrSkip("Wayfinder B2C Tryout", { tag: [TestTags.WAYFINDER] }, () => {
  // TC002 self-registers a new user through the sample app's signup flow; track its username
  // here so afterAll can remove it via the admin API.
  const selfSignedUpUsernames: string[] = [];

  test.afterAll(async ({ request }) => {
    const usersApi = new UsersApi(request);
    for (const username of selfSignedUpUsernames) {
      const deleted = await usersApi.deleteByUsername(username);
      console.log(`Teardown: removed ${deleted} user(s) matching ${username}`);
    }
  });

  test("TC001: Complete sign-in flow with the seed user, then sign out", async ({ page }) => {
    const wayfinderPage = new WayfinderAppPage(page);

    await test.step("Sign in with the seed user", async () => {
      await wayfinderPage.goto(wayfinderUrl!);
      await wayfinderPage.verifyUnAuthenticatedHomePageLoaded();

      await wayfinderPage.clickSignInButton();
      await wayfinderPage.verifyLoginPageLoaded();
      await wayfinderPage.login(SEED_USERNAME, SEED_PASSWORD);
      await wayfinderPage.verifyLoggedIn();
    });

    await test.step("Sign out", async () => {
      await wayfinderPage.logout();
      await wayfinderPage.verifyLoggedOut();
    });
  });

  test("TC002: Self-signup with a new user succeeds, with an existing username rejected", async ({ page }) => {
    const wayfinderPage = new WayfinderAppPage(page);

    await test.step("Attempt signup with an existing username", async () => {
      await wayfinderPage.goto(wayfinderUrl!);
      await wayfinderPage.verifyUnAuthenticatedHomePageLoaded();
      await wayfinderPage.clickSignInButton();
      await wayfinderPage.verifyLoginPageLoaded();
      await wayfinderPage.clickSignupLink();
      await wayfinderPage.verifyFirstSignupPageLoaded();
      await wayfinderPage.fillSignupForm(SEED_USERNAME, SEED_PASSWORD);
      await wayfinderPage.clickContinueButton();
      await wayfinderPage.verifyUserAlreadyExistsError();
    });

    await test.step("Complete self-signup with a new user", async () => {
      // Generate random credentials
      const { username, password, email } = wayfinderPage.generateRandomCredentials();
      selfSignedUpUsernames.push(username);

      await wayfinderPage.goto(wayfinderUrl!);
      await wayfinderPage.verifyUnAuthenticatedHomePageLoaded();
      await wayfinderPage.clickSignInButton();
      await wayfinderPage.verifyLoginPageLoaded();
      await wayfinderPage.clickSignupLink();
      await wayfinderPage.verifyFirstSignupPageLoaded();
      await wayfinderPage.fillSignupForm(username, password);
      await wayfinderPage.clickContinueButton();
      await wayfinderPage.verifySecondSignupPageLoaded();
      await wayfinderPage.fillSignupForm(email);
      await wayfinderPage.clickContinueButton();
      await wayfinderPage.verifyLoggedIn();
    });
  });

  test("TC003: Attempt password reset with fake user", async ({ page }) => {
    const wayfinderPage = new WayfinderAppPage(page);

    await wayfinderPage.goto(wayfinderUrl!);
    await wayfinderPage.verifyUnAuthenticatedHomePageLoaded();
    await wayfinderPage.clickSignInButton();
    await wayfinderPage.verifyLoginPageLoaded();
    await wayfinderPage.clickResetPasswordLink();
    await wayfinderPage.verifyResetPasswordPageLoaded();
    await wayfinderPage.fillResetPasswordForm("fake.user");
    await wayfinderPage.clickSendRecoveryLinkButton();
    await wayfinderPage.verifyRecoverUserNotFoundError();
  });
});
