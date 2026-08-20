// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useConfig} from '@thunderid/contexts';

/**
 * Resolves the environment manager's base URL.
 *
 * The Control Plane serves the environment manager in process, on its own origin, so there is
 * nothing to configure: the management API's base URL is the environment manager's too.
 *
 * Promotion is a Control Plane feature. Every other plane resolves to undefined and the feature
 * reports that it is unavailable, rather than calling a host that does not serve it.
 */
export default function useEnvManagerUrl(): string | undefined {
  const {config, getServerUrl} = useConfig();
  if (config.plane !== 'cp') {
    return undefined;
  }
  const url: string = getServerUrl();

  return url ? url.replace(/\/+$/, '') : undefined;
}
