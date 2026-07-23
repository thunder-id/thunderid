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
 * Welcome Try-It-Out E2E Tests
 *
 * Exercises the console welcome guide for the "Secured Web Application" journey:
 *   1. Welcome landing → Secured Web Application tile
 *   2. Sample setup section → Download button triggers a real download
 *   3. Configure/Import bundle into ThunderID
 *   4. Try-out use cases: Sign-In and Self Sign-Up against a preconfigured wayfinder sample
 *
 * The wayfinder sample is expected to be pre-started. Tests are skipped if WAYFINDER_APP_URL is not provided.
 *
 * Required environment variables:
 * - BASE_URL: Console base URL
 * - WAYFINDER_APP_URL: Pre-started wayfinder sample URL (e.g. http://localhost:5173)
 * - WAYFINDER_TEST_USERNAME: Seed user shipped with the wayfinder bundle (default: john.doe)
 * - WAYFINDER_TEST_PASSWORD: Seed user password (default: john.doe)
 * - SERVER_URL: ThunderID server URL for post-test user cleanup (default: https://localhost:8090)
 * - ADMIN_USERNAME / ADMIN_PASSWORD: Admin creds used for cleanup
 */

import { test, expect } from "../../fixtures/console";
import { SampleAppLoginPage } from "../../pages/sample-app";
import { getAdminToken } from "../../utils/authentication";

const wayfinderUrl = process.env.WAYFINDER_APP_URL || "http://localhost:5173";
const seedUsername = process.env.WAYFINDER_TEST_USERNAME || "john.doe";
const seedPassword = process.env.WAYFINDER_TEST_PASSWORD || "john.doe";
const serverUrl = process.env.SERVER_URL || "https://localhost:8090";

test.describe("Console Welcome — Try It Out (Secured Web Application)", () => {
  const createdSignupUsernames: string[] = [];

  test.afterAll(async ({ request }) => {
    if (createdSignupUsernames.length === 0) return;
    try {
      const token = await getAdminToken(request);
      for (const username of createdSignupUsernames) {
        try {
          const res = await request.get(`${serverUrl}/users?filter=username eq "${username}"`, {
            headers: { Authorization: `Bearer ${token}` },
            ignoreHTTPSErrors: true,
          });
          if (!res.ok()) continue;
          const data = await res.json();
          const userId = data?.users?.[0]?.id;
          if (!userId) continue;
          await request.delete(`${serverUrl}/users/${userId}`, {
            headers: { Authorization: `Bearer ${token}` },
            ignoreHTTPSErrors: true,
          });
          console.log(`Cleaned up signup test user: ${username} (${userId})`);
        } catch (e) {
          console.warn(`Failed to clean up ${username}:`, e);
        }
      }
    } catch (e) {
      console.warn("User cleanup skipped (admin token acquisition failed):", e);
    }
  });

  /** TC001: Welcome → Try-out sample setup → Download works → Import succeeds → Sign-In tab renders */
  test("TC001: Welcome guide loads and drives user through sample setup and sign-in tryout", async ({
    welcomePage,
    page,
  }) => {
    await test.step("Open welcome landing", async () => {
      await welcomePage.goto();
      await welcomePage.verifyWelcomeLoaded();
      await welcomePage.screenshot("welcome-tc001-landing");
    });

    await test.step("Open Secured Web Application try-out", async () => {
      await welcomePage.clickSecuredWebAppTile();
      await welcomePage.verifyTryoutPageLoaded();
      await welcomePage.screenshot("welcome-tc001-tryout-open");
    });

    await test.step("Download button triggers a wayfinder sample download", async () => {
      const download = await welcomePage.triggerDownload();
      console.log("Download suggested filename:", download.suggestedFilename());
      // Cancel so we don't persist the multi-MB zip.
      await download.cancel().catch(() => {});
    });

    await test.step("Configure wayfinder bundle in ThunderID", async () => {
      await welcomePage.importWayfinderBundle();
      await welcomePage.screenshot("welcome-tc001-imported");
    });

    await test.step("Sign-In tryout — perform login against preconfigured wayfinder sample", async () => {
      await welcomePage.selectSignInTab();
      const sampleAppLoginPage = new SampleAppLoginPage(page);
      await sampleAppLoginPage.goto(wayfinderUrl);
      await sampleAppLoginPage.verifyHomePageLoaded();
      await sampleAppLoginPage.clickSignInButton();
      await sampleAppLoginPage.verifyLoginPageLoaded();
      await sampleAppLoginPage.login(seedUsername, seedPassword);
      await sampleAppLoginPage.verifyLoggedIn();
    });
  });

  /** TC002: Self Sign-Up tryout — register a new user via the wayfinder sample gate */
  test("TC002: Self Sign-Up tryout — register new user via the wayfinder sample", async ({ welcomePage, page }) => {
    const timestamp = Date.now();
    const signupUsername = `welcome-signup-${timestamp}`;
    const signupPassword = "SignupTest@123";
    const signupEmail = `${signupUsername}@example.com`;

    await test.step("Ensure wayfinder bundle is configured", async () => {
      await welcomePage.goto();
      await welcomePage.clickSecuredWebAppTile();
      await welcomePage.verifyTryoutPageLoaded();
      await welcomePage.importWayfinderBundle();
    });

    await test.step("Open Self Sign-Up tryout tab", async () => {
      await welcomePage.selectSignUpTab();
      await welcomePage.screenshot("welcome-tc002-signup-tab");
    });

    await test.step("Register new user through wayfinder sample gate", async () => {
      const sampleAppLoginPage = new SampleAppLoginPage(page);
      await sampleAppLoginPage.goto(wayfinderUrl);
      await sampleAppLoginPage.verifyHomePageLoaded();
      await sampleAppLoginPage.clickSignInButton();
      await sampleAppLoginPage.verifyLoginPageLoaded();

      const signUpLink = page
        .locator('a:has-text("Sign Up"), a:has-text("sign up"), button:has-text("Sign Up"), button:has-text("Sign up")')
        .first();
      await expect(signUpLink).toBeVisible({ timeout: 10000 });
      await signUpLink.click();

      await page.locator('input[name="username"]').first().fill(signupUsername);
      await page.locator('input[name="password"]').first().fill(signupPassword);

      const continueButton = page
        .locator('button[type="submit"]:has-text("Continue"), button:has-text("Continue")')
        .first();
      await continueButton.click();

      await expect(page.locator('input[name="email"]').first()).toBeVisible({ timeout: 10000 });
      await page.locator('input[name="email"]').first().fill(signupEmail);
      await page.locator('input[name="given_name"]').first().fill("Welcome");
      await page.locator('input[name="family_name"]').first().fill("Signup");

      const submitButton = page
        .locator(
          'button[type="submit"]:has-text("Sign Up"), button[type="submit"]:has-text("Sign up"), button[type="submit"]:has-text("Submit")'
        )
        .first();
      await submitButton.click();

      // Successful signup should land the user in the authenticated wayfinder shell.
      const avatarOrLogout = page.locator('button[aria-haspopup="true"], button:has(div[class*="MuiAvatar"])').first();
      await avatarOrLogout.waitFor({ state: "visible", timeout: 30000 });

      createdSignupUsernames.push(signupUsername);
      await new SampleAppLoginPage(page).screenshot("welcome-tc002-signed-up");
    });
  });
});
