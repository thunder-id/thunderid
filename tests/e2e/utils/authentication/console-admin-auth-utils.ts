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
import { ConsoleSigninPage } from "../../pages/authentication/console-signin.page";

/** Re-login this far ahead of the real expiry, so a token can't lapse mid-test. */
const TOKEN_EXPIRY_MARGIN_MS = Timeouts.GLOBAL_TEST;

/** Handle returned by context.addInitScript(), used to remove that specific script later. */
type InitScriptHandle = Awaited<ReturnType<BrowserContext["addInitScript"]>>;

/**
 * Substring of the sessionStorage key the @thunderid SDK stores its OIDC session (access/refresh
 * tokens, expiry) under. Shared between checkTokensExpired() and resyncStorageBeforeNavigation()
 * so both agree on exactly which key represents "the session," and nothing else does.
 */
const SESSION_DATA_KEY_SUBSTRING = "session_data-instance_0";

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

  // Restore by `url` rather than the captured `domain`/`path`. Firefox's cookie manager
  // round-trips its own `domain` field inconsistently between captures (e.g. "localhost" vs
  // ".localhost" for the same cookie) and then rejects its own output on replay with
  // NS_ERROR_ILLEGAL_VALUE. Every cookie here is first-party to the one origin captured
  // alongside it, so `url` lets each browser derive scope itself instead of us replaying a
  // browser-reported domain value back into that same browser's stricter validator.
  const origin = authState.origins?.[0]?.origin;
  const cookies = authState.cookies.map(({ name, value, expires, httpOnly, secure, sameSite }) => ({
    name,
    value,
    expires,
    httpOnly,
    secure,
    sameSite,
    url: origin,
  }));

  await context.addCookies(cookies);
  console.log(`✅ Cookies restored: ${cookies.length} cookies added to context`);
}

/**
 * Create init script to inject storage state BEFORE page loads
 * This is critical for OAuth2/OIDC apps that check tokens on page load
 *
 * context.addInitScript() runs this on every new document in the context, not just the console -
 * including any popup a test opens (e.g. a future OAuth consent flow) that lands on a different
 * origin. Without an origin check, this would blindly write the console's session data into
 * whatever unrelated origin loads next. Guarded here so both callers (setupAuthentication and
 * resyncStorageBeforeNavigation) get this for free.
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
      if (window.location.origin !== ${JSON.stringify(origin.origin)}) return;

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
 *
 * Returns the resolved per-worker auth file path, so the caller can re-save whatever session
 * the page ends up holding once the test is done with it (see saveAuthState's doc comment).
 */
