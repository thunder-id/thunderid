// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import { SCOPES } from "./config.js";

const AUTH_SERVER_BASE_URL = import.meta.env.VITE_THUNDERID_BASE_URL || "";
const CLIENT_ID = import.meta.env.VITE_THUNDERID_CLIENT_ID || "WAYFINDER";
const APP_ID = import.meta.env.VITE_THUNDERID_APP_ID || CLIENT_ID;
const FETCH_TIMEOUT_MS = 15000;

async function fetchWithTimeout(url, options = {}, handleResponse) {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), FETCH_TIMEOUT_MS);
  try {
    const response = await fetch(url, {
      ...options,
      signal: controller.signal,
    });
    return await handleResponse(response);
  } finally {
    clearTimeout(timer);
  }
}

async function throwResponseError(response, operation) {
  let body = "";
  try {
    body = (await response.text()).trim();
  } catch {
    // Preserve the HTTP status if the error body cannot be read.
  }
  const details = body ? `: ${body}` : "";
  throw new Error(`${operation} (${response.status})${details}`);
}

export async function getAppMetadata() {
  return fetchWithTimeout(
    `${AUTH_SERVER_BASE_URL}/flow/meta?type=APP&id=${encodeURIComponent(APP_ID)}`,
    {},
    async (res) => {
      if (!res.ok) return null;
      return (await res.json())?.application || null;
    }
  );
}

export async function initiateFlow(flowType) {
  const body = { applicationId: APP_ID, flowType };
  if (flowType === "AUTHENTICATION" || flowType === "REGISTRATION") {
    body.inputs = { requested_permissions: SCOPES.join(" ") };
  }
  return fetchWithTimeout(`${AUTH_SERVER_BASE_URL}/flow/execute`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
    // The SSO session handle rides a cookie the server sets during sign-in. Credentials must be
    // included for the browser to store it here and send it back on later flow calls.
    credentials: "include",
  }, async (res) => {
    if (!res.ok) await throwResponseError(res, "Flow initiation failed");
    return res.json();
  });
}

export async function submitFlowStep({ executionId, action, inputs, challengeToken }) {
  const payload = { executionId };
  if (action) payload.action = action;
  if (inputs && Object.keys(inputs).length > 0) payload.inputs = inputs;
  if (challengeToken) payload.challengeToken = challengeToken;

  return fetchWithTimeout(`${AUTH_SERVER_BASE_URL}/flow/execute`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
    credentials: "include",
  }, async (res) => {
    if (!res.ok) await throwResponseError(res, "Flow step failed");
    return res.json();
  });
}

/**
 * Ends the SSO session by running the application's sign-out flow natively, the same way
 * authentication is driven. The default sign-out flow asks the user to confirm, which is submitted
 * straight away since choosing sign out in the app is itself the confirmation.
 *
 * Throws unless the flow reports COMPLETE, so a caller is never told the session ended when it may
 * still be alive server-side.
 */
export async function signOutNatively() {
  let step = await initiateFlow("SIGNOUT");

  for (let i = 0; i < 5 && step?.flowStatus !== "COMPLETE"; i++) {
    const actions = step?.data?.actions ?? [];
    if (actions.length === 0 || !step?.executionId) break;

    // A step offering a choice cannot be answered without knowing what each option means, so only
    // an unambiguous single action is submitted rather than guessing by list position.
    if (actions.length > 1) {
      throw new Error(
        `Sign-out flow returned ${actions.length} actions (${actions.map((a) => a.ref).join(", ")}); ` +
          "this sample only drives a single-action confirmation."
      );
    }

    step = await submitFlowStep({ executionId: step.executionId, action: actions[0].ref });
  }

  if (step?.flowStatus !== "COMPLETE") {
    throw new Error(`Sign-out flow did not complete (flowStatus: ${step?.flowStatus ?? "unknown"}).`);
  }
}

export async function exchangeAssertion(assertion) {
  return fetchWithTimeout(`${AUTH_SERVER_BASE_URL}/oauth2/token`, {
    method: "POST",
    headers: { "Content-Type": "application/x-www-form-urlencoded" },
    body: new URLSearchParams({
      grant_type: "urn:ietf:params:oauth:grant-type:token-exchange",
      subject_token: assertion,
      subject_token_type: "urn:ietf:params:oauth:token-type:jwt",
      client_id: CLIENT_ID,
    }),
  }, async (res) => {
    if (!res.ok) await throwResponseError(res, "Token exchange failed");
    return res.json();
  });
}
