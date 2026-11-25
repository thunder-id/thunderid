// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {AgentRoutePaths} from '@thunderid/configure-agents';
import type {GroupRoutePaths} from '@thunderid/configure-groups';
import type {RoleRoutePaths} from '@thunderid/configure-roles';
import type {UserRoutePaths} from '@thunderid/configure-users';

/**
 * Route configuration for the Admin Console app: the domains it exposes via `@thunderid/configure-*`
 * packages, composed from the route-shape each package publishes.
 *
 * Each package reads its own slice through a package-level hook built on
 * `@thunderid/contexts`'s `useRoutes`, which lets a host application override any of those paths
 * without touching the package.
 *
 * @public
 */
export type RouteConfig = UserRoutePaths & AgentRoutePaths & GroupRoutePaths & RoleRoutePaths;

/**
 * The resources the Admin Console manages, in navigation order.
 */
export type ManagedResource = 'agents' | 'groups' | 'roles' | 'users';

export const MANAGED_RESOURCES: ManagedResource[] = ['users', 'agents', 'groups', 'roles'];

/**
 * Static path segment for each domain, with no leading slash.
 *
 * `App.tsx`'s `<Route path>` declarations are built from these same segments, so the mounted
 * routes and the destinations packages navigate to via `RouteConfig` below can never drift apart.
 *
 * @public
 */
export const ROUTE_SEGMENTS = {
  users: 'users',
  agents: 'agents',
  groups: 'groups',
  roles: 'roles',
} as const;

const RouteConfig: RouteConfig = {
  users: {
    list: () => `/${ROUTE_SEGMENTS.users}`,
    detail: (userId) => `/${ROUTE_SEGMENTS.users}/${userId}`,
    add: () => `/${ROUTE_SEGMENTS.users}/add`,
  },
  agents: {
    list: () => `/${ROUTE_SEGMENTS.agents}`,
    detail: (id) => `/${ROUTE_SEGMENTS.agents}/${id}`,
    create: () => `/${ROUTE_SEGMENTS.agents}/create`,
  },
  groups: {
    list: () => `/${ROUTE_SEGMENTS.groups}`,
    detail: (id) => `/${ROUTE_SEGMENTS.groups}/${id}`,
    create: () => `/${ROUTE_SEGMENTS.groups}/create`,
  },
  roles: {
    list: () => `/${ROUTE_SEGMENTS.roles}`,
    detail: (id) => `/${ROUTE_SEGMENTS.roles}/${id}`,
    create: () => `/${ROUTE_SEGMENTS.roles}/create`,
  },
};

export default RouteConfig;
