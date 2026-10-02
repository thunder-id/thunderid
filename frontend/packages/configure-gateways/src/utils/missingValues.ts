// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {MissingValues} from '../models/gateway';

/**
 * The error code an apply or a revert fails with when the gateway lacks values the version needs.
 */
export const MISSING_VALUES_ERROR_CODE = 'GTW-1017';

/**
 * Whether a dry run found variables or secrets the gateway does not hold.
 */
export function hasMissingValues(missing?: MissingValues): missing is MissingValues {
  return (missing?.variables?.length ?? 0) + (missing?.secrets?.length ?? 0) > 0;
}

/**
 * Whether an apply or a revert was refused because the gateway lacks values the version needs.
 */
export function isMissingValuesError(error: Error | null | undefined): boolean {
  const response = (error as {response?: {data?: {code?: string}}} | null | undefined)?.response;
  return response?.data?.code === MISSING_VALUES_ERROR_CODE;
}
