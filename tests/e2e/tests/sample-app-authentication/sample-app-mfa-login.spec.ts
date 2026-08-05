/* eslint-disable playwright/require-top-level-describe */
// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Sample App MFA Login Tests
 *
 * Tests for Multi-Factor Authentication (MFA) login flow with SMS OTP.
 * These tests verify the complete MFA authentication process:
 * 1. Username/Password authentication (first factor)
 * 2. SMS OTP verification (second factor)
 * 3. User registration with mobile number
 *
 * Test Cases:
 * - TC001: Complete MFA login flow with username/password + SMS OTP
 *   Verifies successful two-factor authentication for existing users
 * - TC002: Verify incorrect OTP shows error
 *   Validates OTP verification error handling and rejection of invalid codes
 * - TC003: Complete MFA registration flow with mobile number and subsequent login
 *   Tests end-to-end user registration including mobile number capture and MFA login
 *
 * Prerequisites (automatically handled):
 * - Sample app running at SAMPLE_APP_URL
 * - The server running at SERVER_URL
 * - Mock SMS server (automatically started)
 * - MFA authentication flow (automatically created)
 * - MFA registration flow
 * - Test user with mobile number (automatically created)
 * - Notification sender (automatically configured)
 *
 * Required environment variables:
 * - SAMPLE_APP_URL: URL of the sample app (e.g., https://localhost:3000)
 * - SERVER_URL: URL of the server (default: https://localhost:8090)
 * - ADMIN_USERNAME: Admin username (default: "admin")
 * - ADMIN_PASSWORD: Admin password (default: "admin")
 * - SAMPLE_APP_USERNAME: Test user username (default: "e2e-test-user")
 * - SAMPLE_APP_PASSWORD: Test user password (default: "e2e-test-password")
 * - MOCK_SMS_SERVER_PORT: Port for mock SMS server (default: 8098)
 * - AUTO_SETUP_MFA: Enable automatic setup (default: "true")
 */

import { test, expect, UsersApi } from "../../fixtures/sample-app";
import { MockSMSServer } from "../../utils/mock-sms-server";
import { MFASetup, SetupResult } from "../../utils/server-setup";
import { Timeouts } from "../../constants/timeouts";
import { TestDataFactory } from "../../utils/test-data";
import { SampleAppClientIds } from "../../constants/sample-apps";

const sampleAppUrl = process.env.SAMPLE_APP_URL;
const username = process.env.SAMPLE_APP_USERNAME || "e2e-test-user";
const password = process.env.SAMPLE_APP_PASSWORD || "e2e-test-password";
const mockSMSPort = process.env.MOCK_SMS_SERVER_PORT ? parseInt(process.env.MOCK_SMS_SERVER_PORT, 10) : 8098;
const autoSetup = process.env.AUTO_SETUP_MFA !== "false"; // Default to true

async function waitForSMS(
  server: MockSMSServer,
  timeoutMs = 10000
): Promise<ReturnType<MockSMSServer["getLastMessage"]>> {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    const msg = server.getLastMessage();
    if (msg) return msg;
    await new Promise(r => setTimeout(r, 300));
  }
  return null;
}

// Skip tests if SAMPLE_APP_URL is not provided
const describeOrSkip = sampleAppUrl ? test.describe : test.describe.skip;

describeOrSkip("Sample App - MFA Authentication with SMS OTP", () => {
  // Every test in this suite drives the sample app as its own dedicated application
  // (constants/sample-apps.ts), rather than the shared REACT_SDK_SAMPLE default-login uses.
  test.use({ sampleAppClientId: SampleAppClientIds.MFA });

  // Mock SMS server instance - shared across tests in this suite
  let mockSMSServer: MockSMSServer;
  // MFA setup result - contains IDs and cleanup functions
  let setupResult: SetupResult | null = null;
  // Generated at describe scope, same as user-creation.spec.ts, so afterAll can see the
  // username TC003 registers through the sample app's Sign Up flow.
  const registeredUser = TestDataFactory.createUser();

  // Setup: Start mock SMS server and configure MFA before all tests
  test.beforeAll(async ({ request }) => {
    test.setTimeout(Timeouts.SUITE_SETUP);

    console.log("\n=== MFA Test Suite Setup ===");

    // Step 1: Start Mock SMS Server
    console.log(`Starting Mock SMS Server on port ${mockSMSPort}...`);
    mockSMSServer = new MockSMSServer(mockSMSPort);

    try {
      await mockSMSServer.start();
      console.log(`✓ Mock SMS Server started successfully at ${mockSMSServer.getURL()}`);
      console.log(`  SMS Endpoint: ${mockSMSServer.getSendSMSURL()}`);
    } catch (error) {
      console.error("✗ Failed to start Mock SMS Server:", error);
      throw error;
    }

    // Step 2: Automated MFA Setup (if enabled)
    if (autoSetup) {
      console.log("\nPerforming automated server MFA setup...");
      const setup = new MFASetup(request, {
        clientId: SampleAppClientIds.MFA,
        mockSmsUrl: mockSMSServer.getSendSMSURL(),
        testUser: {
          username,
          password,
          email: "e2e@example.com",
          mobile_number: "+12345678920",
          given_name: "E2E Test User",
        },
      });

      // A failure here means the server is not wired for MFA, so every test below would either
      // skip or fail far from the real cause. Fail the suite immediately instead of swallowing it.
      setupResult = await setup.setup();
      console.log("✓ Automated setup completed successfully");
    } else {
      console.log("⚠️  Automated setup disabled (AUTO_SETUP_MFA=false)");
      console.log("⚠️  Ensure the server is configured manually as per README-MFA.md");
    }

    console.log("=========================\n");
  });

  // Teardown: Stop mock SMS server and cleanup server resources after all tests
  test.afterAll(async ({ request }) => {
    test.setTimeout(Timeouts.SUITE_SETUP);
    console.log("\n=== MFA Test Suite Teardown ===");

    // Cleanup the user TC003 registered, if it ran
    const deleted = await new UsersApi(request).deleteByUsername(registeredUser.username);
    console.log(`Teardown: removed ${deleted} user(s) matching ${registeredUser.username}`);

    // Cleanup server resources. `request` here is this hook's own fixture instance, still live -
    // unlike the beforeAll-scoped one the original `setup` was constructed with.
    if (setupResult && autoSetup) {
      await MFASetup.cleanup(request, setupResult.cleanupFunctions);
    }

    // Stop mock SMS server
    if (mockSMSServer) {
      try {
        await mockSMSServer.stop();
        console.log("✓ Mock SMS Server stopped successfully");
      } catch (error) {
        console.error("✗ Failed to stop Mock SMS Server:", error);
      }
    }

    console.log("===============================\n");
  });

  // Clear messages before each test
  test.beforeEach(async () => {
    if (mockSMSServer) {
      mockSMSServer.clearMessages();
      console.log("Cleared SMS message history");
    }
  });

  test("TC001: Complete MFA login flow with username/password + SMS OTP", async ({ sampleAppLoginPage }) => {
    console.log("\n--- TC001: MFA Login with SMS OTP ---");

    // Step 1: Navigate to sample app
    console.log("Step 1: Navigating to sample app...");
    await sampleAppLoginPage.goto(sampleAppUrl!);
    await sampleAppLoginPage.verifyHomePageLoaded();
    console.log("✓ Sample app home page loaded");

    // Step 2: Click Sign In button
    console.log("\nStep 2: Clicking Sign In button...");
    await sampleAppLoginPage.clickSignInButton();
    await sampleAppLoginPage.verifyLoginPageLoaded();
    console.log("✓ Login page displayed");

    // Step 3: Enter username and password (first factor)
    console.log("\nStep 3: Entering credentials (first factor)...");
    await sampleAppLoginPage.fillLoginForm(username, password);
    console.log(`  Username: ${username}`);
    console.log("  Password: ********");

    // Step 4: Submit login form
    console.log("\nStep 4: Submitting login form...");
    await sampleAppLoginPage.clickLogin();
    console.log("✓ Login form submitted");

    // Step 5: Wait for OTP page to load
    console.log("\nStep 5: Waiting for OTP verification page...");

    // beforeAll fails the suite if MFA could not be configured, so by this point the OTP prompt is
    // required: its absence is a real regression in the MFA flow, not an unconfigured server.
    await sampleAppLoginPage.verifyOTPPageLoaded();
    console.log("✓ OTP verification page displayed");

    // Step 6: Wait for SMS to be sent and retrieve OTP from mock server
    console.log("\nStep 6: Retrieving OTP from mock SMS server...");

    const lastMessage = await waitForSMS(mockSMSServer);

    // Validate that SMS was received
    expect(lastMessage).not.toBeNull();
    expect(lastMessage!.otp).toBeTruthy();
    expect(lastMessage!.otp).toMatch(/^\d{4,8}$/); // OTP should be 4-8 digits

    console.log(
      `✓ SMS received: "${lastMessage!.message.substring(0, 60)}${lastMessage!.message.length > 60 ? "..." : ""}"`
    );
    console.log(`✓ OTP extracted: ${lastMessage!.otp}`);

    // Step 7: Enter OTP (second factor)
    console.log("\nStep 7: Entering OTP (second factor)...");
    await sampleAppLoginPage.fillOTP(lastMessage!.otp);
    console.log(`  OTP: ${lastMessage!.otp}`);

    // Step 8: Submit OTP verification
    console.log("\nStep 8: Submitting OTP verification...");
    await sampleAppLoginPage.clickVerifyOTP();
    console.log("✓ OTP verification submitted");

    // Step 9: Verify successful MFA authentication
    console.log("\nStep 9: Verifying successful MFA authentication...");
    await sampleAppLoginPage.verifyLoggedIn();
    console.log("✓ MFA authentication successful - User logged in");

    console.log("\n--- TC001 Completed Successfully ---\n");
  });

  test("TC002: Verify incorrect OTP shows error", async ({ sampleAppLoginPage, page }) => {
    console.log("\n--- TC002: Incorrect OTP Validation ---");

    // Step 1: Navigate and complete password auth
    console.log("Step 1: Completing password authentication...");
    await sampleAppLoginPage.goto(sampleAppUrl!);
    await sampleAppLoginPage.verifyHomePageLoaded();
    await sampleAppLoginPage.clickSignInButton();
    await sampleAppLoginPage.verifyLoginPageLoaded();
    await sampleAppLoginPage.fillLoginForm(username, password);
    await sampleAppLoginPage.clickLogin();

    // Step 2: Wait for OTP page
    console.log("\nStep 2: Waiting for OTP verification page...");
    await sampleAppLoginPage.verifyOTPPageLoaded();
    console.log("✓ OTP verification page displayed");

    // Step 3: Wait for correct OTP to be sent (but don't use it)
    console.log("\nStep 3: Waiting for SMS (will use incorrect OTP)...");
    const lastMessage = await waitForSMS(mockSMSServer);
    expect(lastMessage, "an SMS with the OTP should have been sent").toBeTruthy();
    console.log(`✓ SMS received with OTP: ${lastMessage!.otp}`);

    // Step 4: Enter incorrect OTP
    console.log("\nStep 4: Entering incorrect OTP (000000)...");
    await sampleAppLoginPage.fillOTP("000000");
    await sampleAppLoginPage.clickVerifyOTP();

    // Step 5: Verify error is shown
    console.log("\nStep 5: Verifying incorrect OTP is rejected...");
    const errorLocator = page.locator('.MuiAlert-colorError, [role="alert"]');
    await expect(errorLocator, "an error must be shown for an incorrect OTP").toBeVisible();
    console.log("✓ Incorrect OTP rejected - user cannot login");

    const errorText = await sampleAppLoginPage.getOTPErrorMessage();
    console.log(`  Error: ${errorText}`);

    console.log("\n--- TC002 Completed Successfully ---\n");
  });

  test("TC003: Complete MFA registration flow with mobile number and subsequent login", async ({
    sampleAppLoginPage,
    page,
  }) => {
    console.log("\n--- TC003: MFA Registration and Login Flow ---");

    // registeredUser (username/password/given_name/family_name/email) is generated at describe
    // scope so afterAll can clean it up. mobile_number has no TestDataFactory equivalent, so it's
    // generated here from a fresh timestamp suffix, the same way TestDataFactory does internally.
    const regUsername = registeredUser.username;
    const regPassword = registeredUser.password;
    const regGivenName = registeredUser.given_name;
    const regFamilyName = registeredUser.family_name;
    const regEmail = registeredUser.email;
    const regMobile = `+1234567${Date.now().toString().slice(-4)}`;

    // ========== REGISTRATION FLOW ==========

    // Step 1: Navigate to sample app
    console.log("\n[REGISTRATION] Step 1: Navigating to sample app...");
    await sampleAppLoginPage.goto(sampleAppUrl!);
    await sampleAppLoginPage.verifyHomePageLoaded();
    console.log("✓ Sample app home page loaded");

    // Step 2: Click Sign In button
    console.log("\n[REGISTRATION] Step 2: Clicking Sign In button...");
    await sampleAppLoginPage.clickSignInButton();
    await sampleAppLoginPage.verifyLoginPageLoaded();
    console.log("✓ Login page displayed");

    // Step 3: Click Sign Up link
    console.log("\n[REGISTRATION] Step 3: Clicking Sign Up link...");
    const signUpLink = page.locator('a:has-text("Sign Up"), a:has-text("sign up"), button:has-text("Sign Up")');
    await expect(signUpLink.first()).toBeVisible({ timeout: 5000 });
    await signUpLink.first().click();
    console.log("✓ Sign Up link clicked");

    // Step 4: Verify registration page - credentials form
    console.log("\n[REGISTRATION] Step 4: Verifying registration credentials page...");
    await expect(page.locator('h2:has-text("Sign Up")')).toBeVisible({ timeout: 10000 });
    await expect(page.locator('input[name="username"]')).toBeVisible();
    await expect(page.locator('input[name="password"]')).toBeVisible();
    console.log("✓ Registration credentials form displayed");

    // Step 5: Fill username and password
    console.log("\n[REGISTRATION] Step 5: Entering credentials...");
    await page.locator('input[name="username"]').fill(regUsername);
    await page.locator('input[name="password"]').fill(regPassword);
    console.log(`  Username: ${regUsername}`);
    console.log("  Password: ********");

    // Step 6: Click Continue button
    console.log("\n[REGISTRATION] Step 6: Clicking Continue button...");
    const continueButton = page.locator('button[type="submit"]:has-text("Continue")');
    await continueButton.click();
    console.log("✓ Continue button clicked");

    // Step 7: Verify user info form (with mobile number field)
    console.log("\n[REGISTRATION] Step 7: Verifying user information form...");
    await expect(page.locator('input[name="given_name"]')).toBeVisible({ timeout: 10000 });
    await expect(page.locator('input[name="family_name"]')).toBeVisible();
    await expect(page.locator('input[name="email"]')).toBeVisible();
    await expect(page.locator('input[name="mobile_number"]')).toBeVisible();
    console.log("✓ User information form displayed with mobile number field");

    // Step 8: Fill user information including mobile number
    console.log("\n[REGISTRATION] Step 8: Filling user information form...");
    await page.locator('input[name="given_name"]').fill(regGivenName);
    await page.locator('input[name="family_name"]').fill(regFamilyName);
    await page.locator('input[name="email"]').fill(regEmail);
    await page.locator('input[name="mobile_number"]').fill(regMobile);
    console.log(`  First Name: ${regGivenName}`);
    console.log(`  Last Name: ${regFamilyName}`);
    console.log(`  Email: ${regEmail}`);
    console.log(`  Mobile Number: ${regMobile}`);

    // Step 9: Submit registration, capturing the flow/execute response the click triggers
    console.log("\n[REGISTRATION] Step 9: Submitting registration...");
    const signUpButton = page.locator('button[type="submit"]:has-text("Sign Up")');
    const [registrationResponse] = await Promise.all([
      page.waitForResponse(resp => resp.url().includes("/flow/execute") && resp.request().method() === "POST", {
        timeout: Timeouts.PAGE_LOAD,
      }),
      signUpButton.click(),
    ]);
    console.log("✓ Registration form submitted");

    // Step 10: Verify registration completed server-side.
    // The response status is asserted rather than the body: a successful registration redirects the
    // app immediately, and Chromium evicts response bodies on navigation, so reading the body races
    // that redirect. Registration hands the app an assertion which it exchanges for tokens, so the
    // app ends up signed in as the new user - waiting for that state confirms the account was really
    // created and is usable, and it also lets the redirect and token exchange settle.
    console.log("\n[REGISTRATION] Step 10: Verifying registration completed server-side...");
    expect(registrationResponse.ok(), "the registration flow/execute request should succeed").toBe(true);
    await sampleAppLoginPage.verifyLoggedIn();
    console.log("✓ Registration completed - the new user is signed in");

    // ========== MFA LOGIN FLOW ==========

    console.log("\n[LOGIN] Starting MFA login flow with newly registered user...");

    // Step 11: Sign out, so the MFA login below is exercised from a clean, signed-out state
    console.log("\n[LOGIN] Step 11: Signing out before logging in again...");
    await sampleAppLoginPage.logout();
    await sampleAppLoginPage.verifyLoggedOut();
    console.log("✓ Signed out");

    await sampleAppLoginPage.goto(sampleAppUrl!);
    await sampleAppLoginPage.verifyHomePageLoaded();
    await sampleAppLoginPage.clickSignInButton();
    await sampleAppLoginPage.verifyLoginPageLoaded();
    console.log("✓ Login page displayed");

    // Step 12: Enter registered user credentials
    console.log("\n[LOGIN] Step 12: Entering registered user credentials...");
    await sampleAppLoginPage.fillLoginForm(regUsername, regPassword);
    console.log(`  Username: ${regUsername}`);
    console.log("  Password: ********");

    // Step 13: Submit login form
    console.log("\n[LOGIN] Step 13: Submitting login form...");
    await sampleAppLoginPage.clickLogin();
    console.log("✓ Login form submitted");

    // Step 14: Wait for OTP page
    console.log("\n[LOGIN] Step 14: Waiting for OTP verification page...");
    await sampleAppLoginPage.verifyOTPPageLoaded();
    console.log("✓ OTP verification page displayed");

    // Step 15: Retrieve OTP from mock SMS server
    console.log("\n[LOGIN] Step 15: Retrieving OTP from mock SMS server...");
    const lastMessage = await waitForSMS(mockSMSServer);
    expect(lastMessage).not.toBeNull();
    expect(lastMessage!.otp).toBeTruthy();
    expect(lastMessage!.otp).toMatch(/^\d{4,8}$/);

    console.log(`✓ SMS received for mobile: ${regMobile}`);
    console.log(`✓ OTP extracted: ${lastMessage!.otp}`);

    // Step 16: Enter OTP
    console.log("\n[LOGIN] Step 16: Entering OTP...");
    await sampleAppLoginPage.fillOTP(lastMessage!.otp);
    console.log(`  OTP: ${lastMessage!.otp}`);

    // Step 17: Submit OTP verification
    console.log("\n[LOGIN] Step 17: Submitting OTP verification...");
    await sampleAppLoginPage.clickVerifyOTP();
    console.log("✓ OTP verification submitted");

    // Step 18: Verify successful MFA authentication
    console.log("\n[LOGIN] Step 18: Verifying successful MFA authentication...");
    await sampleAppLoginPage.verifyLoggedIn();
    console.log("✓ MFA authentication successful - Newly registered user logged in");

    console.log("\n--- TC003 Completed Successfully ---");
    console.log("Summary: User registered with mobile number and successfully logged in with MFA");
    console.log("---\n");
  });
});
