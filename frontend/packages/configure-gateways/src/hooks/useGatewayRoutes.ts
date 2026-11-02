// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useRoutes} from '@thunderid/contexts';

/**
 * Route paths this package needs from the host application.
 *
 * The host supplies these via `@thunderid/contexts`'s `RoutesProvider`. When absent (e.g. this
 * package rendered standalone in a unit test), `useGatewayRoutes` falls back to
 * `defaultGatewayRoutePaths` below.
 *
 * @public
 */
export interface GatewayRoutePaths {
  gateways: {
    list: () => string;
    detail: (id: string) => string;
  };
}

/**
 * Default gateway paths, used when no host-supplied override is present.
 *
 * @public
 */
export const defaultGatewayRoutePaths: GatewayRoutePaths = {
  gateways: {
    list: () => '/gateways',
    detail: (id) => `/gateways/${id}`,
  },
};

/**
 * Resolves the gateway route paths, preferring the host application's configuration and falling
 * back to this package's own defaults.
 *
 * @public
 */
export default function useGatewayRoutes(): GatewayRoutePaths['gateways'] {
  const routes = useRoutes<Partial<GatewayRoutePaths>>();
  return routes.gateways ?? defaultGatewayRoutePaths.gateways;
}
