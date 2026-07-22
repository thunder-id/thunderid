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

/**
 * Sends a GET to the CORS-wrapped discovery endpoint with `TEST_ORIGIN` as the `Origin` header and
 * returns the server's Access-Control-Allow-Origin response header — `undefined` when the origin is
 * not allowed.
 */
async function corsAllowOriginHeader(page: Page): Promise<string | undefined> {
  // Use the request client to avoid page navigation, CSP, or on-load redirects.
  const response = await page.request.get(`${BASE_URL}${DISCOVERY_PATH}`, {
    headers: { Origin: TEST_ORIGIN },
    failOnStatusCode: false,
  });
  return response.headers()["access-control-allow-origin"];
}

test.describe("Settings — CORS allowed origins", { tag: [TestTags.SMOKE] }, () => {
  test.beforeEach(async ({ settingsPage }) => {
    // Ensure a clean starting state (origin not yet configured).
    await settingsPage.goto();
    await settingsPage.removeAllowedOrigin(TEST_ORIGIN);
  });

  test.afterEach(async ({ settingsPage }) => {
    // Remove the origin added by the test so the shared deployment config stays clean.
    await settingsPage.goto();
    await settingsPage.removeAllowedOrigin(TEST_ORIGIN);
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

  test("denies the origin again after it is removed through the console", async ({ settingsPage }) => {
    await settingsPage.addAllowedOrigin(TEST_ORIGIN);
    await settingsPage.removeAllowedOrigin(TEST_ORIGIN);

    expect(await settingsPage.hasCustomOrigin(TEST_ORIGIN)).toBe(false);
    const acao = await corsAllowOriginHeader(settingsPage.page);
    expect(acao, "a removed origin must no longer be echoed in Access-Control-Allow-Origin").toBeUndefined();
  });
});
