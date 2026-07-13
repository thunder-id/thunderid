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
import {styled} from '@wso2/oxygen-ui';
import React, {ReactNode, useEffect, useMemo, useState} from 'react';
import IOSLogo from '@site/src/components/icons/IOSLogo';
import LinuxLogo from '@site/src/components/icons/LinuxLogo';
import WindowsLogo from '@site/src/components/icons/WindowsLogo';
import {
  OtherDownloadsActionIcon,
  OtherDownloadsArchitecture,
  OtherDownloadsArchitectureBadge,
  OtherDownloadsArchitectureInfo,
  OtherDownloadsArchitectureMeta,
  OtherDownloadsArchitectureTitle,
  OtherDownloadsArchitectures,
  OtherDownloadsCard,
  OtherDownloadsGrid,
  OtherDownloadsHeader,
  OtherDownloadsHeaderTitle,
  OtherDownloadsOsIcon,
} from '@site/src/components/OtherDownloadsGrid';
import usePlatform from '@site/src/hooks/usePlatform';
import {
  groupAssetsByOs,
  parseDistributionAssets,
  pickAssetForPlatform,
  type DistributionAsset,
  type ReleasesData,
} from '@site/src/utils/downloadAssets';
import type {OsKey} from '@site/src/utils/platform';

/**
 * Props for the DownloadCard component.
 */
interface DownloadCardProps {
  /**
   * A regex pattern to match the desired asset from the releases data.
   * The pattern should include named capture groups for `os` and `arch` to enable platform-specific selection.
   * For example: `^sample-app-wayfinder-[0-9A-Za-z.+-]+-(macos|linux|win)-(arm64|x64)\\.zip$`
   */
  pattern: RegExp;
  /**
   * Content to display if there was an error fetching the releases data or if no assets matched the pattern.
   */
  fallback?: ReactNode;
  /**
   * Optional content to display in the card footer, below the download options.
   * This can be used for additional instructions, links, or disclaimers related to the download.
   */
  footer?: ReactNode;
  /**
   * Whether to show download options for all platforms found in the releases data, or just the one matching the
   * user's platform. If `true`, platforms will be grouped by OS and the recommended option for the user's platform
   * will be highlighted. Defaults to `false`.
   */
  showAllPlatforms?: boolean;
  /**
   * When `showAllPlatforms` is `true`, whether to initially collapse the other platforms behind a
   * "Show other platforms" toggle. Defaults to `false`.
   */
  collapseOtherPlatforms?: boolean;
  /**
   * Whether to use a more compact layout for the card, with smaller text and tighter spacing. Defaults to `false`.
   */
  compact?: boolean;
}

// Mapping of OS keys to their corresponding icons and labels for display in the UI.
const OS_ICONS: Record<OsKey, React.ReactNode> = {
  linux: <LinuxLogo size={18} />,
  macos: <IOSLogo size={18} />,
  win: <WindowsLogo size={18} />,
};

// Mapping of OS keys to their human-readable labels for display in the UI.
const OS_LABELS: Record<OsKey, string> = {
  linux: 'Linux',
  macos: 'Mac OS',
  win: 'Windows',
};

const DownloadFeature = styled('div')<{ownerState: {compact: boolean}}>(({ownerState}) => ({
  display: 'grid',
  gridTemplateColumns: 'minmax(0, 1fr) auto',
  gap: ownerState.compact ? '1rem' : '1.25rem',
  alignItems: 'center',
  marginBottom: '1rem',
  padding: ownerState.compact ? '1rem 1.1rem' : '1.35rem',
  borderRadius: ownerState.compact ? '1rem' : '1.25rem',
  border: '1px solid var(--ifm-color-emphasis-200)',
  background: 'var(--oxygen-palette-background-paper)',
  '@media (max-width: 996px)': {
    gridTemplateColumns: '1fr',
  },
}));

