// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Path segments of the Admin App. `App.tsx` mounts its routes from these and every link is built
 * from `RouteConfig`, so the two cannot drift apart.
 */
export const ROUTE_SEGMENTS = {
  agents: 'agents',
  create: 'new',
  groups: 'groups',
  roles: 'roles',
  users: 'users',
} as const;

/**
 * The resources the Admin App manages, in navigation order.
 */
export type ManagedResource = 'users' | 'groups' | 'roles' | 'agents';

export const MANAGED_RESOURCES: ManagedResource[] = ['users', 'groups', 'roles', 'agents'];

interface ResourceRoutes {
  create: () => string;
  details: (id: string) => string;
  list: () => string;
}

const resourceRoutes = (segment: string): ResourceRoutes => ({
  create: (): string => `/${segment}/${ROUTE_SEGMENTS.create}`,
  details: (id: string): string => `/${segment}/${encodeURIComponent(id)}`,
  list: (): string => `/${segment}`,
});

const RouteConfig: Record<ManagedResource, ResourceRoutes> = {
  agents: resourceRoutes(ROUTE_SEGMENTS.agents),
  groups: resourceRoutes(ROUTE_SEGMENTS.groups),
  roles: resourceRoutes(ROUTE_SEGMENTS.roles),
  users: resourceRoutes(ROUTE_SEGMENTS.users),
};

export default RouteConfig;
