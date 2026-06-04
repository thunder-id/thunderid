/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com). All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { SCOPES } from "./config.js";

const AUTH_SERVER_BASE_URL = import.meta.env.VITE_THUNDER_BASE_URL || "";
const CLIENT_ID = import.meta.env.VITE_THUNDER_CLIENT_ID || "WAYFINDER";
const APP_ID = import.meta.env.VITE_THUNDER_APP_ID || CLIENT_ID;

export async function getAppMetadata() {
  const res = await fetch(
    `${AUTH_SERVER_BASE_URL}/flow/meta?type=APP&id=${encodeURIComponent(APP_ID)}`,
    { method: "GET" }
  );
  if (!res.ok) return null;
  const body = await res.json();
  return body?.application || null;
}

export async function initiateFlow(flowType) {
  const body = { applicationId: APP_ID, flowType };

  // Declare the permissions we need so the AuthorizationExecutor intersects
  // them against the user's actual role assignments and embeds the result in
  // the assertion. Without this, authorized_permissions is empty and token
  // exchange produces an access token with no scope, failing booking API checks.
  if (flowType === "AUTHENTICATION" || flowType === "REGISTRATION") {
    body.inputs = { requested_permissions: SCOPES.join(" ") };
  }

  const res = await fetch(`${AUTH_SERVER_BASE_URL}/flow/execute`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  if (!res.ok) throw new Error(`Flow initiation failed: ${res.status}`);
  return res.json();
}

export async function submitFlowStep({ executionId, action, inputs, challengeToken }) {
  const payload = { executionId };
  if (action) payload.action = action;
  if (inputs && Object.keys(inputs).length > 0) payload.inputs = inputs;
  if (challengeToken) payload.challengeToken = challengeToken;

  const res = await fetch(`${AUTH_SERVER_BASE_URL}/flow/execute`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
  if (!res.ok) throw new Error(`Flow step failed: ${res.status}`);
  return res.json();
}

// Exchanges the auth assertion from the embedded flow for a proper OAuth2 access token.
// The assertion has authorized_permissions (not scope), so it cannot be used directly
// for resource API calls. Token exchange produces an at+jwt with the correct scope claim.
export async function exchangeAssertion(assertion) {
  const res = await fetch(`${AUTH_SERVER_BASE_URL}/oauth2/token`, {
    method: "POST",
    headers: { "Content-Type": "application/x-www-form-urlencoded" },
    body: new URLSearchParams({
      grant_type: "urn:ietf:params:oauth:grant-type:token-exchange",
      subject_token: assertion,
      subject_token_type: "urn:ietf:params:oauth:token-type:jwt",
      client_id: CLIENT_ID,
    }),
  });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(`Token exchange failed (${res.status}): ${text}`);
  }
  return res.json();
}
