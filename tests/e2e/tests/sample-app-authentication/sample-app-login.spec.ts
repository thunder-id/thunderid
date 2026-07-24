/* eslint-disable playwright/require-top-level-describe */
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

import { test } from "../../fixtures/sample-app";
import { getAdminToken } from "../../utils/authentication";
import { TestDataFactory } from "../../utils/test-data";
import { TestTags } from "../../constants/test-tags";

const sampleAppUrl = process.env.SAMPLE_APP_URL;
const serverUrl = process.env.SERVER_URL || "https://localhost:8090";

// Skip tests if SAMPLE_APP_URL is not provided
const describeOrSkip = sampleAppUrl ? test.describe : test.describe.skip;

describeOrSkip("Sample App - Login and Logout", { tag: [TestTags.AUTHENTICATION] }, () => {
  const testUser = TestDataFactory.createUser();
  let userId: string | null = null;

  test.beforeAll(async ({ request }) => {
    console.log("\n=== Login Test Suite Setup ===");
    const adminToken = await getAdminToken(request);

    const userTypesResponse = await request.get(`${serverUrl}/user-types`, {
      headers: { Authorization: `Bearer ${adminToken}` },
      ignoreHTTPSErrors: true,
    });
    if (!userTypesResponse.ok()) {
      throw new Error(`Failed to fetch user types: ${await userTypesResponse.text()}`);
    }
    const userTypesData = await userTypesResponse.json();
    const personType = userTypesData.types?.find((t: any) => t.name === "Person");
    if (!personType?.ouId) {
      throw new Error("Person user type not found or missing organization unit");
    }

    const createResponse = await request.post(`${serverUrl}/users`, {
      data: {
        type: "Person",
        ouId: personType.ouId,
        attributes: {
          username: testUser.username,
          password: testUser.password,
          given_name: testUser.given_name,
          email: testUser.email,
        },
      },
      headers: { Authorization: `Bearer ${adminToken}` },
      ignoreHTTPSErrors: true,
    });
    if (!createResponse.ok()) {
      throw new Error(`Failed to create test user: ${await createResponse.text()}`);
    }
    const createdUser = await createResponse.json();
    userId = createdUser.id;
    console.log(`✓ Test user created: ${userId}`);
    console.log("=========================\n");
  });

  test.afterAll(async ({ request }) => {
    if (!userId) return;

    console.log("\n=== Login Test Suite Teardown ===");
    const adminToken = await getAdminToken(request);
    const deleteResponse = await request.delete(`${serverUrl}/users/${userId}`, {
      headers: { Authorization: `Bearer ${adminToken}` },
      ignoreHTTPSErrors: true,
    });
    if (deleteResponse.ok()) {
      console.log(`✓ Test user deleted: ${userId}`);
    } else {
      console.log(`⚠️  Failed to delete test user ${userId}: ${deleteResponse.status()}`);
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
