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
 * Console Admin Authentication Utilities for Playwright E2E Tests.
 *
 * This module provides utilities to manage authenticated sessions specifically for the
 * Console admin user in end-to-end tests using Playwright.
 *
 * This application uses OAuth2/OIDC token-based authentication stored in sessionStorage,
 * NOT cookies. Therefore, we need to inject tokens via page.addInitScript() to ensure
 * they're available before the page loads.
 *
 * SECURITY NOTE: Do NOT log credentials (username/password) to the console in this file
 * or any consumers of this file to prevent leaking secrets in CI logs.
 *
 * @module authentication/console-admin-auth-utils
 */
import fs from "fs";
import path from "path";
import { Page, BrowserContext } from "@playwright/test";
import { Timeouts } from "../../constants/timeouts";

export interface StorageItem {
  name: string;
  value: string;
}

export interface CookieItem {
  name: string;
  value: string;
  domain?: string;
  path?: string;
  expires?: number;
  httpOnly?: boolean;
  secure?: boolean;
  sameSite?: "Strict" | "Lax" | "None";
}

export interface AuthState {
  cookies: CookieItem[];
  origins: Array<{
    origin: string;
    localStorage?: StorageItem[];
    sessionStorage?: StorageItem[];
  }>;
}

export interface SetupAuthenticationOptions {
  debug?: boolean;
  authFilePath?: string;
}

/**
 * Load authentication state from file
 */
export function loadAuthState(filePath: string, debug: boolean = false): AuthState {
  if (!fs.existsSync(filePath)) {
    throw new Error(`Authentication state file not found: ${filePath}`);
  }
  const authState = JSON.parse(fs.readFileSync(filePath, "utf8"));

  if (debug) {
    console.log("🔍 [DEBUG] Auth file path:", filePath);
    console.log("🔍 [DEBUG] Cookies in auth state:", authState.cookies?.length || 0);
    console.log("🔍 [DEBUG] LocalStorage items:", authState.origins?.[0]?.localStorage?.length || 0);
    console.log("🔍 [DEBUG] SessionStorage items:", authState.origins?.[0]?.sessionStorage?.length || 0);
  }

  return authState;
}

/**
 * Restore cookies to browser context (if any exist)
 */
export async function restoreCookies(
  context: BrowserContext,
  authState: AuthState,
  debug: boolean = false
): Promise<void> {
  if (!authState.cookies || authState.cookies.length === 0) {
    if (debug) {
      console.log("🔍 [DEBUG] No cookies in auth state (app uses token-based auth)");
    }
    return;
  }

  await context.addCookies(authState.cookies);
  console.log(`✅ Cookies restored: ${authState.cookies.length} cookies added to context`);
}

/**
 * Create init script to inject storage state BEFORE page loads
 * This is critical for OAuth2/OIDC apps that check tokens on page load
 */
export function createStorageInitScript(authState: AuthState): string {
  const origin = authState.origins?.[0];
  if (!origin) {
    return "";
  }

  const localStorage = origin.localStorage || [];
  const sessionStorage = origin.sessionStorage || [];

  // Create a script that injects storage items
  const script = `
    (function() {
      // Inject localStorage items
      ${localStorage
        .map(
          item =>
            `try { localStorage.setItem(${JSON.stringify(item.name)}, ${JSON.stringify(item.value)}); } catch(e) {}`
        )
        .join("\n      ")}
      
      // Inject sessionStorage items
      ${sessionStorage
        .map(
          item =>
            `try { sessionStorage.setItem(${JSON.stringify(item.name)}, ${JSON.stringify(item.value)}); } catch(e) {}`
        )
        .join("\n      ")}
    })();
  `;

  return script;
}

/**
 * Verify that authentication is working by checking storage and session state
 */
