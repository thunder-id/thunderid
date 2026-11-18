// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useCapabilitiesContext} from '@thunderid/contexts';
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
   * Whether the admin's permissions are still loading.
   */
  isLoading: boolean;
  /**
   * The pages this admin may see, in navigation order.
   */
  visible: ManagedResource[];
}

/**
 * Decides which pages the signed-in admin sees, from the server's capabilities endpoint.
 */
export default function useVisibleResources(): VisibleResources {
  const {can, isLoading} = useCapabilitiesContext();

  return {
    canRunLifecycle: can(LIFECYCLE_RESOURCE),
    isLoading,
    visible: MANAGED_RESOURCES.filter((resource: ManagedResource) => can(CATALOG_RESOURCES[resource])),
  };
}
