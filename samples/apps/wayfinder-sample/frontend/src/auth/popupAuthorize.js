// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Shared steps of an authorization code request with PKCE that runs in a popup.

const AUTH_SERVER_BASE_URL = import.meta.env.VITE_THUNDER_BASE_URL || "";
const CLIENT_ID = import.meta.env.VITE_THUNDER_CLIENT_ID || "WAYFINDER";

function base64UrlEncode(bytes) {
  let binary = "";
  bytes.forEach((byte) => {
    binary += String.fromCharCode(byte);
  });

  return btoa(binary)
    .replace(/=+$/g, "")
    .replace(/\+/g, "-")
    .replace(/\//g, "_");
}

export function randomString() {
  const bytes = new Uint8Array(32);
  crypto.getRandomValues(bytes);
  return base64UrlEncode(bytes);
}

export async function codeChallengeFor(codeVerifier) {
  const digest = await crypto.subtle.digest("SHA-256", new TextEncoder().encode(codeVerifier));
  return base64UrlEncode(new Uint8Array(digest));
}

export function authorizeUrl({ scope, resource, redirectUri, state, codeChallenge, maxAge }) {
  const params = new URLSearchParams({
    response_type: "code",
    client_id: CLIENT_ID,
    redirect_uri: redirectUri,
    scope,
    resource,
    state,
    code_challenge: codeChallenge,
    code_challenge_method: "S256",
    acr_values: "urn:thunder:auth:user",
  });
  if (maxAge !== undefined) {
    params.set("max_age", String(maxAge));
  }

  return `${AUTH_SERVER_BASE_URL}/oauth2/authorize?${params.toString()}`;
}

// Resolves with what the callback page posts back. Rejects after timeoutMs, or when popup is given and
// closes first.
export function waitForPopupResult({ type, timeoutMs, timeoutMessage, popup, closedMessage }) {
  return new Promise((resolve, reject) => {
    const timeout = timeoutMs
      ? window.setTimeout(() => {
          done();
          reject(new Error(timeoutMessage));
        }, timeoutMs)
      : undefined;
    const closedCheck = popup
      ? window.setInterval(() => {
          if (popup.closed) {
            done();
            reject(new Error(closedMessage));
          }
        }, 500)
      : undefined;

    function done() {
      window.clearTimeout(timeout);
      window.clearInterval(closedCheck);
      window.removeEventListener("message", handleMessage);
    }

    function handleMessage(event) {
      if (event.origin !== window.location.origin || event.data?.type !== type) {
        return;
      }

      done();
      resolve(event.data);
    }

    window.addEventListener("message", handleMessage);
  });
}

export async function exchangeCode({ code, codeVerifier, redirectUri, resource, label = "Token" }) {
  const response = await fetch(`${AUTH_SERVER_BASE_URL}/oauth2/token`, {
    method: "POST",
    headers: { "Content-Type": "application/x-www-form-urlencoded" },
    body: new URLSearchParams({
      grant_type: "authorization_code",
      code,
      redirect_uri: redirectUri,
      client_id: CLIENT_ID,
      code_verifier: codeVerifier,
      resource,
    }),
  });

  const text = await response.text();
  if (!response.ok) {
    throw new Error(`${label} request failed (${response.status}): ${text}`);
  }

  const payload = JSON.parse(text);
  if (!payload.access_token) {
    throw new Error(`${label} response did not include an access token.`);
  }

  return payload.access_token;
}
