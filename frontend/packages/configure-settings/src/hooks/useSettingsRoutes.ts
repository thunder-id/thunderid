// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useRoutes} from '@thunderid/contexts';
import {useMemo} from 'react';

/**
 * Route paths this package needs from the host application: the flow pages the Account security
 * tab links to.
 *
 * The host supplies these via `@thunderid/contexts`'s `RoutesProvider`. When absent (e.g. this
 * package rendered standalone in a unit test), `useSettingsRoutes` falls back to
 * `defaultSettingsRoutePaths` below.
 *
 * @public
 */
export interface SettingsRoutePaths {
  flows: {
    list: () => string;
    detail: (flowId: string) => string;
  };
}

/**
 * Default settings paths, used when no host-supplied override is present.
 *
 * @public
 */
export const defaultSettingsRoutePaths: SettingsRoutePaths = {
  flows: {
    list: () => '/flows',
    detail: (flowId) => `/flows/${flowId}`,
  },
};

/**
 * Resolves the settings route paths, preferring the host application's configuration (supplied via
 * `RoutesProvider`) and falling back to this package's own defaults.
 *
 * @public
 */
export default function useSettingsRoutes(): SettingsRoutePaths {
  const {flows} = useRoutes<Partial<SettingsRoutePaths>>();
  return useMemo(
    () => ({
      flows: flows ?? defaultSettingsRoutePaths.flows,
    }),
    [flows],
  );
}
