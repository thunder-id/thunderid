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

import React, {useEffect, useMemo, useState} from 'react';
import Link from '@docusaurus/Link';
import Layout from '@theme/Layout';
import {useBaseUrlUtils} from '@docusaurus/useBaseUrl';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import type {DocusaurusProductConfig} from '@site/docusaurus.product.config';
import {Avatar, AvatarGroup, Tooltip} from '@wso2/oxygen-ui';
import GithubIcon from '@site/src/components/icons/GithubIcon';
import IOSLogo from '@site/src/components/icons/IOSLogo';
import LinuxLogo from '@site/src/components/icons/LinuxLogo';
import WindowsLogo from '@site/src/components/icons/WindowsLogo';

const OS_LABELS: Record<DistributionAsset['os'], string> = {
  linux: 'Linux',
  macos: 'Mac OS',
  win: 'Windows',
};

const OS_ICONS: Record<DistributionAsset['os'], React.ReactNode> = {
  linux: <LinuxLogo size={18} />,
  macos: <IOSLogo size={18} />,
  win: <WindowsLogo size={18} />,
};

interface ReleaseAsset {
  contentType: string;
  downloadCount: number;
  downloadUrl: string;
  id: number;
  name: string;
  sizeBytes: number;
  sizeLabel: string;
  updatedAt: string;
}

interface DistributionAsset extends ReleaseAsset {
  architecture: 'arm64' | 'x64';
  architectureLabel: string;
  os: 'linux' | 'macos' | 'win';
  osLabel: string;
}

interface DetectedPlatform {
  architecture: DistributionAsset['architecture'] | null;
  os: DistributionAsset['os'] | null;
}

interface NavigatorWithUserAgentData extends Navigator {
  userAgentData?: {
    getHighEntropyValues?: (
      hints: ('architecture' | 'bitness' | 'platform')[],
    ) => Promise<{architecture?: string; bitness?: string; platform?: string}>;
  };
}

interface Contributor {
  avatarUrl: string | null;
  profileUrl: string;
  username: string;
}

interface ReleaseChanges {
  breaking: string[];
  bugs: string[];
  features: string[];
  improvements: string[];
}

interface ReleaseEntry {
  assets: ReleaseAsset[];
  body: string;
  changes: ReleaseChanges;
  contributors: Contributor[];
  htmlUrl: string;
  id: number;
  isDraft: boolean;
  isLatest: boolean;
  isPrerelease: boolean;
  name: string;
  newContributors: Contributor[];
  primaryDownloadUrl: string;
  publishedAt: string;
  publishedDateLabel: string;
  tagName: string;
}

interface ReleasesData {
  generatedAt: string;
  latestRelease: ReleaseEntry | null;
  releases: ReleaseEntry[];
  repository: {
    description: string;
    forks: number;
    fullName: string;
    releasesUrl: string;
    stars: number;
    subscribers: number;
    url: string;
  };
}

const RELEASES_UNAVAILABLE_MESSAGE = 'Release information is currently unavailable. Please check back soon.';

function getDistributionAsset(asset: ReleaseAsset): DistributionAsset | null {
  const match = /^thunder(?:id)?-[0-9A-Za-z.+-]+-(macos|linux|win)-(arm64|x64)\.zip$/i.exec(asset.name);

  if (!match) {
    return null;
  }

  const [, os, architecture] = match;

  const architectureLabelMap: Record<string, string> = {
    arm64: os === 'macos' ? 'ARM64 (Apple Silicon)' : 'ARM64',
    x64: os === 'macos' ? 'x64 (Intel)' : 'x64',
  };

  return {
    ...asset,
    architecture: architecture as DistributionAsset['architecture'],
    architectureLabel: architectureLabelMap[architecture],
    os: os as DistributionAsset['os'],
    osLabel: OS_LABELS[os as DistributionAsset['os']],
  };
}

function detectOperatingSystem(userAgent: string, platform: string): DetectedPlatform['os'] {
  const normalizedPlatform = platform.toLowerCase();
  const normalizedUserAgent = userAgent.toLowerCase();

  if (normalizedPlatform.includes('mac') || /(mac os x|macintosh)/.test(normalizedUserAgent)) {
    return 'macos';
  }

  if (normalizedPlatform.includes('win') || normalizedUserAgent.includes('windows')) {
    return 'win';
  }

  if (normalizedPlatform.includes('linux') || normalizedUserAgent.includes('linux')) {
    return 'linux';
  }

  return null;
}

