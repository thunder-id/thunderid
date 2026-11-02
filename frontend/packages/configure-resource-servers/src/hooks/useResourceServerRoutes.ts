// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useRoutes} from '@thunderid/contexts';

/**
 * Route paths this package needs from the host application.
 *
 * The host supplies these via `@thunderid/contexts`'s `RoutesProvider`. When absent (e.g. this
 * package rendered standalone in Storybook or a unit test), `useResourceServerRoutes` falls back
 * to `defaultResourceServerRoutePaths` below.
 *
 * @public
 */
export interface ResourceServerRoutePaths {
  resourceServers: {
    list: () => string;
    detail: (id: string) => string;
    create: () => string;
  };
  connections: {
    create: () => string;
  };
}

/**
 * Default resource server paths, used when no host-supplied override is present.
 *
 * @public
 */
export const defaultResourceServerRoutePaths: ResourceServerRoutePaths = {
  resourceServers: {
    list: () => '/resource-servers',
    detail: (id) => `/resource-servers/${id}`,
    create: () => '/resource-servers/create',
  },
  connections: {
    create: () => '/connections/create',
  },
};

export function useResourceServerConnectionRoutes(): ResourceServerRoutePaths['connections'] {
  const routes = useRoutes<Partial<ResourceServerRoutePaths>>();
  return routes.connections ?? defaultResourceServerRoutePaths.connections;
}

/**
 * Resolves the resource server route paths, preferring the host application's configuration
 * (supplied via `RoutesProvider`) and falling back to this package's own defaults.
 *
 * Components should never hardcode resource server destination paths; they should call this
 * hook and build the destination from the returned functions instead.
 *
 * @public
 */
export default function useResourceServerRoutes(): ResourceServerRoutePaths['resourceServers'] {
  const routes = useRoutes<Partial<ResourceServerRoutePaths>>();
  return routes.resourceServers ?? defaultResourceServerRoutePaths.resourceServers;
}