const DownloadFeatureCopy = styled('div')({
  display: 'grid',
  gap: '0.65rem',
  minWidth: 0,
});

const DownloadFeatureKicker = styled('span')<{ownerState: {compact: boolean}}>(({ownerState}) => ({
  color: 'var(--ifm-color-primary)',
  fontSize: ownerState.compact ? '0.7rem' : '0.78rem',
  fontWeight: 700,
  letterSpacing: '0.04em',
  textTransform: 'uppercase',
}));

const DownloadFeatureTitle = styled('h3')<{ownerState: {compact: boolean}}>(({ownerState}) => ({
  margin: 0,
  fontSize: ownerState.compact ? '1.15rem' : '1.5rem',
}));

const DownloadFeatureMeta = styled('div')<{ownerState: {compact: boolean}}>(({ownerState}) => ({
  display: 'flex',
  flexWrap: 'wrap',
  gap: ownerState.compact ? '0.4rem 0.85rem' : '0.65rem 1rem',
  minWidth: 0,
  overflowWrap: 'anywhere',
  color: 'var(--ifm-color-emphasis-600)',
  fontSize: ownerState.compact ? '0.82rem' : '0.9rem',
}));

const DownloadPrimaryButton = styled('a')<{ownerState: {compact: boolean}}>(({ownerState}) => ({
  display: 'inline-flex',
  alignItems: 'center',
  justifyContent: 'center',
  gap: '0.75rem',
  minHeight: ownerState.compact ? '2.4rem' : '2.9rem',
  padding: ownerState.compact ? '0.45rem 0.85rem' : '0.65rem 1rem',
  borderRadius: '999px',
  background: 'var(--ifm-color-primary)',
  color: '#fff',
  fontSize: ownerState.compact ? '0.82rem' : '0.92rem',
  fontWeight: 700,
  textDecoration: 'none',
  transition: 'background 0.15s, transform 0.15s, box-shadow 0.15s',
  '&:hover': {
    background: 'var(--ifm-color-primary-dark)',
    color: '#fff',
    textDecoration: 'none',
    transform: 'translateY(-2px)',
    boxShadow: '0 14px 30px rgb(var(--oxygen-palette-primary-mainChannel) / 0.35)',
  },
  '@media (max-width: 996px)': {
    width: '100%',
  },
}));

const DownloadIcon = styled('span')<{ownerState: {compact: boolean}}>(({ownerState}) => ({
  display: 'inline-flex',
  alignItems: 'center',
  justifyContent: 'center',
  width: ownerState.compact ? '1.45rem' : '1.8rem',
  height: ownerState.compact ? '1.45rem' : '1.8rem',
  borderRadius: '999px',
  background: 'rgb(var(--oxygen-palette-primary-mainChannel) / 0.1)',
  color: 'var(--ifm-color-primary)',
}));

const OtherDownloadsDetails = styled('details')({
  marginTop: '1rem',
  '& > summary': {
    cursor: 'pointer',
    fontSize: '0.9rem',
    fontWeight: 600,
    color: 'var(--ifm-color-emphasis-700)',
    padding: '0.4rem 0',
  },
  '&[open] > summary': {
    marginBottom: '0.75rem',
  },
});

/**
 * A card component that displays a download link for the latest release asset matching a specified pattern,
 * with optional support for showing all platform options and additional footer content.
 */
