// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Authenticated Backend Request Helpers
 *
 * The plumbing every API helper needs: the backend base URL, an admin bearer token, the
 * self-signed-cert allowance, and one consistent error carrying the status and body.
 *
 * Exported as functions rather than a base class: the helpers are peers, not a hierarchy,
 * and the only shared state (the token) is deliberately process-wide.
 */

import type { APIRequestContext, APIResponse } from "@playwright/test";
import { getAdminToken } from "../authentication";

export const serverUrl = process.env.SERVER_URL || "https://localhost:8090";

type Method = "GET" | "POST" | "PUT" | "DELETE";

// One token per worker process, shared by every helper in it. getAdminToken costs two
// /flow/execute round trips and the assertion is valid for far longer than a suite run. A
// failed fetch clears the memo so one flaky start does not poison every later test in the worker.
let tokenPromise: Promise<string> | undefined;

function auth(request: APIRequestContext): Promise<string> {
  tokenPromise ??= getAdminToken(request).catch((error: unknown) => {
    tokenPromise = undefined;
    throw error;
  });
  return tokenPromise;
}

/** Authenticated call against the backend. Returns the response as-is, non-2xx included. */
export async function send(
  request: APIRequestContext,
  method: Method,
  path: string,
  data?: unknown,
): Promise<APIResponse> {
  return request.fetch(`${serverUrl}${path}`, {
    method,
    headers: { Authorization: `Bearer ${await auth(request)}` },
    ignoreHTTPSErrors: true,
    ...(data === undefined ? {} : { data }),
  });
}

/** Same as `send`, but a non-2xx becomes an error carrying the status and response body. */
export async function sendOk(
  request: APIRequestContext,
  method: Method,
  path: string,
  data?: unknown,
): Promise<APIResponse> {
  const response = await send(request, method, path, data);
  if (!response.ok()) {
    throw new Error(`${method} ${path} failed (${response.status()}): ${await response.text()}`);
  }
  return response;
}
