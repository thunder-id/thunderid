// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useEffect, useState} from 'react';
import {isAvailable} from './presentation';
import type {EcosystemEntry} from '@site/src/types/ecosystem';

interface SdkRelease {
  packageId: string;
  publishedDateLabel: string | null;
}

/**
 * Facts about a package that would go stale if written into the registry:
 * how often it is downloaded, and when it was last published.
 *
 * Both come from sources the project already has. Downloads are read live from
 * the npm registry's public endpoint; the published date comes from
 * `static/data/sdk-releases.json`, which `scripts/generate-sdk-releases.mjs`
 * builds from each SDK repo's own releases. Either may be unavailable, and the
 * hero simply omits the line rather than showing a guess.
 */
export default function useEntryFacts(entry: EcosystemEntry): {downloads?: string; updated?: string} {
  const [downloads, setDownloads] = useState<string>();
  const [updated, setUpdated] = useState<string>();

  const isNpm = entry.package.manager === 'npm' && isAvailable(entry);
  const packageName = entry.package.name;
  const {id} = entry;

  useEffect(() => {
    if (!isNpm) return undefined;
    let active = true;
    fetch(`https://api.npmjs.org/downloads/point/last-week/${packageName}`)
      .then((res) => res.json())
      .then((data: {downloads?: number}) => {
        if (!active || typeof data.downloads !== 'number' || data.downloads <= 0) return;
        const count =
          data.downloads >= 1000 ? `${(data.downloads / 1000).toFixed(1)}K` : String(data.downloads);
        setDownloads(`${count} weekly downloads`);
      })
      .catch(() => {
        // No downloads line; nothing else depends on it.
      });
    return () => {
      active = false;
    };
  }, [isNpm, packageName]);

  useEffect(() => {
    let active = true;
    fetch('/data/sdk-releases.json')
      .then((res) => res.json())
      .then((data: {releases?: SdkRelease[]}) => {
        const label = data.releases?.find((release) => release.packageId === id)?.publishedDateLabel;
        if (active && label) setUpdated(`Updated ${label}`);
      })
      .catch(() => {
        // No updated line.
      });
    return () => {
      active = false;
    };
  }, [id]);

  return {downloads, updated};
}
