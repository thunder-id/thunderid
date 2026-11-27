// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

const THUNDERID_BASE_URL =
  import.meta.env.VITE_THUNDERID_BASE_URL || "https://localhost:8090";

async function thunderRequest(path, accessToken, options = {}) {
  const headers = {
    Accept: "application/json",
    ...(options.body ? { "Content-Type": "application/json" } : {}),
    ...(accessToken ? { Authorization: `Bearer ${accessToken}` } : {}),
    ...options.headers
  };
  const response = await fetch(`${THUNDERID_BASE_URL}${path}`, { ...options, headers });
  const text = await response.text();
  const body = text ? safeJson(text) : null;
  if (!response.ok) {
    // ThunderID error messages are objects with a defaultValue.
    const readable = (m) => (m && typeof m === "object" ? m.defaultValue : m);
    const message =
      (body && (readable(body.description) || readable(body.message) || body.error)) ||
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

// Changes one of the user's own credentials. A credential the user holds needs its current value; one the user
// does not hold yet is set without it.
export async function updateMyCredential(accessToken, name, currentValue, newValue) {
  return thunderRequest(`/users/me/update-credentials`, accessToken, {
    method: "POST",
    body: JSON.stringify({ name, currentValue: currentValue || undefined, newValue })
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
