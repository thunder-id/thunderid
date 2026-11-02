// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/** The control plane's error code, such as `GTW-1030`, from a failed call, if the answer carried one. */
export function errorCodeOf(error: unknown): string | undefined {
  return (error as {response?: {data?: {code?: string}}} | undefined)?.response?.data?.code;
}

/** The apply would remove more resources than the deployment's threshold and was not confirmed. */
export const REMOVALS_NEED_CONFIRMATION = 'GTW-1030';

/** Whether a failed call was refused for want of a permission. */
export function isForbidden(error: unknown): boolean {
  return (error as {response?: {status?: number}} | undefined)?.response?.status === 403;
}
