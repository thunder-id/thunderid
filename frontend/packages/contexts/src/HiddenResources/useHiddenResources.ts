// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useContext, useMemo} from 'react';
import HiddenResourcesContext from './HiddenResourcesContext';

/**
 * Checks for whether a resource is hidden from the console's views. Values are matched ignoring case,
 * and a missing value is never hidden.
 *
 * @public
 */
export interface HiddenResourceChecks {
  /** Whether the application with this OAuth client id is hidden. */
  isHiddenApplication: (clientId: string | undefined) => boolean;
  /** Whether the resource server with this identifier is hidden. */
  isHiddenResourceServer: (identifier: string | undefined) => boolean;
  /** Whether the resource of this type, e.g. `application`, with this id is hidden. */
  isHiddenResource: (resourceType: string, id: string | undefined) => boolean;
}

/**
 * Whether a value is in a list, ignoring case.
 *
 * @param list - The values to look in.
 * @param value - The value to look for; a missing one is never found.
 */
function includesIgnoringCase(list: string[] | undefined, value: string | undefined): boolean {
  if (!value || !list) {
    return false;
  }
  const normalized = value.toLowerCase();
  return list.some((entry: string) => entry.toLowerCase() === normalized);
}

/**
 * React hook for checking whether a resource is hidden, as the nearest `HiddenResourcesProvider`
 * declares. Without one, nothing is hidden.
 *
 * @public
 */
export default function useHiddenResources(): HiddenResourceChecks {
  const hidden = useContext(HiddenResourcesContext);

  return useMemo(
    () => ({
      isHiddenApplication: (clientId: string | undefined): boolean =>
        includesIgnoringCase(hidden.applicationClientIds, clientId),
      isHiddenResourceServer: (identifier: string | undefined): boolean =>
        includesIgnoringCase(hidden.resourceServerIdentifiers, identifier),
      isHiddenResource: (resourceType: string, id: string | undefined): boolean =>
        includesIgnoringCase(hidden.ids?.[resourceType], id),
    }),
    [hidden],
  );
}
