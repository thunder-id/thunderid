// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/** Whether a failed call was refused for want of a permission. */
export default function isForbidden(error: unknown): boolean {
  return (error as {response?: {status?: number}} | undefined)?.response?.status === 403;
}