export async function setupAuthentication(
  page: Page,
  baseUrl: string,
  options: SetupAuthenticationOptions = {}
): Promise<string> {
  const { debug = false, authFilePath } = options;

  // Default auth file path. Keyed by worker index: the backend rotates refresh tokens and
  // revokes the whole token family if a used one is replayed (see refresh_token.go), so two
  // workers sharing one file can race a silent refresh and lock each other out. Workers run
  // their tests sequentially, so a per-worker file never sees that race.
  const workerIndex = process.env.TEST_PARALLEL_INDEX ?? "0";
  const defaultAuthPath = path.join(__dirname, `../../playwright/.auth/console-admin-${workerIndex}.json`);
  const authPath = authFilePath || defaultAuthPath;

  console.log("Setting up authentication...");

  if (debug) {
    console.log("🔍 [DEBUG] Debug mode enabled");
    console.log("🔍 [DEBUG] Base URL:", baseUrl);
  }

  // Missing, unreadable and expired all mean the same thing here: log in and rewrite the file.
  const authState = loadAuthStateNoThrow(authPath, debug);

  if (!authState) {
    console.log("⚠️ No usable auth state, performing inline login...");
    await performInlineLogin(page, baseUrl, authPath, debug);
    return authPath;
  }

  if (checkTokensExpired(authState, debug)) {
    console.log("⚠️ Tokens expired, performing inline login...");
    await performInlineLogin(page, baseUrl, authPath, debug);
    return authPath;
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

  return authPath;
}

/**
 * Re-inject the SDK's own session key as an additional init script, so the next navigation on
 * this page doesn't revert it to the snapshot addInitScript was originally registered with.
 * addInitScript re-runs its original script, unchanged, on every new document for the life of the
 * context - including a plain page.reload() or a mid-test page.goto() - so if the SDK silently
 * rotated the access/refresh token pair earlier in this test (see saveAuthState's doc comment), a
 * later navigation would otherwise replay the stale, already-used refresh token and trigger the
 * same family revocation mid-test instead of just across tests.
 *
 * Deliberately scoped to just that one sessionStorage key (see SESSION_DATA_KEY_SUBSTRING) -
 * never the rest of storage. A test can register its own init script for unrelated state between
 * fixture setup and its own navigation (e.g. welcome-screen.spec.ts's simulateFirstStart()
 * clearing the welcome-dismissed flag right before its own goto()); reinjecting a full snapshot of
 * whatever this page happened to hold before this navigation would silently overwrite that,
 * undoing the test's own setup instead of just protecting the session.
 *
 * Pass the handle this returned last time as `previous` so it gets disposed (removed) right
 * before the fresh one is registered - otherwise every navigation in a test would pile on another
 * script forever. Callers should hold on to the returned handle and pass it back in on the next
 * call; the very first call in a test has nothing to pass.
 *
 * No-ops quietly (returning `previous` unchanged) if there's no session key to resync yet - e.g.
 * the page is still on about:blank before the test's first navigation, it ended up on a different
 * origin (see saveAuthState's origin guard for why that's treated the same way), or the SDK simply
 * hasn't written a session yet.
 */
export async function resyncStorageBeforeNavigation(
  page: Page,
  baseUrl: string,
  previous?: InitScriptHandle
): Promise<InitScriptHandle | undefined> {
  if (safeOrigin(page.url()) !== safeOrigin(baseUrl)) {
    return previous;
  }

  // Scoped to the SDK's own session key only - never the rest of sessionStorage (or any of
  // localStorage). A test can register its own later init script to manipulate other state (e.g.
  // welcome-screen.spec.ts's simulateFirstStart() clearing the welcome-dismissed flag right before
  // its own goto()); reinjecting a full snapshot capturing whatever this page held before this
  // navigation would silently overwrite that, undoing the test's own setup.
  const sessionEntries = await page
    .evaluate(
      keySubstring =>
        Object.entries({ ...window.sessionStorage })
          .filter(([name]) => name.includes(keySubstring))
          .map(([name, value]) => ({ name, value })),
      SESSION_DATA_KEY_SUBSTRING
    )
    .catch(() => null);

  if (!sessionEntries || sessionEntries.length === 0) {
    return previous;
  }

  const script = createStorageInitScript({
    cookies: [],
    origins: [{ origin: baseUrl, sessionStorage: sessionEntries }],
  });

  if (!script) {
    return previous;
  }

  const handle = await page.context().addInitScript(script);
  await previous?.dispose();
  return handle;
}

/** Origin of a URL, or null if it can't be parsed as one. */
function safeOrigin(url: string): string | null {
  try {
    return new URL(url).origin;
  } catch {
    return null;
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
    item.name.includes(SESSION_DATA_KEY_SUBSTRING)
  );

  if (!sessionDataKey) {
    return true; // No session data = expired
  }

  try {
    const sessionData = JSON.parse(sessionDataKey.value);
    if (sessionData.access_token && sessionData.created_at && sessionData.expires_in) {
      const expirationTime = sessionData.created_at + sessionData.expires_in * 1000;
      // Treat a token that expires during the test as already expired - reusing one with seconds
      // left buys nothing and costs a mid-test 401 that reads as a product failure.
      const isExpired = Date.now() + TOKEN_EXPIRY_MARGIN_MS >= expirationTime;
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
 * Wait for redirect to the console page, then for the SDK to actually write the session it
 * establishes there - checkTokensExpired() reads this same sessionStorage key, and
 * saveAuthState() would otherwise capture storage before the token exists.
 */
async function waitForSessionData(page: Page): Promise<void> {
  await page.waitForURL("**/console/**", { timeout: Timeouts.REDIRECT });

  // The welcome key is the later of the two: WelcomeRedirect marks itself dismissed in
  // sessionStorage the first time a signed-in session renders a non-/welcome route (see
  // WelcomeRedirect.tsx), which is a useEffect that only runs once the session token exists.
  // Capturing state before that effect commits replays a session that looks brand new to every
  // consumer of this file, redirecting whatever page they navigate to next into /welcome instead
  // of the page the test actually asked for. Both are still checked because a retried sign-in
  // starts with the previous attempt's welcome key already in storage.
  await page.waitForFunction(
    () => {
      const keys = Object.keys(sessionStorage);
      return (
        keys.some(key => key.includes("session_data-instance_0")) &&
        keys.some(key => key.endsWith(":welcome:dismissed"))
      );
    },
    undefined,
    { timeout: Timeouts.PAGE_LOAD }
  );
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

  // Enter through /console rather than the sign-in route: it's the console's own redirect into
  // the OAuth2 flow that ends with the SDK writing a session, which is what this function is for.
  const signinPage = new ConsoleSigninPage(page, baseUrl);
  await signinPage.gotoHome();
  await signinPage.login(username, password);

  try {
    await waitForSessionData(page);
  } catch {
    // The @thunderid SDK has an unguarded race: ThunderIDProvider's and ProtectedRoute's own
    // sign-in effects can each exchange the same single-use authorization code, and the loser
    // gets a 400 invalid_grant with no session ever written - no amount of waiting recovers
    // from that. Re-visiting /console rides the IdP's SSO cookie (see
    // backend/internal/flow/session/transport.go) to a fresh code without re-entering credentials.
    console.log(
      "⚠️ Sign-in did not settle (SDK code-exchange race, or the welcome effect never committed), retrying..."
    );
    await signinPage.gotoHome();
    await waitForSessionData(page);
  }

  console.log("✅ Inline login successful!");

  // Save auth state for future tests
  await saveAuthState(page, baseUrl, authPath, debug);
}

/**
 * Delete a worker's auth file so the next test performs a fresh login instead of loading it.
 *
 * Called when the authenticatedPage fixture's post-test saveAuthState() write fails: the file on
 * disk at that point still holds whatever the previous successful save left there, which - if a
 * refresh happened during the test whose save just failed - can be an already-used refresh token.
 * Leaving that in place risks the exact same replay-triggered family revocation this fixture
 * exists to prevent; deleting it trades one extra login for that risk.
 */
export function invalidateAuthState(authPath: string): void {
  fs.rmSync(authPath, { force: true });
}

/**
 * Save authentication state to file.
 *
 * Also called by the authenticatedPage fixture after every test: the @thunderid SDK
 * transparently rotates the access/refresh token pair on a 401 or network error (see
 * httpRequest() in @thunderid/browser), but only in that test's own page. Since a worker reuses
 * one auth file across all its tests via a static addInitScript snapshot, a test that never
 * re-saves after such a rotation leaves the file holding an already-used refresh token - the
 * next test in that worker replays it, and the backend's reuse detection revokes the whole token
 * family (RFC 9700 §4.14.2), 401ing every request for the rest of that worker's run.
 */
export async function saveAuthState(page: Page, baseUrl: string, authPath: string, debug: boolean): Promise<void> {
  // Guard against persisting the wrong origin's storage. This is only meant to capture baseUrl's
  // session, but a test that ends on a different origin (a social-login redirect, an IdP sign-in
  // page, ...) would otherwise have this silently read and save THAT origin's storage under
  // baseUrl's label, corrupting the file for the next test in this worker. Parse failures are
  // treated the same as a mismatch: better to skip a save than risk writing the wrong thing.
  const currentOrigin = safeOrigin(page.url());
  const expectedOrigin = safeOrigin(baseUrl);
  if (currentOrigin !== expectedOrigin) {
    console.warn(
      `⚠️ Skipping auth state save: page is on ${currentOrigin ?? "an unparseable URL"}, expected ${expectedOrigin}`
    );
    return;
  }

  const context = page.context();
  const authDir = path.dirname(authPath);

  // Ensure directory exists
  if (!fs.existsSync(authDir)) {
    fs.mkdirSync(authDir, { recursive: true });
  }

  const cookies = await context.cookies();
  // context.storageState() would cover the cookies and localStorage, but never sessionStorage -
  // which is where this app's tokens live - so both are read here in one round trip instead.
  const { localStorage, sessionStorage } = await page.evaluate(() => {
    const entries = (storage: Storage) => Object.entries({ ...storage }).map(([name, value]) => ({ name, value }));
    return { localStorage: entries(window.localStorage), sessionStorage: entries(window.sessionStorage) };
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
