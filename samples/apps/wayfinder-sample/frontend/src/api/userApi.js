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

export async function updateMyCredentials(accessToken, attributes) {
  return thunderRequest(`/users/me/update-credentials`, accessToken, {
    method: "POST",
    body: JSON.stringify({ attributes })
  });
}
