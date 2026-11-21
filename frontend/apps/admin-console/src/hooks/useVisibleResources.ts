// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useCapabilitiesContext} from '@thunderid/contexts';
import type {AppPageConfig} from '@thunderid/system-apps';
import useAdminAppConfig from './useAdminAppConfig';
import {MANAGED_RESOURCES, type ManagedResource} from '../configs/RouteConfig';

/**
 * The catalog resource each page shows: an admin who may not view it never sees the page. The server
 * still decides every request.
 */
const CATALOG_RESOURCES: Record<ManagedResource, string> = {
  agents: 'agent',
  groups: 'group',
  roles: 'role',
  users: 'user',
};

/**
 * Reading the configured administration flows needs the server configuration, so lifecycle actions are
 * offered only to admins who may view it.
 */
const LIFECYCLE_RESOURCE = 'serverconfig';

export interface VisibleResources {
  /**
   * Whether the admin may suspend, unsuspend and unlock users and agents.
   */
  canRunLifecycle: boolean;
  /**
   * Whether the admin's permissions or the Admin App configuration are still loading.
   */
  isLoading: boolean;
  /**
   * The pages this admin may see, in the Admin App's navigation order.
   */
  visible: ManagedResource[];
}

const isManaged = (page: AppPageConfig): boolean => (MANAGED_RESOURCES as string[]).includes(page.key);

/**
 * Decides which pages the signed-in admin sees: the pages the Admin App turns on, in its order, that
 * the server's capabilities endpoint lets the admin view. A page the admin may not view stays hidden
 * whatever the configuration's visibility says, since every request on it would be refused.
 */
export default function useVisibleResources(): VisibleResources {
  const {can, isLoading: isCapabilitiesLoading} = useCapabilitiesContext();
  const {config, isLoading: isConfigLoading} = useAdminAppConfig();

  return {
    canRunLifecycle: can(LIFECYCLE_RESOURCE),
    isLoading: isCapabilitiesLoading || isConfigLoading,
    visible: config.pages
      .filter(isManaged)
      .filter((page: AppPageConfig) => page.enabled && can(CATALOG_RESOURCES[page.key as ManagedResource]))
      .map((page: AppPageConfig) => page.key as ManagedResource),
  };
}
