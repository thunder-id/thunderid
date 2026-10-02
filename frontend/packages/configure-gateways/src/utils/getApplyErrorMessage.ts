// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {getErrorMessage} from '@thunderid/utils';

type TranslateFn = (key: string, options?: Record<string, unknown>) => string;

/**
 * Resolves the message for a failed apply or revert. A 502 that carries no code of its own means
 * the gateway did not answer, which is worth saying in those words rather than as a generic failure.
 */
export default function getApplyErrorMessage(error: Error, t: TranslateFn): string {
  const response = (error as {response?: {status?: number; data?: {code?: string}}}).response;

  if (response?.status === 502 && !response.data?.code) {
    return t('apply.unreachable', {
      defaultValue: 'The gateway could not be reached. Check that it is running and that its base URL is correct.',
    });
  }

  return getErrorMessage(error, t, 'apply.error', 'The configuration could not be applied. Please try again.');
}