export async function verifyAuthState(page: Page, baseUrl: string, debug: boolean = false): Promise<boolean> {
  const localStorage = await page.evaluate(() => {
    const items: Record<string, string> = {};
    for (let i = 0; i < window.localStorage.length; i++) {
      const key = window.localStorage.key(i);
      if (key) {
        items[key] = window.localStorage.getItem(key) || "";
      }
    }
    return items;
  });

  const sessionStorage = await page.evaluate(() => {
    const items: Record<string, string> = {};
    for (let i = 0; i < window.sessionStorage.length; i++) {
      const key = window.sessionStorage.key(i);
      if (key) {
        items[key] = window.sessionStorage.getItem(key) || "";
      }
    }
    return items;
  });

  const isSessionActive = localStorage["thunderid-session-active"] === "true";
  const hasSessionData = Object.keys(sessionStorage).some(key => key.includes("session_data-instance_0"));

  // Check if tokens exist and are not expired
  let tokensValid = false;
  let accessToken: string | undefined;
  const sessionDataKey = Object.keys(sessionStorage).find(key => key.includes("session_data-instance_0"));
  if (sessionDataKey) {
    try {
      const sessionData = JSON.parse(sessionStorage[sessionDataKey]);
      if (sessionData.access_token && sessionData.created_at && sessionData.expires_in) {
        const expirationTime = sessionData.created_at + sessionData.expires_in * 1000;
        tokensValid = Date.now() < expirationTime;
        accessToken = sessionData.access_token;
        if (debug) {
          const timeLeft = Math.round((expirationTime - Date.now()) / 1000 / 60);
          console.log(`🔍 [DEBUG] Token expires in: ${timeLeft} minutes`);
          if (!tokensValid) {
            console.log("⚠️ [DEBUG] ACCESS TOKEN EXPIRED! Need to re-run auth.setup.ts");
          }
        }
      }
    } catch {
      if (debug) console.log("🔍 [DEBUG] Could not parse session data");
    }
  }

  if (debug) {
    console.log("🔍 [DEBUG] localStorage keys:", Object.keys(localStorage));
    console.log("🔍 [DEBUG] sessionStorage keys:", Object.keys(sessionStorage));
    console.log(
      `🔍 [DEBUG] Session active: ${isSessionActive}, Has session data: ${hasSessionData}, Tokens valid: ${tokensValid}`
    );
  }

  console.log(
    `🔍 Auth verification: session active: ${isSessionActive}, has session data: ${hasSessionData}, tokens valid: ${tokensValid}`
  );

  if (!isSessionActive || !hasSessionData || !tokensValid) {
    return false;
  }

  // The checks above are all client-side and can't detect a token the server has already
  // rejected (e.g. an in-flight request interrupted by a crashed worker leaving the session in
  // a bad state)
  try {
    const response = await page.request.get(`${baseUrl}/oauth2/userinfo`, {
      headers: { Authorization: `Bearer ${accessToken}` },
      ignoreHTTPSErrors: true,
    });
    if (!response.ok()) {
      if (debug) console.log(`⚠️ [DEBUG] Server rejected stored token: ${response.status()}`);
      return false;
    }
  } catch (error) {
    if (debug) console.log("⚠️ [DEBUG] Server verification request failed:", error);
    return false;
  }

  return true;
}

/**
 * Setup authentication for a test by loading and injecting auth state.
 * If auth file doesn't exist or tokens are expired, performs inline login.
 */
export async function setupAuthentication(
  page: Page,
  baseUrl: string,
  options: SetupAuthenticationOptions = {}
): Promise<void> {
  const { debug = false, authFilePath } = options;

  // Default auth file path
  const defaultAuthPath = path.join(__dirname, "../../playwright/.auth/console-admin.json");
  const authPath = authFilePath || defaultAuthPath;

  console.log("Setting up authentication...");

  if (debug) {
    console.log("🔍 [DEBUG] Debug mode enabled");
    console.log("🔍 [DEBUG] Base URL:", baseUrl);
  }

  // Check if auth file exists
  if (!fs.existsSync(authPath)) {
    console.log("⚠️ Auth file not found, performing inline login...");
    await performInlineLogin(page, baseUrl, authPath, debug);
    return;
  }

  // Load authentication state
  const authState = loadAuthStateNoThrow(authPath, debug);

  if (!authState) {
    console.log("⚠️ Failed to load auth state, performing inline login...");
    await performInlineLogin(page, baseUrl, authPath, debug);
    return;
  }

  // Check if tokens are expired
  const tokensExpired = checkTokensExpired(authState, debug);
  if (tokensExpired) {
    console.log("⚠️ Tokens expired, performing inline login...");
    await performInlineLogin(page, baseUrl, authPath, debug);
    return;
  }

  console.log(
    `Loaded auth state: ${authState.origins?.[0]?.localStorage?.length || 0} localStorage, ${authState.origins?.[0]?.sessionStorage?.length || 0} sessionStorage items`
  );

  // Get the browser context
  const context = page.context();

  // Restore cookies if any exist
  await restoreCookies(context, authState, debug);

  // CRITICAL: Add init script to inject storage BEFORE page loads
  const initScript = createStorageInitScript(authState);
  if (initScript) {
    await context.addInitScript(initScript);
    if (debug) {
      console.log("🔍 [DEBUG] Added init script to inject storage on page load");
    }
  }

  // Navigate to base URL - storage will be injected automatically
  await page.goto(baseUrl, { waitUntil: "domcontentloaded" });

  if (debug) {
    console.log("🔍 [DEBUG] Page URL after navigation:", page.url());
  }

  // Wait for the app to settle after token injection
  await page.waitForLoadState("networkidle");

  // Verify authentication is working
  const isValid = await verifyAuthState(page, baseUrl, debug);
  if (!isValid) {
    console.log("⚠️ Auth verification failed, performing inline login...");
    await performInlineLogin(page, baseUrl, authPath, debug);
  }
}