function detectArchitecture(
  userAgent: string,
  operatingSystem: DetectedPlatform['os'],
): DetectedPlatform['architecture'] {
  const normalizedUserAgent = userAgent.toLowerCase();
  const hasArmToken = /(arm64|aarch64|armv8|apple silicon|silicon)/.test(normalizedUserAgent);
  const hasX64Token = /\b(wow64|win64|x64|x86_64|amd64|intel)\b/.test(normalizedUserAgent);

  if (hasArmToken) {
    return 'arm64';
  }

  if (hasX64Token) {
    // Safari on Apple Silicon often exposes Intel-style tokens in the UA string.
    // Avoid recommending x64 on macOS unless we have explicit ARM evidence.
    if (operatingSystem === 'macos') {
      return null;
    }

    return 'x64';
  }

  return null;
}

async function detectPlatform(): Promise<DetectedPlatform> {
  if (typeof navigator === 'undefined') {
    return {architecture: null, os: null};
  }

  const {platform, userAgent} = navigator;
  const {userAgentData} = navigator as NavigatorWithUserAgentData;
  const fallbackOs = detectOperatingSystem(userAgent, platform);

  const fallback = {
    architecture: detectArchitecture(userAgent, fallbackOs),
    os: fallbackOs,
  };

  if (!userAgentData?.getHighEntropyValues) {
    return fallback;
  }

  try {
    const values = await userAgentData.getHighEntropyValues(['architecture', 'bitness', 'platform']);
    const {architecture: detectedArchitectureValue, bitness: detectedBitness, platform: detectedPlatformValue} = values;
    const detectedPlatform = detectedPlatformValue?.toLowerCase() ?? '';
    const architecture = detectedArchitectureValue?.toLowerCase() ?? '';
    const bitness = detectedBitness?.toLowerCase() ?? '';
    const {architecture: fallbackArchitecture, os: fallbackOs} = fallback;
    let os = fallbackOs;
    let resolvedArchitecture = fallbackArchitecture;

    if (detectedPlatform === 'macos') {
      os = 'macos';
    } else if (detectedPlatform === 'windows') {
      os = 'win';
    } else if (detectedPlatform === 'linux') {
      os = 'linux';
    }

    if (architecture === 'arm') {
      resolvedArchitecture = 'arm64';
    } else if (architecture === 'x86' && bitness === '64') {
      resolvedArchitecture = 'x64';
    }

    return {architecture: resolvedArchitecture, os};
  } catch {
    return fallback;
  }
}

function ContributorsAvatarGroup({contributors}: {contributors: Contributor[]}) {
  return (
    <AvatarGroup className="releases-contributors-avatar-group" max={5} total={contributors.length}>
      {contributors.map((contributor) => (
        <Tooltip key={contributor.username} title={`@${contributor.username}`} arrow>
          <a
            className="releases-contributor-avatar-link"
            href={contributor.profileUrl}
            target="_blank"
            rel="noreferrer"
            aria-label={`View @${contributor.username} on GitHub`}
          >
            <Avatar alt={contributor.username} src={contributor.avatarUrl ?? undefined}>
              {contributor.username.charAt(0).toUpperCase()}
            </Avatar>
          </a>
        </Tooltip>
      ))}
    </AvatarGroup>
  );
}

function normalizeReleaseChangeItem(item: string): string {
  const trimmed = item.trim();

  if (!trimmed) {
    return '';
  }

  // Drop malformed markdown-only headings like "*Linux/macOS:**" that appear in older release notes.
  if (/^\*[^*]+:\*\*\s*$/i.test(trimmed)) {
    return '';
  }

  return trimmed
    .replace(/^\*\s*/, '')
    .replace(/^\*([^*]+):\*\*\s*/i, '$1: ')
    .replace(/\*\*/g, '')
    .trim();
}

function ChangeGroup({title, items}: {items: string[]; title: string}) {
  const filteredItems = items
    .map(normalizeReleaseChangeItem)
    .filter((item) => item && !/made their first contribution/i.test(item));

  if (filteredItems.length === 0) {
    return null;
  }

  return (
    <section className="releases-change-group">
      <h3>{title}</h3>
      <ul>
        {filteredItems.map((item) => (
          <li key={item}>{item}</li>
        ))}
      </ul>
    </section>
  );
}

