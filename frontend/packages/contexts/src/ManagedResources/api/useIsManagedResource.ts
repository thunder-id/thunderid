// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useCallback} from 'react';
import useManagedResources, {type ManagedResourceType} from './useManagedResources';

/**
 * Tells whether a given resource is owned by the control plane, so a view can present it read only
 * instead of offering controls the server will refuse.
 *
 * While the answer is still loading nothing is reported as managed, so the UI does not flicker into
 * a read-only state and back. The server is the authority either way: it refuses the write with 403
 * even if a control slips through.
 */
export default function useIsManagedResource(type: ManagedResourceType): (id: string) => boolean {
  const {data} = useManagedResources();

  return useCallback(
    (id: string): boolean => {
      if (!data?.enabled || !id) {
        return false;
      }
      return (data.managed[type] ?? []).includes(id);
    },
    [data, type],
  );
}
