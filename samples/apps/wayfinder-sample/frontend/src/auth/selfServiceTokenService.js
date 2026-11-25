// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import { SELF_SERVICE_SCOPES, SYSTEM_RESOURCE } from "./config";
import {
  authorizeUrl,
  codeChallengeFor,
  exchangeCode,
  randomString,
  waitForPopupResult,
} from "./popupAuthorize";

const CALLBACK_PATH = "/self-service-callback";
export const SELF_SERVICE_MESSAGE_TYPE = "wayfinder-self-service-oauth";

// Gets a System resource server token with the self-service scopes through a popup. The gate reuses
// a recent sign-in; maxAge, from an insufficient_user_authentication challenge, asks for a newer one.
export async function getSelfServiceAccessToken({ maxAge } = {}) {
  // Open the popup before any await so the browser counts it as part of the click.
  const popup = window.open("", "wayfinder-self-service", "width=520,height=720");
  if (!popup) {
    throw new Error("Allow popups to confirm your sign-in.");
  }

  const state = randomString();
  const codeVerifier = randomString();
  const redirectUri = `${window.location.origin}${CALLBACK_PATH}`;
  popup.location.href = authorizeUrl({
    scope: SELF_SERVICE_SCOPES.join(" "),
    resource: SYSTEM_RESOURCE,
    redirectUri,
    state,
    codeChallenge: await codeChallengeFor(codeVerifier),
    maxAge,
  });

  const result = await waitForPopupResult({
    type: SELF_SERVICE_MESSAGE_TYPE,
    popup,
    closedMessage: "The sign-in window was closed.",
  });
  if (result.error) {
    throw new Error(result.errorDescription || result.error);
  }
  if (!result.code || result.state !== state) {
    throw new Error("Invalid sign-in response.");
  }

  return exchangeCode({ code: result.code, codeVerifier, redirectUri, resource: SYSTEM_RESOURCE });
}

// Returns the sub claim of a JWT access token, or undefined when it cannot be read.
export function tokenSubject(accessToken) {
  try {
    const payload = accessToken.split(".")[1].replace(/-/g, "+").replace(/_/g, "/");
    return JSON.parse(atob(payload)).sub;
  } catch {
    return undefined;
  }
}
