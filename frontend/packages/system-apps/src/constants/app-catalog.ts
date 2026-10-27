// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {AppConfig, AppPageKey, AppType} from '../models/app-config';

/**
 * What the builder knows about each page of an app: the actions it can offer, the SDK components
 * it is built from, and, for pages only some users may see, the permission that gates it.
 *
 * @public
 */
export interface AppPageDefinition {
  actions: string[];
  components: string[];
  key: AppPageKey;
  /**
   * The permission a page shown `BY_PERMISSION` needs. A page without one is shown to every user
   * of the app.
   */
  permission?: string;
}

export const ADMIN_APP_PAGES: AppPageDefinition[] = [
  {
    actions: ['invite', 'create', 'edit', 'delete'],
    components: ['UserList', 'UserCreate', 'UserDetails'],
    key: 'users',
    permission: 'system:user:view',
  },
  {
    actions: ['create', 'members', 'delete'],
    components: ['GroupList', 'GroupCreate', 'GroupMembers'],
    key: 'groups',
    permission: 'system:group:view',
  },
  {
    actions: ['create', 'assign', 'delete'],
    components: ['RoleList', 'RoleCreate', 'RolePermissions', 'RoleAssignments'],
    key: 'roles',
    permission: 'system:role:view',
  },
  {
    actions: ['create', 'delete'],
    components: ['AgentList', 'AgentCreate', 'AgentDetails'],
    key: 'agents',
    permission: 'system:agent:view',
  },
];

export const ACCOUNT_APP_PAGES: AppPageDefinition[] = [
  {actions: ['edit'], components: ['UserProfile'], key: 'profile'},
  {actions: ['password'], components: ['ChangeCredential'], key: 'security'},
];

/**
 * The pages each app has, in navigation order.
 *
 * @public
 */
export const APP_PAGES: Record<AppType, AppPageDefinition[]> = {
  ACCOUNT: ACCOUNT_APP_PAGES,
  ADMIN: ADMIN_APP_PAGES,
};

const defaultAppConfig = (type: AppType): AppConfig => ({
  branding: {showSecuredBy: true},
  // The Account App reads like a personal account page, with its pages beside the content.
  navigation: {layout: type === 'ACCOUNT' ? 'SIDE_BAR' : 'TOP_BAR'},
  // Both apps start centered in a frame.
  content: {width: 'CONTAINED'},
  pages: APP_PAGES[type].map((page: AppPageDefinition) => ({
    actions: Object.fromEntries(page.actions.map((action: string) => [action, action !== 'delete'])),
    enabled: true,
    key: page.key,
    visibility: 'ALL',
  })),
  type,
});

/**
 * The configuration each app uses: every page on and shown to every user of the app, with every
 * action but deleting allowed.
 *
 * @public
 */
export const DEFAULT_APP_CONFIGS: Record<AppType, AppConfig> = {
  ACCOUNT: defaultAppConfig('ACCOUNT'),
  ADMIN: defaultAppConfig('ADMIN'),
};

export const DEFAULT_ADMIN_APP_CONFIG: AppConfig = DEFAULT_APP_CONFIGS.ADMIN;

export const DEFAULT_ACCOUNT_APP_CONFIG: AppConfig = DEFAULT_APP_CONFIGS.ACCOUNT;
