// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useEffect, useSyncExternalStore} from 'react';
import loadOxygenIcons, {
  getLoadedOxygenIcons,
  subscribeToOxygenIcons,
  type OxygenIconSet,
} from '../utils/loadOxygenIcons';

/**
 * Gives the icon set of `@wso2/oxygen-ui-icons-react`, for code that picks an icon by a name it only
 * knows at runtime. The set is loaded on first use (see {@link loadOxygenIcons}).
 *
 * @example
 * ```tsx
 * const icons = useOxygenIcons();
 * const icon = icons?.get(name);
 * return icon ? <OxygenIcon icon={icon} size={16} /> : null;
 * ```
 *
 * @param enabled - Whether the set is needed; nothing is loaded while it is false.
 * @returns The icon set, or undefined until it has loaded.
 */
export default function useOxygenIcons(enabled = true): OxygenIconSet | undefined {
  // The set comes from the loader's cache, so a load made by any caller reaches this component,
  // including a retry after this component's own load failed.
  const icons = useSyncExternalStore(subscribeToOxygenIcons, getLoadedOxygenIcons);

  useEffect(() => {
    if (icons || !enabled) {
      return;
    }
    loadOxygenIcons().catch(() => {
      // The set stays unloaded until a later load, by this or another component, succeeds.
    });
  }, [icons, enabled]);

  return icons;
}
