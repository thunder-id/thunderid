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

import { SCOPES } from "./config.js";

const AUTH_SERVER_BASE_URL = import.meta.env.VITE_THUNDER_BASE_URL || "";
const CLIENT_ID = import.meta.env.VITE_THUNDER_CLIENT_ID || "WAYFINDER";
const APP_ID = import.meta.env.VITE_THUNDER_APP_ID || CLIENT_ID;
const FETCH_TIMEOUT_MS = 15000;

async function fetchWithTimeout(url, options = {}) {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), FETCH_TIMEOUT_MS);
  try {
    return await fetch(url, { ...options, signal: controller.signal });
  } finally {
    clearTimeout(timer);
  }
}

export async function getAppMetadata() {
  const res = await fetchWithTimeout(
    `${AUTH_SERVER_BASE_URL}/flow/meta?type=APP&id=${encodeURIComponent(APP_ID)}`
  );
  if (!res.ok) return null;
  return (await res.json())?.application || null;
}

export async function initiateFlow(flowType) {
  const body = { applicationId: APP_ID, flowType };
  if (flowType === "AUTHENTICATION" || flowType === "REGISTRATION") {
    body.inputs = { requested_permissions: SCOPES.join(" ") };
  }
  const res = await fetchWithTimeout(`${AUTH_SERVER_BASE_URL}/flow/execute`, {
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

  const res = await fetchWithTimeout(`${AUTH_SERVER_BASE_URL}/flow/execute`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
  if (!res.ok) throw new Error(`Flow step failed: ${res.status}`);
  return res.json();
}

export async function exchangeAssertion(assertion) {
  const res = await fetchWithTimeout(`${AUTH_SERVER_BASE_URL}/oauth2/token`, {
    method: "POST",
    headers: { "Content-Type": "application/x-www-form-urlencoded" },
    body: new URLSearchParams({
      grant_type: "urn:ietf:params:oauth:grant-type:token-exchange",
      subject_token: assertion,
      subject_token_type: "urn:ietf:params:oauth:token-type:jwt",
      client_id: CLIENT_ID,
    }),
  });
  if (!res.ok) throw new Error(`Token exchange failed (${res.status})`);
  return res.json();
}
