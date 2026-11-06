// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useRoutes} from '@thunderid/contexts';

/** Where the Deploy page and a gateway's explorer are, and the home page they exit to, as the console routes them. */
export interface DeployRoutePaths {
  home: {
    list: () => string;
  };
  deploy: {
    /** The Deploy page: the captured versions and every gateway. */
    list: () => string;
    /** The explorer of one gateway: what it runs and the values it holds. */
    explore: (gatewayId: string) => string;
    /** The variables and secrets one gateway holds. */
    values: (gatewayId: string) => string;
    /** One environment's own page: what it runs, its endpoints and its values, for a project when given. */
    environment: (gatewayId: string, projectId?: string) => string;
    /** A project's home, which a project's environment page returns to. */
    project?: (projectId: string) => string;
    /** Where a resource is edited in the console, for the kinds the console edits on a page of their own. */
    resource?: (resourceType: string, id: string) => string | undefined;
  };
  /** The console's administrators, and one administrator's page. */
  users?: {
    list: () => string;
    detail: (userId: string) => string;
  };
}

/**
 * Where the console edits each kind of resource a deploy's check names, by its resource type. A kind
 * edited on a list rather than a page of its own, such as a connection or a notification template, opens
 * that list; a kind the console does not edit has none.
 */
const RESOURCE_EDIT_PAGES: Record<string, (id: string) => string> = {
  application: (id) => `/applications/${id}`,
  agent: (id) => `/agents/${id}`,
  agent_type: (id) => `/agent-types/${id}`,
  flow: (id) => `/flows/${id}`,
  connection: () => '/connections',
  resource_server: (id) => `/resource-servers/${id}`,
  translation: (id) => `/translations/${id}`,
  theme: (id) => `/design/themes/${id}`,
  layout: (id) => `/design/layouts/${id}`,
  user: (id) => `/users/${id}`,
  group: (id) => `/groups/${id}`,
  role: (id) => `/roles/${id}`,
  user_type: (id) => `/user-types/${id}`,
  organization_unit: (id) => `/organization-units/${id}`,
  notification_template: () => '/notification-templates',
  server_config: () => '/settings',
  credential_configuration: (id) => `/verifiable-credentials/${id}`,
  presentation_definition: (id) => `/verifiable-presentations/${id}`,
};

/** The console's administrators, where the console does not route them. */
const DEFAULT_USER_ROUTES: NonNullable<DeployRoutePaths['users']> = {
  list: () => '/users',
  detail: (userId: string) => `/users/${encodeURIComponent(userId)}`,
};

/** Where the console's administrators are, as the console routes them or by default. */
export function userRoutesOf(routes: DeployRoutePaths): NonNullable<DeployRoutePaths['users']> {
  return routes.users ?? DEFAULT_USER_ROUTES;
}

export const defaultDeployRoutePaths: DeployRoutePaths = {
  home: {
    list: () => '/home',
  },
  deploy: {
    list: () => '/deploy',
    explore: (gatewayId: string) => `/deploy/gateways/${encodeURIComponent(gatewayId)}/explore`,
    values: (gatewayId: string) => `/deploy/gateways/${encodeURIComponent(gatewayId)}/values`,
    environment: (gatewayId: string, projectId?: string) =>
      `${projectId ? `/projects/${encodeURIComponent(projectId)}` : ''}/environments/${encodeURIComponent(gatewayId)}`,
    project: (projectId: string) => `/projects/${encodeURIComponent(projectId)}`,
    resource: (resourceType: string, id: string) => {
      const editOf = RESOURCE_EDIT_PAGES[resourceType];
      return editOf ? editOf(encodeURIComponent(id)) : undefined;
    },
  },
};

/** A project's home, as the console routes it or by default. */
export function projectHomeOf(routes: DeployRoutePaths, projectId: string): string {
  return (routes.deploy.project ?? defaultDeployRoutePaths.deploy.project ?? (() => '/'))(projectId);
}

export default function useDeployRoutes(): DeployRoutePaths {
  const routes = useRoutes<Partial<DeployRoutePaths>>();
  return {
    home: routes.home ?? defaultDeployRoutePaths.home,
    deploy: routes.deploy ?? defaultDeployRoutePaths.deploy,
    users: routes.users,
  };
}

/** Where a resource is edited, as the console routes it or by default; undefined for a kind it does not edit. */
export function resourceEditOf(routes: DeployRoutePaths, resourceType: string, id: string): string | undefined {
  return (routes.deploy.resource ?? defaultDeployRoutePaths.deploy.resource ?? (() => undefined))(resourceType, id);
}

/** The sections of an environment's own page, each at an address of its own under the environment's. */
export type EnvironmentSection = 'overview' | 'values' | 'endpoints' | 'history';

/** Where one section of an environment's page is. */
export function environmentSectionOf(
  routes: DeployRoutePaths,
  gatewayId: string,
  projectId: string | undefined,
  section: EnvironmentSection | 'explore',
): string {
  const base = routes.deploy.environment(gatewayId, projectId);
  return section === 'overview' ? base : `${base}/${section}`;
}
