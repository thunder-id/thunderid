// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {ManageAgentRoutePaths} from '@thunderid/manage-agents';
import type {ManageGroupRoutePaths} from '@thunderid/manage-groups';
import type {ManageRoleRoutePaths} from '@thunderid/manage-roles';
import type {ManageUserRoutePaths} from '@thunderid/manage-users';

/**
 * Route configuration for the Admin Console: the paths the `@thunderid/manage-*` pages link to,
 * composed from the route shape each package publishes. A package reads its own slice through
 * `@thunderid/contexts`'s `useRoutes`.
 *
 * @public
 */
export type RouteConfig = ManageUserRoutePaths & ManageGroupRoutePaths & ManageRoleRoutePaths & ManageAgentRoutePaths;

/**
 * The resources the Admin Console manages, in navigation order.
 */
export type ManagedResource = keyof RouteConfig;

export const MANAGED_RESOURCES: ManagedResource[] = ['users', 'agents', 'groups', 'roles'];

/**
 * Static path segments, with no leading slash. `App.tsx` mounts its routes from these and every link
 * is built from `RouteConfig`, so the two cannot drift apart.
 *
 * @public
 */
export const ROUTE_SEGMENTS = {
  agents: 'agents',
  create: 'new',
  groups: 'groups',
  roles: 'roles',
  users: 'users',
} as const;

const resourceRoutes = (segment: string): RouteConfig[ManagedResource] => ({
  create: (): string => `/${segment}/${ROUTE_SEGMENTS.create}`,
  details: (id: string): string => `/${segment}/${encodeURIComponent(id)}`,
  list: (): string => `/${segment}`,
});

const RouteConfig: RouteConfig = {
  agents: resourceRoutes(ROUTE_SEGMENTS.agents),
  groups: resourceRoutes(ROUTE_SEGMENTS.groups),
  roles: resourceRoutes(ROUTE_SEGMENTS.roles),
  users: resourceRoutes(ROUTE_SEGMENTS.users),
};

export default RouteConfig;
