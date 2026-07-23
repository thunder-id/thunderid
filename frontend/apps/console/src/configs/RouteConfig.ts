/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import type {AgentTypeRoutePaths} from '@thunderid/configure-agent-types';
import type {ConnectionRoutePaths} from '@thunderid/configure-connections';
import type {OrganizationUnitRoutePaths} from '@thunderid/configure-organization-units';
import type {ResourceServerRoutePaths} from '@thunderid/configure-resource-servers';
import type {TranslationRoutePaths} from '@thunderid/configure-translations';
import type {UserTypeRoutePaths} from '@thunderid/configure-user-types';
import type {UserRoutePaths} from '@thunderid/configure-users';

/**
 * Route paths for the domains Console implements itself, rather than through a
 * `@thunderid/configure-*` package. Unlike the package-published slices below, nothing outside
 * this app depends on this shape, but centralizing it here still buys the same thing: `App.tsx`'s
 * `<Route path>` declarations and every `navigate()`/`<Link>` call site across `features/*` are
 * built from the same source, so they can't drift apart.
 *
 * @public
 */
export interface ConsoleRoutePaths {
  home: {
    list: () => string;
  };
  applications: {
    list: () => string;
    detail: (id: string) => string;
    types: () => string;
    create: () => string;
  };
  agents: {
    list: () => string;
    detail: (id: string) => string;
    create: () => string;
  };
  groups: {
    list: () => string;
    detail: (id: string) => string;
    create: () => string;
  };
  roles: {
    list: () => string;
    detail: (id: string) => string;
    create: () => string;
  };
  verifiableCredentials: {
    list: () => string;
    detail: (id: string) => string;
    create: () => string;
  };
  verifiablePresentations: {
    list: () => string;
    detail: (id: string) => string;
    create: () => string;
  };
  flows: {
    list: () => string;
    create: () => string;
    byType: (type: string) => string;
    detail: (type: string, flowId: string) => string;
  };
  design: {
    list: () => string;
    themesCreate: () => string;
    themeDetail: (themeId: string) => string;
    layoutDetail: (layoutId: string) => string;
  };
  importExport: {
    list: () => string;
  };
  export: {
    page: () => string;
  };
  importConfiguration: {
    upload: () => string;
    validate: () => string;
    summary: () => string;
  };
  welcome: {
    root: () => string;
    createProject: () => string;
    getStarted: () => string;
    getStartedApplicationsTypes: () => string;
    getStartedApplicationsCreate: () => string;
    tryoutSecuringApplication: () => string;
    tryoutAiAgents: () => string;
    tryoutMcp: () => string;
    importConfigurationUpload: () => string;
    importConfigurationValidate: () => string;
    importConfigurationSummary: () => string;
  };
  settings: {
    list: () => string;
  };
}

/**
 * Route configuration for the whole Console app: every domain implemented by a reusable
 * `@thunderid/configure-*` package, composed from the route-shape each package publishes, plus
 * every domain Console implements itself (`ConsoleRoutePaths`).
 *
 * Each package reads its own slice through a package-level hook built on
 * `@thunderid/contexts`'s `useRoutes`, which lets a future host application override any of
 * those paths without touching the package. Console-local code has no such indirection to
 * worry about — it just imports `RouteConfig` (the value, below) directly.
 *
 * @public
 */
export type RouteConfig = OrganizationUnitRoutePaths &
  UserRoutePaths &
  UserTypeRoutePaths &
  AgentTypeRoutePaths &
  ConnectionRoutePaths &
  ResourceServerRoutePaths &
  TranslationRoutePaths &
  ConsoleRoutePaths;

/**
 * Static path segment for each domain, with no leading slash.
 *
 * `App.tsx`'s `<Route path>` declarations for these domains are built from these same segments
 * (see the "// Routes" comments there), so the mounted routes and the destinations packages
 * navigate to via `RouteConfig` below can never drift apart.
 *
 * @public
 */
export const ROUTE_SEGMENTS = {
  organizationUnits: 'organization-units',
  users: 'users',
  userTypes: 'user-types',
  agentTypes: 'agent-types',
  agents: 'agents',
  connections: 'connections',
  trustedIssuers: 'trusted-issuers',
  resourceServers: 'resource-servers',
  translations: 'translations',
  home: 'home',
  applications: 'applications',
  groups: 'groups',
  roles: 'roles',
  verifiableCredentials: 'verifiable-credentials',
  verifiablePresentations: 'verifiable-presentations',
  flows: 'flows',
  design: 'design',
  importExport: 'import-export',
  export: 'export',
  importConfiguration: 'import-configuration',
  welcome: 'welcome',
  settings: 'settings',
} as const;

