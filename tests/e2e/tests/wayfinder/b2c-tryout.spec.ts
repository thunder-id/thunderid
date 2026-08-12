/* eslint-disable playwright/require-top-level-describe */
// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Wayfinder B2C Tryout E2E Tests
 *
 * Exercises the standalone Wayfinder sample app itself (not the console): sign-in and sign-out
 * with the seed user shipped in the Wayfinder config bundle.
 *
 * This spec must run after wayfinder-sample-setup.spec.ts has imported that bundle (the seed
 * user and the "Wayfinder" OAuth client only exist afterward) - enforced structurally by the
 * wayfinder-setup -> wayfinder-tryout project dependency in playwright.config.ts, not by any
 * import step in this file.
 *
 * Skipped entirely if WAYFINDER_APP_URL is not provided (matches sample-app-login.spec.ts's
 * SAMPLE_APP_URL pattern).
 *
 * Required environment variables:
 * - WAYFINDER_APP_URL: URL of the running Wayfinder sample app (e.g. http://localhost:5173)
 */

import { test } from "@playwright/test";
import { WayfinderAppPage, MockEmailAppPage } from "../../pages/wayfinder-sample";
import { TestTags } from "../../constants/test-tags";

const wayfinderUrl = process.env.WAYFINDER_APP_URL;

// Seed user shipped with the Wayfinder bundle - see
// samples/apps/wayfinder-sample/thunderid-config/redirect/thunderid.env
const SEED_USERNAME = "john.doe";
const SEED_PASSWORD = "john.doe";

// Skip tests if WAYFINDER_APP_URL is not provided
const describeOrSkip = wayfinderUrl ? test.describe : test.describe.skip;

describeOrSkip("Wayfinder B2C Tryout", { tag: [TestTags.WAYFINDER] }, () => {
  test("TC001: Complete sign-in flow with the seed user", async ({ page }) => {
    const wayfinderPage = new WayfinderAppPage(page);

    await wayfinderPage.goto(wayfinderUrl!);
    await wayfinderPage.verifyUnAuthenticatedHomePageLoaded();

    await wayfinderPage.clickSignInButton();
    await wayfinderPage.verifyLoginPageLoaded();
    await wayfinderPage.login(SEED_USERNAME, SEED_PASSWORD);
    await wayfinderPage.verifyLoggedIn();
  });

  test("TC002: Sign out after a successful sign-in", async ({ page }) => {
    const wayfinderPage = new WayfinderAppPage(page);

    await wayfinderPage.goto(wayfinderUrl!);
    await wayfinderPage.verifyUnAuthenticatedHomePageLoaded();
    await wayfinderPage.clickSignInButton();
    await wayfinderPage.verifyLoginPageLoaded();
    await wayfinderPage.login(SEED_USERNAME, SEED_PASSWORD);
    await wayfinderPage.verifyLoggedIn();

    await wayfinderPage.logout();
    await wayfinderPage.verifyLoggedOut();
  });

  test.fixme("TC003: Attempt self-sign with new user with default signup flow", async ({ page }) => {
    const wayfinderPage = new WayfinderAppPage(page);

    await wayfinderPage.goto(wayfinderUrl!);
    await wayfinderPage.verifyUnAuthenticatedHomePageLoaded();
    await wayfinderPage.clickSignInButton();
    await wayfinderPage.verifyLoginPageLoaded();
    await wayfinderPage.clickSignupLink();
    await wayfinderPage.verifyFirstSignupPageLoaded();
    await wayfinderPage.fillSignupForm("emily.wilson", "emily.wilson");
    await wayfinderPage.clickContinueButton();
    await wayfinderPage.verifySecondSignupPageLoaded();
    await wayfinderPage.fillSignupForm("emilywilson@example.com");
    await wayfinderPage.clickContinueButton();
    // Self-registration only creates the account; it does not sign the new user in. The flow
    // ends on a confirmation screen whose link returns to sign-in, not on an authenticated state.
    await wayfinderPage.verifyRegistrationCompleteScreenLoaded();
    await wayfinderPage.clickRegistrationCompleteLink();
    await wayfinderPage.verifyLoginPageLoaded();
  });

  test("TC004: Attempt self-sign with existing user with default signup flow", async ({ page }) => {
    const wayfinderPage = new WayfinderAppPage(page);

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

    test("TC005: Attempt password reset with fake user", async ({ page }) => {
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

  test.fixme("TC006: Attempt password reset with existing user", async ({ page, context }) => {
    const wayfinderPage = new WayfinderAppPage(page);
    
    await wayfinderPage.goto(wayfinderUrl!);
    await wayfinderPage.verifyUnAuthenticatedHomePageLoaded();
    await wayfinderPage.clickSignInButton();
    await wayfinderPage.verifyLoginPageLoaded();
    await wayfinderPage.clickResetPasswordLink();
    await wayfinderPage.verifyResetPasswordPageLoaded();
    await wayfinderPage.fillResetPasswordForm(SEED_USERNAME);
    await wayfinderPage.clickSendRecoveryLinkButton();
    await wayfinderPage.verifyResetPasswordConfirmationScreenLoaded();

    // Get the reset link from the email sent to the user using the mock email service.
    const mailPage = await context.newPage();
    const mockEmailPage = new MockEmailAppPage(mailPage);
    await mockEmailPage.goto("http://localhost:8788/"); // Navigate to the mock email service
    
    // Wait until load, click the email, and click the reset link in the email body
    await mockEmailPage.openEmailBySubject(/Reset your password/i);
    const resetPage = await mockEmailPage.clickLinkInEmail(/reset password/i);
    
    // Now on the reset password form page in the new tab
    const wayfinderResetPage = new WayfinderAppPage(resetPage);
    await wayfinderResetPage.verifyNewPasswordPageLoaded();
    await wayfinderResetPage.fillNewPasswordForm(SEED_PASSWORD);
    await wayfinderResetPage.clickResetPasswordSubmitButton();
    await wayfinderResetPage.verifyPasswordResetSuccessful();

    await resetPage.close();
    await mailPage.close();
  });
});