/**
 * Load auth state without throwing - returns null on error
 */
function loadAuthStateNoThrow(filePath: string, debug: boolean): AuthState | null {
  try {
    return loadAuthState(filePath, debug);
  } catch (error) {
    if (debug) {
      console.error("⚠️ [DEBUG] Failed to load auth state from file:", filePath, error);
    }
    return null;
  }
}

/**
 * Check if tokens in auth state are expired
 */
function checkTokensExpired(authState: AuthState, debug: boolean): boolean {
  const sessionDataKey = authState.origins?.[0]?.sessionStorage?.find(item =>
    item.name.includes("session_data-instance_0")
  );

  if (!sessionDataKey) {
    return true; // No session data = expired
  }

  try {
    const sessionData = JSON.parse(sessionDataKey.value);
    if (sessionData.access_token && sessionData.created_at && sessionData.expires_in) {
      const expirationTime = sessionData.created_at + sessionData.expires_in * 1000;
      const isExpired = Date.now() >= expirationTime;
      if (debug) {
        const timeLeft = Math.round((expirationTime - Date.now()) / 1000 / 60);
        console.log(`🔍 [DEBUG] Token expires in: ${timeLeft} minutes`);
      }
      return isExpired;
    }
  } catch (error) {
    if (debug) {
      console.error("🔍 [DEBUG] Failed to parse session data for token expiry check:", {
        error,
      });
    }
    return true;
  }
  return true;
}

/**
 * Perform inline login when auth file doesn't exist or tokens expired
 */
async function performInlineLogin(page: Page, baseUrl: string, authPath: string, debug: boolean): Promise<void> {
  const username = process.env.ADMIN_USERNAME;
  const password = process.env.ADMIN_PASSWORD;

  if (!username || !password) {
    throw new Error(
      `ADMIN_USERNAME and ADMIN_PASSWORD environment variables are required for inline login.
Please ensure they are set in your .env file or the test environment configuration.`
    );
  }

  console.log("🔐 Performing inline login...");

  // Navigate to console page (will redirect to login)
  await page.goto(`${baseUrl}/console`, { waitUntil: "networkidle" });

  // Wait for login form
  await page.waitForSelector('input[name="username"], input[type="text"]', { timeout: Timeouts.FORM_LOAD });

  // Fill credentials
  try {
    await page.fill('input[name="username"]', username);
  } catch {
    await page.fill('input[type="text"]', username);
  }

  try {
    await page.fill('input[name="password"]', password);
  } catch {
    await page.fill('input[type="password"]', password);
  }

  // Click sign in
  const signInButton = page
    .locator('button[type="submit"]')
    .or(page.getByRole("button", { name: /sign in|login|submit/i }));
  await signInButton.first().click();

  // Wait for redirect to console page
  await page.waitForURL("**/console/**", { timeout: Timeouts.REDIRECT });
  await page.waitForLoadState("networkidle");

  console.log("✅ Inline login successful!");

  // Save auth state for future tests
  await saveAuthState(page, baseUrl, authPath, debug);
}

/**
 * Save authentication state to file
 */
async function saveAuthState(page: Page, baseUrl: string, authPath: string, debug: boolean): Promise<void> {
  const context = page.context();
  const authDir = path.dirname(authPath);

  // Ensure directory exists
  if (!fs.existsSync(authDir)) {
    fs.mkdirSync(authDir, { recursive: true });
  }

  const cookies = await context.cookies();
  const localStorage = await page.evaluate(() => {
    const items: { name: string; value: string }[] = [];
    for (let i = 0; i < window.localStorage.length; i++) {
      const key = window.localStorage.key(i);
      if (key) items.push({ name: key, value: window.localStorage.getItem(key) || "" });
    }
    return items;
  });

  const sessionStorage = await page.evaluate(() => {
    const items: { name: string; value: string }[] = [];
    for (let i = 0; i < window.sessionStorage.length; i++) {
      const key = window.sessionStorage.key(i);
      if (key) items.push({ name: key, value: window.sessionStorage.getItem(key) || "" });
    }
    return items;
  });

  const storageState = {
    cookies,
    origins: [
      {
        origin: baseUrl,
        localStorage,
        sessionStorage,
      },
    ],
  };

  fs.writeFileSync(authPath, JSON.stringify(storageState, null, 2));
  console.log("💾 Auth state saved to:", authPath);

  if (debug) {
    console.log(
      `🔍 [DEBUG] Saved: ${cookies.length} cookies, ${localStorage.length} localStorage, ${sessionStorage.length} sessionStorage`
    );
  }
}
