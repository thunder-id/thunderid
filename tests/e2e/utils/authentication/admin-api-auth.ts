// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Application ID of the native app used for E2E admin authentication.
// Declared in tests/e2e/thunderid-config.yaml with a fixed UUID. Uses only the
// client_credentials grant type, so it is not subject to the redirect-based flow
// initiation guard, but as a backend app it must present its Flow Secret to initiate a flow.
const E2E_ADMIN_NATIVE_APP_ID = "019e3a5c-0501-7f3e-a66e-66fc7918c3a7";

// Flow Secret declared for the E2E admin native app in tests/e2e/thunderid-config.yaml.
const E2E_ADMIN_NATIVE_FLOW_SECRET = "e2e-admin-native-app-secret";

/**
 * Obtain a short-lived admin bearer token via the flow execution API.
 * Reads SERVER_URL, ADMIN_USERNAME, and ADMIN_PASSWORD from environment variables.
 */
export async function getAdminToken(request: import("@playwright/test").APIRequestContext): Promise<string> {
  const serverUrl = process.env.SERVER_URL || "https://localhost:8090";
  const adminUsername = process.env.ADMIN_USERNAME || "admin";
  const adminPassword = process.env.ADMIN_PASSWORD || "admin";
  const applicationId = E2E_ADMIN_NATIVE_APP_ID;

  // resource_server_identifier scopes the permission evaluation to the System resource server
  // (identifier from backend/cmd/server/bootstrap/01-default-resources.yaml). Direct /flow/execute
  // calls do not pass through the OAuth layer, so the target resource server must be declared here.
  // A flow takes these only from the request that starts it.
  const flowResponse = await request.post(`${serverUrl}/flow/execute`, {
    data: {
      applicationId,
      flowType: "AUTHENTICATION",
      inputs: {
        requested_permissions: "system",
        resource_server_identifier: "https://localhost:8090/mcp",
      },
    },
    headers: { "Flow-Secret": E2E_ADMIN_NATIVE_FLOW_SECRET },
    ignoreHTTPSErrors: true,
  });
  if (!flowResponse.ok()) throw new Error(`Failed to start authentication flow: ${await flowResponse.text()}`);
  const flowData = await flowResponse.json();

  const authResponse = await request.post(`${serverUrl}/flow/execute`, {
    data: {
      executionId: flowData.executionId,
      ...(flowData.challengeToken && { challengeToken: flowData.challengeToken }),
      inputs: {
        username: adminUsername,
        password: adminPassword,
      },
      action: "action_001",
    },
    ignoreHTTPSErrors: true,
  });
  if (!authResponse.ok()) throw new Error(`Admin authentication failed: ${await authResponse.text()}`);
  const { assertion } = await authResponse.json();
  return assertion;
}