export default function DownloadCard({
  pattern,
  fallback = 'Unable to load download options at this time.',
  footer = null,
  showAllPlatforms = false,
  collapseOtherPlatforms = false,
  compact = false,
}: DownloadCardProps): ReactNode {
  const {withBaseUrl} = useBaseUrlUtils();
  const platform = usePlatform();
  const [assets, setAssets] = useState<DistributionAsset[] | null>(null);
  const [tag, setTag] = useState<string>('');
  const [errored, setErrored] = useState(false);
  const ownerState = {compact};

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
        setErrored(false);
        setTag(release.tagName);
        setAssets(parseDistributionAssets(release.assets, pattern));
      })
      .catch((err: unknown) => {
        if (err instanceof DOMException && err.name === 'AbortError') return;
        setErrored(true);
      });
    return () => controller.abort();
  }, [withBaseUrl, pattern]);

  const selected = useMemo(() => pickAssetForPlatform(assets ?? [], platform), [assets, platform]);
  const matched = selected && selected.os === platform?.os && selected.arch === platform?.arch;
  const groups = useMemo(
    () => (showAllPlatforms ? groupAssetsByOs(assets ?? [], selected?.os ?? null) : []),
    [assets, selected, showAllPlatforms],
  );

  if (errored || (assets !== null && assets.length === 0)) {
    return fallback;
  }

  if (!selected) {
    return null;
  }

  return (
    <div>
      <DownloadFeature ownerState={ownerState}>
        <DownloadFeatureCopy>
          <DownloadFeatureKicker ownerState={ownerState}>
            {matched ? 'Recommended for this device' : 'Selected download'}
          </DownloadFeatureKicker>
          <DownloadFeatureTitle ownerState={ownerState}>
            {selected.osLabel} · {selected.archLabel}
          </DownloadFeatureTitle>
          <DownloadFeatureMeta ownerState={ownerState}>
            <span>{selected.sizeLabel}</span>
            {tag ? <span>{tag}</span> : null}
            <span>{selected.name}</span>
          </DownloadFeatureMeta>
        </DownloadFeatureCopy>
        <DownloadPrimaryButton ownerState={ownerState} href={selected.downloadUrl} target="_blank" rel="noreferrer">
          <span>Download for {selected.osLabel}</span>
          <DownloadIcon ownerState={ownerState} aria-hidden="true">
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
          </DownloadIcon>
        </DownloadPrimaryButton>
      </DownloadFeature>
      {showAllPlatforms && groups.length > 0
        ? (() => {
            const grid = (
              <OtherDownloadsGrid ownerState={ownerState}>
                {groups.map(({os, assets: osAssets}) => (
                  <OtherDownloadsCard key={os}>
                    <OtherDownloadsHeader>
                      <OtherDownloadsOsIcon aria-hidden="true">{OS_ICONS[os]}</OtherDownloadsOsIcon>
                      <OtherDownloadsHeaderTitle>{OS_LABELS[os]}</OtherDownloadsHeaderTitle>
                    </OtherDownloadsHeader>
                    <OtherDownloadsArchitectures>
                      {osAssets.map((asset) => {
                        const isRecommended = asset.downloadUrl === selected.downloadUrl;
                        return (
                          <OtherDownloadsArchitecture
                            key={asset.name}
                            href={asset.downloadUrl}
                            target="_blank"
                            rel="noreferrer"
                          >
                            <OtherDownloadsArchitectureInfo>
                              <OtherDownloadsArchitectureTitle>
                                {asset.osLabel} {asset.archLabel} ({asset.name.slice(asset.name.lastIndexOf('.'))})
                              </OtherDownloadsArchitectureTitle>
                              <OtherDownloadsArchitectureMeta>{asset.sizeLabel}</OtherDownloadsArchitectureMeta>
                              {isRecommended ? <OtherDownloadsArchitectureBadge>Recommended</OtherDownloadsArchitectureBadge> : null}
                            </OtherDownloadsArchitectureInfo>
                            <OtherDownloadsActionIcon />
                          </OtherDownloadsArchitecture>
                        );
                      })}
                    </OtherDownloadsArchitectures>
                  </OtherDownloadsCard>
                ))}
              </OtherDownloadsGrid>
            );
            return collapseOtherPlatforms ? (
              <OtherDownloadsDetails>
                <summary>Other Download Options</summary>
                {grid}
              </OtherDownloadsDetails>
            ) : (
              grid
            );
          })()
        : null}
      {footer}
    </div>
  );
}