function ReleaseCard({release}: {release: ReleaseEntry}) {
  const distributionAssets = useMemo(
    () => release.assets.map(getDistributionAsset).filter((asset): asset is DistributionAsset => asset !== null),
    [release.assets],
  );
  const [detectedPlatform, setDetectedPlatform] = useState<DetectedPlatform | null>(null);

  useEffect(() => {
    let isMounted = true;

    detectPlatform()
      .then((platform) => {
        if (!isMounted) {
          return;
        }

        setDetectedPlatform(platform);
      })
      .catch(() => undefined);

    return () => {
      isMounted = false;
    };
  }, [release.id]);

  const detectedAsset = useMemo(
    () =>
      distributionAssets.find(
        (asset) => asset.os === detectedPlatform?.os && asset.architecture === detectedPlatform?.architecture,
      ) ??
      distributionAssets.find((asset) => asset.os === detectedPlatform?.os) ??
      null,
    [detectedPlatform, distributionAssets],
  );
  const selectedAsset = useMemo(
    () => detectedAsset ?? distributionAssets[0] ?? null,
    [detectedAsset, distributionAssets],
  );
  const hasRenderedChanges = useMemo(() => {
    const changeItemsByCategory: string[][] = [
      release.changes.breaking,
      release.changes.features,
      release.changes.improvements,
      release.changes.bugs,
    ];

    return changeItemsByCategory.some((items: string[]) =>
      items.some((item: string) => {
        const normalized = normalizeReleaseChangeItem(item);

        if (!normalized || /made their first contribution/i.test(normalized)) {
          return false;
        }

        return normalized.length > 0;
      }),
    );
  }, [release.changes]);
  const matchingAssetId = detectedAsset?.id;
  const groupedAssetsByOs = useMemo(() => {
    const osOrder: DistributionAsset['os'][] = ['linux', 'win', 'macos'];
    const recommendedOs = detectedAsset?.os;

    const prioritizedOsOrder = recommendedOs
      ? [recommendedOs, ...osOrder.filter((os) => os !== recommendedOs)]
      : osOrder;

    return prioritizedOsOrder
      .map((os) => ({
        assets: distributionAssets.filter((asset) => asset.os === os),
        os,
      }))
      .filter((group) => group.assets.length > 0);
  }, [distributionAssets, detectedAsset?.os]);

  return (
    <article className="releases-release-card">
      <div className="releases-release-header">
        <div>
          <div className="releases-release-title-row">
            <h2>{release.tagName}</h2>
            {release.isLatest ? <span className="releases-release-badge">Latest release</span> : null}
            {release.isPrerelease ? <span className="releases-release-badge">Pre-release</span> : null}
          </div>
          <p>Released on {release.publishedDateLabel}</p>
        </div>
      </div>

      {distributionAssets.length > 0 ? (
        <section className="releases-section-block">
          <h2>Downloads</h2>
          {selectedAsset ? (
            <div className="releases-download-feature">
              <div className="releases-download-feature-copy">
                <span className="releases-download-feature-kicker">
                  {matchingAssetId === selectedAsset.id ? 'Recommended for this device' : 'Selected download'}
                </span>
                <h3>
                  {selectedAsset.osLabel} · {selectedAsset.architectureLabel}
                </h3>
                <div className="releases-download-feature-meta">
                  <span>{selectedAsset.sizeLabel}</span>
                  <span>{selectedAsset.downloadCount.toLocaleString('en-US')} downloads</span>
                  <span>{selectedAsset.name}</span>
                </div>
              </div>
              <a
                className="releases-download-primary"
                href={selectedAsset.downloadUrl}
                target="_blank"
                rel="noreferrer"
              >
                <span>Download for {selectedAsset.osLabel}</span>
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
          ) : null}
          <div className="releases-other-downloads-grid">
            {groupedAssetsByOs.map(({assets, os}) => (
              <section key={os} className="releases-other-downloads-card">
                <header className="releases-other-downloads-header">
                  <span aria-hidden="true" className="releases-other-downloads-os-icon">
                    {OS_ICONS[os]}
                  </span>
                  <h4>{OS_LABELS[os]}</h4>
                </header>
                <div className="releases-other-downloads-architectures">
                  {assets.map((asset) => {
                    const isRecommended = matchingAssetId === asset.id;

                    return (
                      <a
                        key={asset.id}
                        className="releases-other-downloads-architecture"
                        href={asset.downloadUrl}
                        target="_blank"
                        rel="noreferrer"
                      >
                        <span className="releases-other-downloads-architecture-title">
                          {asset.osLabel} {asset.architectureLabel} ({asset.name.slice(asset.name.lastIndexOf('.'))})
                        </span>
                        <span className="releases-other-downloads-architecture-meta">{asset.sizeLabel}</span>
                        {isRecommended ? <em>Recommended</em> : null}
                      </a>
                    );
                  })}
                </div>
              </section>
            ))}
          </div>
        </section>
      ) : null}

      {release.contributors.length > 0 ? (
        <section className="releases-section-block">
          <h2>Contributors ({release.contributors.length})</h2>
          <ContributorsAvatarGroup contributors={release.contributors} />
        </section>
      ) : null}

      {release.newContributors.length > 0 ? (
        <section className="releases-section-block">
          <h3>New Contributors ({release.newContributors.length})</h3>
          <ContributorsAvatarGroup contributors={release.newContributors} />
        </section>
      ) : null}

      {hasRenderedChanges ? (
        <section className="releases-section-block">
          <h2>What's Changed</h2>
          <div className="releases-change-grid">
            <ChangeGroup title="⚠️ Breaking Changes" items={release.changes.breaking} />
            <ChangeGroup title="🚀 Features" items={release.changes.features} />
            <ChangeGroup title="✨ Improvements" items={release.changes.improvements} />
            <ChangeGroup title="🐛 Bug Fixes" items={release.changes.bugs} />
          </div>
        </section>
      ) : null}
    </article>
  );
}

