// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Settings — CORS allowed origins (end-to-end)
 *
 * Verifies that configuring a custom allowed origin through the console changes the server's CORS
 * decision at runtime, without a server restart: an unconfigured origin is not echoed back, and once
 * added via the console the server echoes it in Access-Control-Allow-Origin.
 *
 * Each check sends a GET to a CORS-wrapped public endpoint (the OIDC discovery document) with an
 * explicit `Origin` header and asserts on the server's Access-Control-Allow-Origin response header.
 *
 * Required environment variables:
 *   - BASE_URL: ThunderID server URL (default https://localhost:8090).
 */

import { test, expect } from "../../fixtures/console";
import { TestTags } from "../../constants/test-tags";
import type { Page } from "@playwright/test";

const BASE_URL = process.env.BASE_URL || "https://localhost:8090";
const DISCOVERY_PATH = "/.well-known/openid-configuration";
// A fake origin dedicated to this test - it only needs to be a valid origin string
const TEST_ORIGIN = "https://e2e-cors-probe.invalid";
// The same origin expressed as an anchored pattern, for the regex entry type.
const TEST_ORIGIN_PATTERN = "^https://e2e-cors-probe\\.invalid$";
// The sample app's origin, configured by the imported deployment config. Editing allowed origins
// through the console must never drop it, otherwise every later sample-app test loses its ability to
// read cross-origin responses.
const SAMPLE_APP_ORIGIN = process.env.SAMPLE_APP_URL || "https://localhost:3000";

/**
 * Sends a GET to the CORS-wrapped discovery endpoint with the given `Origin` header and returns the
 * server's Access-Control-Allow-Origin response header — `undefined` when the origin is not allowed.
 */
async function corsAllowOriginHeader(page: Page, origin: string = TEST_ORIGIN): Promise<string | undefined> {
  // Use the request client to avoid page navigation, CSP, or on-load redirects.
  const response = await page.request.get(`${BASE_URL}${DISCOVERY_PATH}`, {
    headers: { Origin: origin },
    failOnStatusCode: false,
  });
  return response.headers()["access-control-allow-origin"];
}

test.describe("Settings — CORS allowed origins", { tag: [TestTags.SMOKE] }, () => {
  test.beforeEach(async ({ settingsPage }) => {
    // Ensure a clean starting state (neither entry yet configured).
    await settingsPage.goto();
    await settingsPage.removeAllowedOrigin(TEST_ORIGIN);
    await settingsPage.removeAllowedOrigin(TEST_ORIGIN_PATTERN);
  });

  test.afterEach(async ({ settingsPage }) => {
    // Remove the entries added by the test so the shared deployment config stays clean.
    await settingsPage.goto();
    await settingsPage.removeAllowedOrigin(TEST_ORIGIN);
    await settingsPage.removeAllowedOrigin(TEST_ORIGIN_PATTERN);

    // Editing origins here must leave the pre-configured ones intact and still enforced at runtime.
    // Without this, a regression that drops them would surface far away, as unexplained cross-origin
    // failures in whichever suite happens to run next.
    const acao = await corsAllowOriginHeader(settingsPage.page, SAMPLE_APP_ORIGIN);
    expect(acao, `editing allowed origins must not stop ${SAMPLE_APP_ORIGIN} from being allowed`).toBe(
      SAMPLE_APP_ORIGIN
    );
  });

  test("denies a cross-origin request from an origin that is not configured", async ({ settingsPage }) => {
    const acao = await corsAllowOriginHeader(settingsPage.page);
    expect(acao, "an unconfigured origin must not be echoed in Access-Control-Allow-Origin").toBeUndefined();
  });

  test("persists a new allowed origin added through the console", async ({ settingsPage }) => {
    await settingsPage.addAllowedOrigin(TEST_ORIGIN);
    expect(await settingsPage.hasCustomOrigin(TEST_ORIGIN)).toBe(true);
  });

  test("allows a cross-origin request once the origin is configured (no server restart)", async ({ settingsPage }) => {
    await settingsPage.addAllowedOrigin(TEST_ORIGIN);

    const acao = await corsAllowOriginHeader(settingsPage.page);
    expect(acao, "a configured origin must be echoed in Access-Control-Allow-Origin").toBe(TEST_ORIGIN);
  });

  test("allows a cross-origin request matched by an entry saved as a regex", async ({ settingsPage }) => {
    await settingsPage.addAllowedOriginRegex(TEST_ORIGIN_PATTERN);

    expect(await settingsPage.hasCustomOrigin(TEST_ORIGIN_PATTERN)).toBe(true);
    const acao = await corsAllowOriginHeader(settingsPage.page);
    expect(acao, "an origin matched by a configured pattern must be echoed in Access-Control-Allow-Origin").toBe(
      TEST_ORIGIN
    );
  });

  test("denies the origin again after it is removed through the console", async ({ settingsPage }) => {
    await settingsPage.addAllowedOrigin(TEST_ORIGIN);
    await settingsPage.removeAllowedOrigin(TEST_ORIGIN);

    expect(await settingsPage.hasCustomOrigin(TEST_ORIGIN)).toBe(false);
    const acao = await corsAllowOriginHeader(settingsPage.page);
    expect(acao, "a removed origin must no longer be echoed in Access-Control-Allow-Origin").toBeUndefined();
  });
});
