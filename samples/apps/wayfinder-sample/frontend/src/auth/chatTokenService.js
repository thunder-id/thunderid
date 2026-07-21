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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { CHAT_SCOPES } from "./config";

const AUTH_SERVER_BASE_URL = import.meta.env.VITE_THUNDER_BASE_URL || "";
const CLIENT_ID = import.meta.env.VITE_THUNDER_CLIENT_ID || "WAYFINDER";
const AGENT_CHAT_URL = import.meta.env.VITE_AGENT_CHAT_URL || "http://localhost:8790/chat";
const CALLBACK_PATH = "/chat-token-callback";
const TOKEN_STORAGE_KEY = "wf_chat_token";
const FLOW_STORAGE_KEY = "wf_chat_token_flow";

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

async function sha256(value) {
  const encoded = new TextEncoder().encode(value);
  return new Uint8Array(await crypto.subtle.digest("SHA-256", encoded));
}

function randomString() {
  const bytes = new Uint8Array(32);
  crypto.getRandomValues(bytes);
  return base64UrlEncode(bytes);
}

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
  const codeChallenge = base64UrlEncode(await sha256(codeVerifier));
  const redirectUri = `${window.location.origin}${CALLBACK_PATH}`;

  sessionStorage.setItem(FLOW_STORAGE_KEY, JSON.stringify({ state, codeVerifier }));

  const params = new URLSearchParams({
    response_type: "code",
    client_id: CLIENT_ID,
    redirect_uri: redirectUri,
    scope: CHAT_SCOPES.join(" "),
    resource: AGENT_CHAT_URL,
    state,
    code_challenge: codeChallenge,
    code_challenge_method: "S256",
    acr_values: "urn:thunder:auth:user",
  });

  popup.location.href = `${AUTH_SERVER_BASE_URL}/oauth2/authorize?${params.toString()}`;

  return new Promise((resolve, reject) => {
    const timeout = window.setTimeout(() => {
      window.removeEventListener("message", handleMessage);
      reject(new Error("Chat authorization timed out."));
    }, 120000);

    async function handleMessage(event) {
      if (event.origin !== window.location.origin || event.data?.type !== "wayfinder-chat-token-oauth") {
        return;
      }

      window.clearTimeout(timeout);
      window.removeEventListener("message", handleMessage);

      try {
        if (event.data.error) {
          throw new Error(event.data.errorDescription || event.data.error);
        }

        const stored = JSON.parse(sessionStorage.getItem(FLOW_STORAGE_KEY) || "{}");
        sessionStorage.removeItem(FLOW_STORAGE_KEY);

        if (!event.data.code || event.data.state !== stored.state || !stored.codeVerifier) {
          throw new Error("Invalid chat authorization response.");
        }

        const token = await exchangeCodeForChatToken(event.data.code, stored.codeVerifier, redirectUri);
        sessionStorage.setItem(TOKEN_STORAGE_KEY, token);
        resolve(token);
      } catch (error) {
        reject(error);
      }
    }

    window.addEventListener("message", handleMessage);
  });
}

async function exchangeCodeForChatToken(code, codeVerifier, redirectUri) {
  const response = await fetch(`${AUTH_SERVER_BASE_URL}/oauth2/token`, {
    method: "POST",
    headers: { "Content-Type": "application/x-www-form-urlencoded" },
    body: new URLSearchParams({
      grant_type: "authorization_code",
      code,
      redirect_uri: redirectUri,
      client_id: CLIENT_ID,
      code_verifier: codeVerifier,
      resource: AGENT_CHAT_URL,
    }),
  });

  const text = await response.text();
  if (!response.ok) {
    throw new Error(`Chat token request failed (${response.status}): ${text}`);
  }

  const payload = JSON.parse(text);
  if (!payload.access_token) {
    throw new Error("Chat token response did not include an access token.");
  }

  return payload.access_token;
}
