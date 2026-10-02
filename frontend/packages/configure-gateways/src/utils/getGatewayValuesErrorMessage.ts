// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {getErrorMessage} from '@thunderid/utils';

type TranslateFn = (key: string, options?: Record<string, unknown>) => string;

/**
 * Whether a request to a gateway's variables or secrets failed because the gateway did not answer.
 * The control plane passes these requests through, and answers 502 when the gateway cannot be reached.
 */
export function isGatewayUnreachable(error: Error | null | undefined): boolean {
  return (error as {response?: {status?: number}} | null | undefined)?.response?.status === 502;
}

/**
 * Resolves the message for a failed read or write of a gateway's variables or secrets. The
 * gateway's own store answers these, so its error codes resolve through the catalog like any other.
 */
export default function getGatewayValuesErrorMessage(
  error: Error,
  t: TranslateFn,
  fallbackKey: string,
  fallbackDefaultValue?: string,
): string {
  if (isGatewayUnreachable(error)) {
    return t('values.unreachable', {
      defaultValue: 'The gateway could not be reached. Check that it is running and that its base URL is correct.',
    });
  }

  return getErrorMessage(error, t, fallbackKey, fallbackDefaultValue);
}
