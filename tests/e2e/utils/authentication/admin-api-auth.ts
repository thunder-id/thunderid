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
      inputs: { username: adminUsername, password: adminPassword, requested_permissions: "system" },
      action: "action_001",
    },
    ignoreHTTPSErrors: true,
  });
  if (!authResponse.ok()) throw new Error(`Admin authentication failed: ${await authResponse.text()}`);
  const { assertion } = await authResponse.json();
  return assertion;
}
