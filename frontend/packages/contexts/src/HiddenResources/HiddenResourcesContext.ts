// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Context, createContext} from 'react';

/**
 * The resources a console leaves out of its views, such as the ones its platform manages. A hidden
 * resource is still deployed with the others.
 *
 * @public
 */
export interface HiddenResources {
  /** OAuth client ids of the applications to hide. */
  applicationClientIds?: string[];
  /** Identifiers of the resource servers to hide. */
  resourceServerIdentifiers?: string[];
  /**
   * Ids of the resources to hide where only a resource's type and id are known, such as a deploy's
   * changes, keyed by resource type, e.g. `application` or `resource_server`.
   */
  ids?: Record<string, string[]>;
}

/**
 * React context carrying the resources a console hides.
 *
 * Defaults to an empty object, so a feature package rendered without a `HiddenResourcesProvider`
 * hides nothing.
 *
 * @public
 */
const HiddenResourcesContext: Context<HiddenResources> = createContext<HiddenResources>({});

export default HiddenResourcesContext;
