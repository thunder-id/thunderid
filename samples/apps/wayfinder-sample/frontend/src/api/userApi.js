// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

const THUNDER_BASE_URL =
  import.meta.env.VITE_THUNDER_BASE_URL || "https://localhost:8090";

async function thunderRequest(path, accessToken, options = {}) {
  const headers = {
    Accept: "application/json",
    ...(options.body ? { "Content-Type": "application/json" } : {}),
    ...(accessToken ? { Authorization: `Bearer ${accessToken}` } : {}),
    ...options.headers
  };
  const response = await fetch(`${THUNDER_BASE_URL}${path}`, { ...options, headers });
  const text = await response.text();
  const body = text ? safeJson(text) : null;
  if (!response.ok) {
    const message =
      (body && (body.description || body.message || body.error)) ||
      `Request failed (${response.status})`;
    const error = new Error(message);
    error.status = response.status;
    error.body = body;
    throw error;
  }
  return body;
}

function safeJson(text) {
  try {
    return JSON.parse(text);
  } catch {
    return null;
  }
}

export async function getMyUser(accessToken) {
  return thunderRequest(`/users/me`, accessToken);
}

export async function updateMyUser(accessToken, attributes) {
  return thunderRequest(`/users/me`, accessToken, {
    method: "PUT",
    body: JSON.stringify({ attributes })
  });
}

export async function updateMyCredentials(accessToken, attributes, currentPassword) {
  return thunderRequest(`/users/me/update-credentials`, accessToken, {
    method: "POST",
    body: JSON.stringify(currentPassword ? { currentPassword, attributes } : { attributes })
  });
}

// Issuer-initiated OpenID4VCI credential offer for the Wayfinder Sky Pass. The
// endpoint is public (the wallet authenticates during the flow), so no token is
// sent. Returns { credential_offer, credential_offer_uri } — render the
// credential_offer_uri as a QR for the wallet to scan.
export async function getSkyPassOffer() {
  return thunderRequest(
    `/openid4vci/offer?credential_configuration_id=wayfinder-skypass`
  );
}
