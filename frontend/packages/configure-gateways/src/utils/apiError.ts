// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/** The control plane's error code, such as `GTW-1030`, from a failed call, if the answer carried one. */
export function errorCodeOf(error: unknown): string | undefined {
  return (error as {response?: {data?: {code?: string}}} | undefined)?.response?.data?.code;
}

/** Why a call failed, as the answer says it, or the error's own message. */
export function errorReasonOf(error: unknown): string | undefined {
  const data = (error as {response?: {data?: unknown}} | undefined)?.response?.data;
  if (typeof data === 'string' && data.trim()) return data.trim();
  const body = (data ?? {}) as {message?: unknown; description?: unknown; error?: unknown; code?: unknown};
  for (const field of [body.description, body.message, body.error, body.code]) {
    if (typeof field === 'string' && field.trim()) return field.trim();
  }
  return error instanceof Error && error.message ? error.message : undefined;
}

/** The gateway section is set in the deployment configuration, so it cannot be changed at runtime. */
export const SETTINGS_DECLARATIVE = 'SCF-1003';

/** The apply would remove more resources than the deployment's threshold and was not confirmed. */
export const REMOVALS_NEED_CONFIRMATION = 'GTW-1030';

/** Whether a failed call was refused for want of a permission. */
export function isForbidden(error: unknown): boolean {
  return (error as {response?: {status?: number}} | undefined)?.response?.status === 403;
}
