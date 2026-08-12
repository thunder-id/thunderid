/* eslint-disable playwright/require-top-level-describe */
// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Sample App Login Tests
 *
 * Basic single-factor login/logout tests for the sample app (no MFA).
 *
 * Test Cases:
 * - TC001: Complete login flow with valid username/password
 * - TC002: Logout after a successful login
 *
 * Prerequisites (automatically handled):
 * - Sample app running at SAMPLE_APP_URL
 * - The server running at SERVER_URL
 * - A dedicated test user is created via the API before these tests and removed afterward
 *
 * Required environment variables:
 * - SAMPLE_APP_URL: URL of the sample app (e.g., https://localhost:3000)
 * - SERVER_URL: URL of the server (default: https://localhost:8090)
 * - ADMIN_USERNAME: Admin username (default: "admin")
 * - ADMIN_PASSWORD: Admin password (default: "admin")
 */

import { test, UsersApi } from "../../fixtures/sample-app";
import { TestDataFactory } from "../../utils/test-data";
import { TestTags } from "../../constants/test-tags";

const sampleAppUrl = process.env.SAMPLE_APP_URL;

// Skip tests if SAMPLE_APP_URL is not provided
const describeOrSkip = sampleAppUrl ? test.describe : test.describe.skip;

describeOrSkip("Sample App - Login and Logout", { tag: [TestTags.AUTHENTICATION] }, () => {
  const testUser = TestDataFactory.createUser();
  let userId: string | null = null;

  test.beforeAll(async ({ request }) => {
    console.log("\n=== Login Test Suite Setup ===");
    // beforeAll/afterAll cannot take custom test-scoped fixtures, so construct the shared
    // helper directly here - same class the fixture uses inside test bodies elsewhere.
    const user = await new UsersApi(request).createUser(testUser);
    userId = user.id;
    console.log(`✓ Test user created: ${userId}`);
    console.log("=========================\n");
  });

  test.afterAll(async ({ request }) => {
    if (!userId) return;

    console.log("\n=== Login Test Suite Teardown ===");
    const deleted = await new UsersApi(request).deleteById(userId);
    if (deleted) {
      console.log(`✓ Test user deleted: ${userId}`);
    } else {
      console.log(`⚠️  Failed to delete test user ${userId}`);
    }
    console.log("===============================\n");
  });

  test("TC001: Complete login flow with valid username/password", async ({ sampleAppLoginPage }) => {
    console.log("\n--- TC001: Basic Login ---");

    console.log("Step 1: Navigating to sample app...");
    await sampleAppLoginPage.goto(sampleAppUrl!);
    await sampleAppLoginPage.verifyHomePageLoaded();
    console.log("✓ Sample app home page loaded");

    console.log("Step 2: Clicking Sign In button...");
    await sampleAppLoginPage.clickSignInButton();
    await sampleAppLoginPage.verifyLoginPageLoaded();
    console.log("✓ Login page displayed");

    console.log("Step 3: Entering credentials...");
    await sampleAppLoginPage.fillLoginForm(testUser.username, testUser.password);
    console.log(`  Username: ${testUser.username}`);
    console.log("  Password: ********");

    console.log("Step 4: Submitting login form...");
    await sampleAppLoginPage.clickLogin();
    console.log("✓ Login form submitted");

    console.log("Step 5: Verifying successful login...");
    await sampleAppLoginPage.verifyLoggedIn();
    console.log("✓ Login successful");

    console.log("\n--- TC001 Completed Successfully ---\n");
  });

  test("TC002: Logout after a successful login", async ({ sampleAppLoginPage }) => {
    console.log("\n--- TC002: Logout ---");

    console.log("Step 1: Navigating to sample app...");
    await sampleAppLoginPage.goto(sampleAppUrl!);
    await sampleAppLoginPage.verifyHomePageLoaded();
    console.log("✓ Sample app home page loaded");

    console.log("Step 2: Logging in...");
    await sampleAppLoginPage.clickSignInButton();
    await sampleAppLoginPage.verifyLoginPageLoaded();
    await sampleAppLoginPage.fillLoginForm(testUser.username, testUser.password);
    await sampleAppLoginPage.clickLogin();
    await sampleAppLoginPage.verifyLoggedIn();
    console.log("✓ Logged in");

    console.log("Step 3: Logging out...");
    await sampleAppLoginPage.logout();
    console.log("✓ Logout submitted");

    console.log("Step 4: Verifying logout...");
    await sampleAppLoginPage.verifyLoggedOut();
    console.log("✓ Logged out - login page displayed again");

    console.log("\n--- TC002 Completed Successfully ---\n");
  });
});
