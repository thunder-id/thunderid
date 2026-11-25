// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import { CHAT_SCOPES } from "./config";
import {
  authorizeUrl,
  codeChallengeFor,
  exchangeCode,
  randomString,
  waitForPopupResult,
} from "./popupAuthorize";

const AUTH_SERVER_BASE_URL = import.meta.env.VITE_THUNDER_BASE_URL || "";
const AGENT_CHAT_URL = import.meta.env.VITE_AGENT_CHAT_URL || "http://localhost:8790/chat";
const CALLBACK_PATH = "/chat-token-callback";
const TOKEN_STORAGE_KEY = "wf_chat_token";
const FLOW_STORAGE_KEY = "wf_chat_token_flow";

function decodeJWTPayload(token) {
  try {
    const b64 = token.split(".")[1].replace(/-/g, "+").replace(/_/g, "/");
    const padded = b64 + "=".repeat((4 - (b64.length % 4)) % 4);
    return JSON.parse(atob(padded));
  } catch {
    return null;
  }
}

function isTokenUsable(token) {
  const payload = token ? decodeJWTPayload(token) : null;
  if (!payload?.exp) {
    return Boolean(token);
  }

  return Date.now() / 1000 < payload.exp - 30;
}

export function getCachedChatAccessToken() {
  const token = sessionStorage.getItem(TOKEN_STORAGE_KEY);
  if (!isTokenUsable(token)) {
    sessionStorage.removeItem(TOKEN_STORAGE_KEY);
    return null;
  }

  return token;
}

export function clearChatAccessToken() {
  sessionStorage.removeItem(TOKEN_STORAGE_KEY);
  sessionStorage.removeItem(FLOW_STORAGE_KEY);
}

export async function getChatAccessToken({ interactive = false } = {}) {
  const cachedToken = getCachedChatAccessToken();
  if (cachedToken) {
    return cachedToken;
  }

  if (!interactive) {
    return null;
  }

  if (!AUTH_SERVER_BASE_URL || CHAT_SCOPES.length === 0) {
    throw new Error("Chat authorization is not configured.");
  }

  const popup = window.open("", "wayfinder-chat-token", "width=520,height=720");
  if (!popup) {
    throw new Error("Allow popups to authorize chat access.");
  }

  const state = randomString();
  const codeVerifier = randomString();
  const codeChallenge = await codeChallengeFor(codeVerifier);
  const redirectUri = `${window.location.origin}${CALLBACK_PATH}`;

  sessionStorage.setItem(FLOW_STORAGE_KEY, JSON.stringify({ state, codeVerifier }));

  popup.location.href = authorizeUrl({
    scope: CHAT_SCOPES.join(" "),
    resource: AGENT_CHAT_URL,
    redirectUri,
    state,
    codeChallenge,
  });

  const result = await waitForPopupResult({
    type: "wayfinder-chat-token-oauth",
    timeoutMs: 120000,
    timeoutMessage: "Chat authorization timed out.",
  });
  if (result.error) {
    throw new Error(result.errorDescription || result.error);
  }

  const stored = JSON.parse(sessionStorage.getItem(FLOW_STORAGE_KEY) || "{}");
  sessionStorage.removeItem(FLOW_STORAGE_KEY);

  if (!result.code || result.state !== stored.state || !stored.codeVerifier) {
    throw new Error("Invalid chat authorization response.");
  }

  const token = await exchangeCode({
    code: result.code,
    codeVerifier: stored.codeVerifier,
    redirectUri,
    resource: AGENT_CHAT_URL,
    label: "Chat token",
  });
  sessionStorage.setItem(TOKEN_STORAGE_KEY, token);
  return token;
}
