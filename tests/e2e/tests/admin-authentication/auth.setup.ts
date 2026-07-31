// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Authentication Setup for Console E2E Tests
 *
 * This setup file performs admin login and saves the authentication state
 * (cookies, localStorage, sessionStorage) to a JSON file. Other tests can
 * then reuse this session via Playwright's storageState feature.
 *
 * Required environment variables:
 * - ADMIN_USERNAME: Admin user login name
 * - ADMIN_PASSWORD: Admin user password
 * - BASE_URL: Console base URL
 *
 * @see https://playwright.dev/docs/auth
 */

import { setup, routes } from "../../fixtures/console";
import path from "path";
import fs from "fs";
import { Timeouts } from "../../constants/timeouts";

/** Path to save authentication state */
const AUTH_FILE = path.join(__dirname, "../../playwright/.auth/console-admin.json");

setup("Admin login test", async ({ page, context, signinPage }) => {
  const authDir = path.dirname(AUTH_FILE);
  if (!fs.existsSync(authDir)) {
    fs.mkdirSync(authDir, { recursive: true });
  }

  const username = process.env.ADMIN_USERNAME as string;
  const password = process.env.ADMIN_PASSWORD as string;
  const baseUrl = process.env.BASE_URL as string;
  if (!baseUrl) {
    throw new Error(
      "BASE_URL is not defined in environment variables. Please check your .env file or environment configuration."
    );
  }

  console.log("🔐 Starting authentication process...");
  console.log("Base URL:", baseUrl);

  // Capture auth-related network requests for real-time debugging
  page.on("response", response => {
    if (response.url().includes("/token") || response.url().includes("/auth") || response.url().includes("/session")) {
      console.log(`📡 Auth Response: ${response.status()} ${response.url()}`);
    }
  });

  // Navigate to develop page (redirects to login if not authenticated)
  console.log("🌐 Navigating to develop page for authentication...");
  await signinPage.gotoHome();

  console.log("⏳ Waiting for redirect to authentication page...");
  try {
    // Wait for URL to contain 'signin' or 'auth' or 'login'
    await page.waitForURL(
      url => url.pathname.includes("signin") || url.pathname.includes("auth") || url.pathname.includes("login"),
      { timeout: 10000 }
    );
    console.log("✅ Redirected to authentication page:", page.url());
  } catch {
    console.log("ℹ️ No redirect detected after 10s. Current URL:", page.url());
    if (!(await signinPage.isOnLoginPage())) {
      console.log("⚠️ Not on login page, attempting direct navigation to signin...");
      await signinPage.goto();
      console.log("Current URL after direct navigation:", page.url());
    }
  }

  await signinPage.screenshot("debug-login-page");

  // Use POM to login
  console.log("📝 Filling login credentials...");
  try {
    await signinPage.waitForLoginForm();
  } catch (error) {
    console.log("❌ Login form not visible after timeout. Current URL:", page.url());
    const content = await page.content();
    console.log("📄 Page content snippet:", content.substring(0, 500));
    await signinPage.screenshot("debug-login-form-timeout");
    throw error;
  }
  await signinPage.fillUsername(username);
  await signinPage.fillPassword(password);

  await signinPage.screenshot("debug-before-signin");

  await signinPage.clickSignIn();

  console.log("⏳ Waiting for authentication to complete...");

  try {
    await signinPage.waitForLoginSuccess();
    console.log("✅ Redirected to develop page");
  } catch (error) {
    console.log("❌ Failed to redirect to develop page");
    await signinPage.screenshot("debug-after-signin-failed");
    throw error;
  }

  await signinPage.verifyLoginSuccess();
  console.log("✅ Login verified via URL check");

  console.log("⏳ Waiting for authentication to be fully established...");
  await page.waitForLoadState("networkidle");

  // Check storage availability (using raw page evaluate as this is somewhat internal state check)
  await page.evaluate(() => {
    if (window.localStorage) console.log("LocalStorage available");
    if (window.sessionStorage) console.log("SessionStorage available");
  });

  // Small delay to ensure cookies and session are fully established
  await page.waitForTimeout(Timeouts.POST_AUTH);

  console.log("✅ Authentication successful!");

  console.log("🧪 Testing navigation to protected page...");
  try {
    await page.goto(`${baseUrl}${routes.applications}`, { waitUntil: "domcontentloaded", timeout: Timeouts.FORM_LOAD });
    if (!page.url().includes(routes.signin)) {
      console.log("✅ Can access protected applications page - auth working");
    } else {
      console.log("❌ Redirected to signin when accessing applications page");
    }
  } catch (error) {
    console.log("⚠️ Error testing protected page:", String(error));
  }

  await signinPage.gotoHome();

  console.log("🍪 Capturing cookies from current session...");

  const allCookies = await context.cookies();
  console.log("All cookies from context:", allCookies.length);

  const urlCookies = await context.cookies(baseUrl);
  console.log("URL-specific cookies:", urlCookies.length);

  const pageCookies = await page.evaluate(() => {
    return document.cookie
      .split(";")
      .map(cookie => {
        const [name, ...rest] = cookie.split("=");
        return { name: name.trim(), value: rest.join("=").trim() };
      })
      .filter(cookie => cookie.name && cookie.value);
  });
  console.log("Page-accessible cookies:", pageCookies.length);

  const capturedCookies = allCookies.length > 0 ? allCookies : urlCookies;

  console.log("🍪 Final cookie summary:");
  console.log(`  - Context cookies: ${allCookies.length}`);
  console.log(`  - URL cookies: ${urlCookies.length}`);
  console.log(`  - Page cookies: ${pageCookies.length}`);

  if (capturedCookies.length > 0) {
    capturedCookies.forEach(cookie => {
      console.log(`  ✓ ${cookie.name}: ${cookie.domain} (httpOnly: ${cookie.httpOnly}, secure: ${cookie.secure})`);
    });
  } else {
    console.warn("⚠️ No cookies captured! This will likely cause authentication issues.");
    await signinPage.screenshot("debug-no-cookies-captured");
  }

  const storageState = {
    cookies: capturedCookies,
    origins: [
      {
        origin: baseUrl,
        localStorage: await page.evaluate(() => {
          const items = [];
          for (let i = 0; i < localStorage.length; i++) {
            const key = localStorage.key(i);
            if (key) items.push({ name: key, value: localStorage.getItem(key) });
          }
          return items;
        }),
        sessionStorage: await page.evaluate(() => {
          const items = [];
          for (let i = 0; i < sessionStorage.length; i++) {
            const key = sessionStorage.key(i);
            if (key) items.push({ name: key, value: sessionStorage.getItem(key) });
          }
          return items;
        }),
      },
    ],
  };

  fs.writeFileSync(AUTH_FILE, JSON.stringify(storageState, null, 2));
  console.log("💾 Enhanced authentication state saved to:", AUTH_FILE);

  const workingAuthFile = path.join(authDir, "working-login.json");
  fs.writeFileSync(workingAuthFile, JSON.stringify(storageState, null, 2));
  console.log("💾 Working authentication state also saved to:", workingAuthFile);

  console.log("📋 Saved state summary:");
  console.log(`  - Cookies: ${storageState.cookies.length}`);
  console.log(`  - Origins: ${storageState.origins.length}`);
  console.log(`  - LocalStorage items: ${storageState.origins[0]?.localStorage?.length || 0}`);
  console.log(`  - SessionStorage items: ${storageState.origins[0]?.sessionStorage?.length || 0}`);
});
