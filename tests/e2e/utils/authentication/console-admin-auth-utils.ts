// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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

  // CRITICAL: Add init script to inject storage BEFORE page loads. checkTokensExpired above
  // already confirmed the token isn't expired, and the test itself navigates to its real target
  // route right after this fixture resolves - that navigation is what applies this init script,
  // so there's no need to load the console here too just to re-check what it would find.
  const initScript = createStorageInitScript(authState);
  if (initScript) {
    await context.addInitScript(initScript);
    if (debug) {
      console.log("🔍 [DEBUG] Added init script to inject storage on page load");
    }
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
  await page.goto(`${baseUrl}/console`, { timeout: Timeouts.PAGE_LOAD });

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

  // Wait for redirect to console page, then for the SDK to actually write the session it
  // establishes there - checkTokensExpired() above reads this same sessionStorage key, and
  // saveAuthState() below would otherwise capture storage before the token exists.
  await page.waitForURL("**/console/**", { timeout: Timeouts.REDIRECT });
  await page.waitForFunction(() => Object.keys(sessionStorage).some(key => key.includes("session_data-instance_0")), {
    timeout: Timeouts.PAGE_LOAD,
  });

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