export default function ReleasesPage() {
  const {siteConfig} = useDocusaurusContext();
  const project = siteConfig.customFields?.product as DocusaurusProductConfig | undefined;
  const productName = project?.project?.name ?? siteConfig.title;
  const repoUrl = project?.project?.source?.github?.url ?? '';
  const githubReleasesUrl = repoUrl ? `${repoUrl}/releases` : '#';
  const {withBaseUrl} = useBaseUrlUtils();
  const [data, setData] = useState<ReleasesData | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [selectedReleaseId, setSelectedReleaseId] = useState<number | null>(null);

  useEffect(() => {
    const controller = new AbortController();

    async function loadReleases() {
      try {
        const response = await fetch(withBaseUrl('/data/releases.json'), {
          signal: controller.signal,
        });

        if (!response.ok) {
          throw new Error(RELEASES_UNAVAILABLE_MESSAGE);
        }

        const payloadText = await response.text();
        let payload: ReleasesData;

        try {
          payload = JSON.parse(payloadText) as ReleasesData;
        } catch {
          throw new Error(RELEASES_UNAVAILABLE_MESSAGE);
        }

        if (!payload || typeof payload !== 'object' || !Array.isArray(payload.releases)) {
          throw new Error(RELEASES_UNAVAILABLE_MESSAGE);
        }

        setError(null);
        setData(payload);
        setSelectedReleaseId(payload.latestRelease?.id ?? payload.releases[0]?.id ?? null);
      } catch {
        if (controller.signal.aborted) {
          return;
        }

        setError(RELEASES_UNAVAILABLE_MESSAGE);
      }
    }

    loadReleases().catch(() => undefined);

    return () => controller.abort();
  }, [withBaseUrl]);
  const repository = data?.repository;
  const releases = data?.releases ?? [];
  const selectedRelease =
    releases.find((release: ReleaseEntry) => release.id === selectedReleaseId) ?? data?.latestRelease ?? null;

  return (
    <Layout
      title="Releases"
      description={`Explore ${productName} releases, changelogs, and downloads.`}
      wrapperClassName="releases-page"
    >
      <main className="releases-shell">
        <section className="releases-hero">
          <h1>
            Releases
          </h1>
          <p>Explore every release with detailed changelogs, download options, and the people building {productName}.</p>

          <div className="releases-hero-actions">
            <a
              className="button button--secondary"
              href={repository?.releasesUrl ?? githubReleasesUrl}
              target="_blank"
              rel="noreferrer"
            >
              <span style={{display: 'inline-flex', alignItems: 'center', gap: '0.5rem'}}>
                <span aria-hidden="true" style={{display: 'inline-flex'}}>
                  <GithubIcon size={16} />
                </span>
                <span>View on GitHub</span>
                <svg width="12" height="12" aria-label="(opens in new tab)" className="iconExternalLink_lApV">
                  <use href="#theme-svg-external-link"></use>
                </svg>
              </span>
            </a>
          </div>
        </section>

        {error ? <p className="releases-status">{error}</p> : null}
        {!data && !error ? <p className="releases-status">Loading releases...</p> : null}
        {data && !selectedRelease ? (
          <p className="releases-status">No releases are available right now. Please check back soon.</p>
        ) : null}

        {selectedRelease ? (
          <section className="releases-latest">
            <div className="releases-release-toolbar">
              <label className="releases-version-picker" htmlFor="releases-version-select">
                <span>Version</span>
                <select
                  id="releases-version-select"
                  value={selectedRelease.id}
                  onChange={(event) => {
                    setSelectedReleaseId(Number(event.target.value));
                  }}
                >
                  {releases.map((release) => (
                    <option key={release.id} value={release.id}>
                      {release.tagName}
                      {release.isPrerelease ? ' · Pre-release' : ''}
                    </option>
                  ))}
                </select>
              </label>
            </div>
            <ReleaseCard release={selectedRelease} />
          </section>
        ) : null}

        <section className="releases-footer-note">
          <p>
            Source:{' '}
            <Link to={repository?.url ?? repoUrl}>
              {repository?.fullName ?? siteConfig.title}
            </Link>
            {data?.generatedAt ? ` · Updated ${new Date(data.generatedAt).toLocaleString('en-US')}` : ''}
          </p>
        </section>
      </main>
    </Layout>
  );
}
