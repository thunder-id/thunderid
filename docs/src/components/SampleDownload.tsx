/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {useBaseUrlUtils} from '@docusaurus/useBaseUrl';
import React, {useEffect, useMemo, useState} from 'react';
import type {ReleaseAssetInput, ReleasesData} from '@site/src/utils/downloadAssets';

/**
 * Props for the SampleDownload component.
 */
interface SampleDownloadProps {
  /**
   * Sample name prefix, matching the asset filename before the version segment.
   * For example, `sample-app-wayfinder` matches `sample-app-wayfinder-1.0.0.zip`.
   */
  sample: string;
}

/**
 * Renders a download button for the latest release asset whose filename begins with the given
 * sample name prefix followed by a version segment (e.g. `sample-app-wayfinder-1.0.0.zip`).
 * Shows a fallback message if the asset cannot be found.
 */
export default function SampleDownload({sample}: SampleDownloadProps): React.ReactElement {
  const {withBaseUrl} = useBaseUrlUtils();
  const [asset, setAsset] = useState<ReleaseAssetInput | null>(null);
  const [tag, setTag] = useState('');
  const [errored, setErrored] = useState(false);

  const pattern = useMemo(() => {
    const escaped = sample.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
    return new RegExp(`^${escaped}-[0-9A-Za-z.+-]+\\.zip$`, 'i');
  }, [sample]);

  useEffect(() => {
    const controller = new AbortController();
    fetch(withBaseUrl('/data/releases.json'), {signal: controller.signal})
      .then((r) => r.json() as Promise<ReleasesData>)
      .then((data) => {
        const release = data.latestRelease ?? data.releases?.[0];
        if (!release) {
          setErrored(true);
          return;
        }
        const match = release.assets.find((a) => pattern.test(a.name));
        if (!match) {
          setErrored(true);
          return;
        }
        setErrored(false);
        setTag(release.tagName);
        setAsset(match);
      })
      .catch((err: unknown) => {
        if (err instanceof DOMException && err.name === 'AbortError') return;
        setErrored(true);
      });
    return () => controller.abort();
  }, [withBaseUrl, pattern]);

  if (errored) {
    return <p>The sample distribution is currently unavailable. Please check back soon.</p>;
  }

  if (!asset) return null;

  return (
    <div className="download-card download-card--compact">
      <div className="releases-download-feature">
        <div className="releases-download-feature-meta">
          {tag ? <span>{tag}</span> : null}
          {asset.sizeLabel ? <span>{asset.sizeLabel}</span> : null}
          <span>{asset.name}</span>
        </div>
        <a className="releases-download-primary" href={asset.downloadUrl} target="_blank" rel="noreferrer">
          <span>Download</span>
          <span
            className="releases-download-icon"
            aria-hidden="true"
            style={{display: 'inline-flex', alignItems: 'center'}}
          >
            <svg
              viewBox="0 0 24 24"
              fill="none"
              stroke="white"
              strokeWidth="2"
              strokeLinecap="round"
              strokeLinejoin="round"
              style={{width: '20px', height: '20px'}}
            >
              <path d="M12 3v12" />
              <path d="m7 10 5 5 5-5" />
              <path d="M5 21h14" />
            </svg>
          </span>
        </a>
      </div>
    </div>
  );
}
