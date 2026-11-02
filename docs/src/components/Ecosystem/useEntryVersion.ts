// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useEffect, useState} from 'react';
import {isAvailable} from './presentation';
import type {EcosystemEntry} from '@site/src/types/ecosystem';

/**
 * Resolves the version shown on a card and in a detail page hero.
 *
 * A pinned `version` in the registry wins. Otherwise npm packages are looked up
 * live, so a release shows up without a docs change. Everything else (Maven,
 * CocoaPods, pub, plugins) has no equivalent anonymous endpoint, so it renders
 * without a version rather than with a stale one.
 */
export default function useEntryVersion(entry: EcosystemEntry): string | undefined {
  const [fetched, setFetched] = useState<string>();
  const pinned = entry.version;
  const shouldFetch = !pinned && isAvailable(entry) && entry.package.manager === 'npm';
  const packageName = entry.package.name;

  useEffect(() => {
    if (!shouldFetch) return undefined;
    let active = true;
    fetch(`https://registry.npmjs.org/${packageName}/latest`)
      .then((res) => res.json())
      .then((data: {version?: string}) => {
        if (active && data.version) setFetched(`v${data.version}`);
      })
      .catch(() => {
        // A registry hiccup just means no version chip; nothing else depends on it.
      });
    return () => {
      active = false;
    };
  }, [shouldFetch, packageName]);

  return pinned ?? fetched;
}
