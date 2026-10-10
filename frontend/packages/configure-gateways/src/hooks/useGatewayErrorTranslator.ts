// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useCallback} from 'react';
import {useTranslation} from 'react-i18next';

/**
 * Returns a translate function for `getErrorMessage` and `QueryErrorNotice` that resolves bare
 * keys against the `gateways` namespace and forwards explicit `ns:` prefixes unchanged, so a
 * backend code maps to `gateways:errors.<CODE>` first and `common:errors.<CODE>` after.
 */
export default function useGatewayErrorTranslator(): (key: string, options?: Record<string, unknown>) => string {
  const {t} = useTranslation();

  return useCallback(
    (key: string, options?: Record<string, unknown>): string => t(key.includes(':') ? key : `gateways:${key}`, options),
    [t],
  );
}
