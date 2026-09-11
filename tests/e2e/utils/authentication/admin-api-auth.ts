// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type { APIRequestContext } from "@playwright/test";

// Application ID of the native app used for E2E admin authentication.
// Declared in tests/e2e/thunderid-config.yaml with a fixed UUID. It has no redirect-based grant,
// so it is not subject to the redirect-based flow initiation guard, but as a backend app it must
// present its Flow Secret to initiate a flow.
const E2E_ADMIN_NATIVE_APP_ID = "019e3a5c-0501-7f3e-a66e-66fc7918c3a7";

// Flow Secret declared for the E2E admin native app in tests/e2e/thunderid-config.yaml.
const E2E_ADMIN_NATIVE_FLOW_SECRET = "e2e-admin-native-app-secret";

// OAuth client credentials declared for the same app in tests/e2e/thunderid-config.yaml. The
// assertion names the app in its `aud` claim and the subject-token validator requires the
// exchanging client to be that app, so these belong to the app above, not a separate client.
const E2E_ADMIN_CLIENT_ID = "E2E_ADMIN";
const E2E_ADMIN_CLIENT_SECRET = "e2e-admin-client-secret";

// Resource server the admin token is bound to (RFC 8707). It must match the resource_server_identifier
// the flow evaluates permissions against, and it is required: without a resource the exchange resolves
// no target resource server and drops every permission scope, leaving a token that authenticates but
// authorizes nothing.
const E2E_ADMIN_RESOURCE = "https://localhost:8090/mcp";

/**
 * Obtain a short-lived admin access token.
 *
 * Runs the admin authentication flow to get an assertion, then exchanges that assertion for an
 * access token (RFC 8693). The exchange is not optional: the API gate accepts only RFC 9068 access
 * tokens, so the assertion itself is not a usable API credential.
 *
 * Reads SERVER_URL, ADMIN_USERNAME, and ADMIN_PASSWORD from environment variables.
 */
export async function getAdminToken(request: APIRequestContext): Promise<string> {
  const serverUrl = process.env.SERVER_URL || "https://localhost:8090";
  const adminUsername = process.env.ADMIN_USERNAME || "admin";
  const adminPassword = process.env.ADMIN_PASSWORD || "admin";
  const applicationId = E2E_ADMIN_NATIVE_APP_ID;

  const flowResponse = await request.post(`${serverUrl}/flow/execute`, {
    data: { applicationId, flowType: "AUTHENTICATION" },
    headers: { "Flow-Secret": E2E_ADMIN_NATIVE_FLOW_SECRET },
    ignoreHTTPSErrors: true,
  });
  if (!flowResponse.ok()) throw new Error(`Failed to start authentication flow: ${await flowResponse.text()}`);
  const flowData = await flowResponse.json();

  const authResponse = await request.post(`${serverUrl}/flow/execute`, {
    data: {
      executionId: flowData.executionId,
      ...(flowData.challengeToken && { challengeToken: flowData.challengeToken }),
      // resource_server_identifier scopes the permission evaluation to the System resource server
      // (identifier from backend/cmd/server/bootstrap/01-default-resources.yaml). Direct /flow/execute
      // calls do not pass through the OAuth layer, so the target resource server must be declared here.
      inputs: {
        username: adminUsername,
        password: adminPassword,
        requested_permissions: "system",
        resource_server_identifier: "https://localhost:8090/mcp",
      },
      action: "action_001",
    },
    ignoreHTTPSErrors: true,
  });
  if (!authResponse.ok()) throw new Error(`Admin authentication failed: ${await authResponse.text()}`);
  const { assertion } = await authResponse.json();

  const tokenResponse = await request.post(`${serverUrl}/oauth2/token`, {
    form: {
      grant_type: "urn:ietf:params:oauth:grant-type:token-exchange",
      subject_token: assertion,
      subject_token_type: "urn:ietf:params:oauth:token-type:jwt",
      resource: E2E_ADMIN_RESOURCE,
    },
    headers: {
      Authorization: `Basic ${Buffer.from(`${E2E_ADMIN_CLIENT_ID}:${E2E_ADMIN_CLIENT_SECRET}`).toString("base64")}`,
    },
    ignoreHTTPSErrors: true,
  });
  if (!tokenResponse.ok()) {
    throw new Error(`Admin token exchange failed: ${await tokenResponse.text()}`);
  }
  const { access_token: accessToken } = await tokenResponse.json();
  if (!accessToken) throw new Error("Admin token exchange returned no access_token");
  return accessToken;
}
