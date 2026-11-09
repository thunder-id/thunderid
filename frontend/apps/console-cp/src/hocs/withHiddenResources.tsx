// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {HiddenResourcesProvider, type HiddenResources} from '@thunderid/contexts';
import type {JSX, ComponentType} from 'react';

/**
 * The resources the platform gives every organization to administer its gateways with, which the
 * organization does not manage: the Admin Console application a gateway's console signs in with, and
 * the Data Plane resource server that carries the gateway's system permission. Lists and detail pages
 * know them by client id and identifier, and a deploy's changes by id.
 */
export const CONTROL_PLANE_HIDDEN_RESOURCES: HiddenResources = {
  applicationClientIds: ['ADMIN_CONSOLE'],
  resourceServerIdentifiers: ['var:DATA_PLANE_URL'],
  ids: {
    application: ['01a19b04-d7f7-7790-ad53-9139664d0a6a'],
    resource_server: ['01a1864b-ef42-7c41-b0a2-97f53fd3b1e4'],
  },
};

/**
 * Keeps the platform's own resources out of the console's views. They still deploy with everything
 * else.
 */
export default function withHiddenResources<P extends object>(WrappedComponent: ComponentType<P>) {
  return function WithHiddenResources(props: P): JSX.Element {
    return (
      <HiddenResourcesProvider hidden={CONTROL_PLANE_HIDDEN_RESOURCES}>
        <WrappedComponent {...props} />
      </HiddenResourcesProvider>
    );
  };
}