const RouteConfig: RouteConfig = {
  organizationUnits: {
    list: () => `/${ROUTE_SEGMENTS.organizationUnits}`,
    detail: (id) => `/${ROUTE_SEGMENTS.organizationUnits}/${id}`,
    create: () => `/${ROUTE_SEGMENTS.organizationUnits}/create`,
  },
  users: {
    list: () => `/${ROUTE_SEGMENTS.users}`,
    detail: (userId) => `/${ROUTE_SEGMENTS.users}/${userId}`,
    add: () => `/${ROUTE_SEGMENTS.users}/add`,
    addCreate: () => `/${ROUTE_SEGMENTS.users}/add/create`,
    addInvite: () => `/${ROUTE_SEGMENTS.users}/add/invite`,
  },
  userTypes: {
    list: () => `/${ROUTE_SEGMENTS.userTypes}`,
    detail: (id) => `/${ROUTE_SEGMENTS.userTypes}/${id}`,
    create: () => `/${ROUTE_SEGMENTS.userTypes}/create`,
  },
  agentTypes: {
    detail: (id) => `/${ROUTE_SEGMENTS.agentTypes}/${id}`,
  },
  agents: {
    list: () => `/${ROUTE_SEGMENTS.agents}`,
    detail: (id) => `/${ROUTE_SEGMENTS.agents}/${id}`,
    create: () => `/${ROUTE_SEGMENTS.agents}/create`,
  },
  connections: {
    list: () => `/${ROUTE_SEGMENTS.connections}`,
    byType: (type) => `/${ROUTE_SEGMENTS.connections}/${type}`,
    detail: (type, id) => `/${ROUTE_SEGMENTS.connections}/${type}/${id}`,
    configure: (type) => `/${ROUTE_SEGMENTS.connections}/${type}/configure`,
    create: () => `/${ROUTE_SEGMENTS.connections}/create`,
  },
  trustedIssuers: {
    detail: (id) => `/${ROUTE_SEGMENTS.trustedIssuers}/${id}`,
  },
  resourceServers: {
    list: () => `/${ROUTE_SEGMENTS.resourceServers}`,
    detail: (resourceServerId) => `/${ROUTE_SEGMENTS.resourceServers}/${resourceServerId}`,
    create: () => `/${ROUTE_SEGMENTS.resourceServers}/create`,
  },
  translations: {
    list: () => `/${ROUTE_SEGMENTS.translations}`,
    detail: (language) => `/${ROUTE_SEGMENTS.translations}/${language}`,
    create: () => `/${ROUTE_SEGMENTS.translations}/create`,
  },
  home: {
    list: () => `/${ROUTE_SEGMENTS.home}`,
  },
  applications: {
    list: () => `/${ROUTE_SEGMENTS.applications}`,
    detail: (id) => `/${ROUTE_SEGMENTS.applications}/${id}`,
    types: () => `/${ROUTE_SEGMENTS.applications}/types`,
    create: () => `/${ROUTE_SEGMENTS.applications}/create`,
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
  verifiableCredentials: {
    list: () => `/${ROUTE_SEGMENTS.verifiableCredentials}`,
    detail: (id) => `/${ROUTE_SEGMENTS.verifiableCredentials}/${id}`,
    create: () => `/${ROUTE_SEGMENTS.verifiableCredentials}/create`,
  },
  verifiablePresentations: {
    list: () => `/${ROUTE_SEGMENTS.verifiablePresentations}`,
    detail: (id) => `/${ROUTE_SEGMENTS.verifiablePresentations}/${id}`,
    create: () => `/${ROUTE_SEGMENTS.verifiablePresentations}/create`,
  },
  flows: {
    list: () => `/${ROUTE_SEGMENTS.flows}`,
    create: () => `/${ROUTE_SEGMENTS.flows}/create`,
    byType: (type) => `/${ROUTE_SEGMENTS.flows}/${type}`,
    detail: (type, flowId) => `/${ROUTE_SEGMENTS.flows}/${type}/${flowId}`,
  },
  design: {
    list: () => `/${ROUTE_SEGMENTS.design}`,
    themesCreate: () => `/${ROUTE_SEGMENTS.design}/themes/create`,
    themeDetail: (themeId) => `/${ROUTE_SEGMENTS.design}/themes/${themeId}`,
    layoutDetail: (layoutId) => `/${ROUTE_SEGMENTS.design}/layouts/${layoutId}`,
  },
  importExport: {
    list: () => `/${ROUTE_SEGMENTS.importExport}`,
  },
  export: {
    page: () => `/${ROUTE_SEGMENTS.export}`,
  },
  importConfiguration: {
    upload: () => `/${ROUTE_SEGMENTS.importConfiguration}`,
    validate: () => `/${ROUTE_SEGMENTS.importConfiguration}/validate`,
    summary: () => `/${ROUTE_SEGMENTS.importConfiguration}/summary`,
  },
  welcome: {
    root: () => `/${ROUTE_SEGMENTS.welcome}`,
    createProject: () => `/${ROUTE_SEGMENTS.welcome}/create-project`,
    getStarted: () => `/${ROUTE_SEGMENTS.welcome}/get-started`,
    getStartedApplicationsTypes: () => `/${ROUTE_SEGMENTS.welcome}/get-started/applications/types`,
    getStartedApplicationsCreate: () => `/${ROUTE_SEGMENTS.welcome}/get-started/applications/create`,
    tryoutSecuringApplication: () => `/${ROUTE_SEGMENTS.welcome}/tryout/securing-application`,
    tryoutAiAgents: () => `/${ROUTE_SEGMENTS.welcome}/tryout/ai-agents`,
    tryoutMcp: () => `/${ROUTE_SEGMENTS.welcome}/tryout/mcp`,
    importConfigurationUpload: () => `/${ROUTE_SEGMENTS.welcome}/import-configuration`,
    importConfigurationValidate: () => `/${ROUTE_SEGMENTS.welcome}/import-configuration/validate`,
    importConfigurationSummary: () => `/${ROUTE_SEGMENTS.welcome}/import-configuration/summary`,
  },
  settings: {
    list: () => `/${ROUTE_SEGMENTS.settings}`,
  },
};

export default RouteConfig;
