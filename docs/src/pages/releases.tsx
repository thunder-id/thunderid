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

import Link from '@docusaurus/Link';
import {useBaseUrlUtils} from '@docusaurus/useBaseUrl';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import Layout from '@theme/Layout';
import {Avatar, Box, styled, Typography} from '@wso2/oxygen-ui';
import React, {useEffect, useMemo, useState} from 'react';
import type {DocusaurusProductConfig} from '@site/docusaurus.product.config';
import GithubIcon from '@site/src/components/icons/GithubIcon';
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

type ChangeTone = 'breaking' | 'features' | 'improvements' | 'bugs';

const CHANGE_GROUP_ICONS: Record<ChangeTone, React.ReactNode> = {
  breaking: (
    <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z" />
      <line x1="12" y1="9" x2="12" y2="13" />
      <line x1="12" y1="17" x2="12.01" y2="17" />
    </svg>
  ),
  features: (
    <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
      <polygon points="12 2 15.09 8.26 22 9.27 17 14.14 18.18 21.02 12 17.77 5.82 21.02 7 14.14 2 9.27 8.91 8.26 12 2" />
    </svg>
  ),
  improvements: (
    <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
      <polyline points="23 6 13.5 15.5 8.5 10.5 1 18" />
      <polyline points="17 6 23 6 23 12" />
    </svg>
  ),
  bugs: (
    <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="12" cy="12" r="10" />
      <line x1="12" y1="8" x2="12" y2="12" />
      <line x1="12" y1="16" x2="12.01" y2="16" />
    </svg>
  ),
};

const CHANGE_GROUP_LABELS: Record<ChangeTone, string> = {
  breaking: 'Breaking Changes',
  features: 'New Features',
  improvements: 'Improvements',
  bugs: 'Bug Fixes',
};

const CHANGE_GROUP_TONE_COLORS: Record<ChangeTone, string> = {
  breaking: 'var(--ifm-color-danger)',
  features: 'var(--ifm-color-primary)',
  improvements: 'var(--ifm-color-info)',
  bugs: 'var(--ifm-color-warning)',
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

const Shell = styled('main')({
  minWidth: 0,
  overflowX: 'hidden',
  width: '100%',
  maxWidth: '1200px',
  margin: '0 auto',
  padding: '4rem 1.5rem 5rem',
  '@media (max-width: 996px)': {
    padding: '3rem 1rem 4rem',
  },
});

const StatusText = styled('p')({
  margin: '0 0 2rem',
  color: 'var(--ifm-color-emphasis-700)',
});

const LatestSection = styled('section')({
  display: 'grid',
  gridTemplateColumns: '240px minmax(0, 1fr)',
  gap: '3rem',
  alignItems: 'start',
  marginBottom: '3rem',
  '@media (max-width: 996px)': {
    gridTemplateColumns: '1fr',
    gap: '1.5rem',
  },
});

const VersionSidebar = styled('div')({
  position: 'sticky',
  top: 'calc(var(--ifm-navbar-height) + 1.5rem)',
  '@media (max-width: 996px)': {
    position: 'static',
  },
});

const VersionSidebarLabel = styled('div')({
  marginBottom: '0.75rem',
  color: 'var(--ifm-color-emphasis-600)',
  fontSize: '0.72rem',
  fontWeight: 700,
  letterSpacing: '0.08em',
  textTransform: 'uppercase',
});

const VersionList = styled('div')({
  display: 'flex',
  flexDirection: 'column',
  gap: '0.15rem',
  '@media (max-width: 996px)': {
    flexDirection: 'row',
    flexWrap: 'nowrap',
    overflowX: 'auto',
    gap: '0.4rem',
    paddingBottom: '0.25rem',
  },
});

const VersionListItem = styled('button')<{ownerState: {active: boolean}}>(({ownerState}) => ({
  display: 'flex',
  alignItems: 'center',
  gap: '0.6rem',
  width: '100%',
  padding: '0.6rem 0.75rem',
  border: 'none',
  borderRadius: '0.6rem',
  background: ownerState.active ? 'rgb(var(--oxygen-palette-primary-mainChannel) / 0.12)' : 'transparent',
  color: ownerState.active ? 'var(--ifm-color-primary)' : 'var(--ifm-color-emphasis-700)',
  font: 'inherit',
  fontSize: '0.85rem',
  fontWeight: 500,
  textAlign: 'left',
  cursor: 'pointer',
  transition: 'background 0.15s, color 0.15s',
  '@media (max-width: 996px)': {
    flexShrink: 0,
    width: 'auto',
  },
  ...(!ownerState.active && {
    '&:hover': {
      background: 'var(--ifm-hover-overlay)',
      color: 'var(--ifm-font-color-base)',
    },
  }),
}));

const VersionListItemTag = styled('span')({
  fontFamily: 'var(--ifm-font-family-monospace, monospace)',
});

const VersionListItemBadge = styled('span')({
  marginLeft: 'auto',
  padding: '0.12rem 0.55rem',
  borderRadius: '999px',
  background: 'var(--ifm-color-primary)',
  color: '#fff',
  fontSize: '0.68rem',
  fontWeight: 700,
  letterSpacing: '0.02em',
});

const VersionSidebarFooter = styled('div')({
  marginTop: '1.25rem',
  paddingTop: '1rem',
  borderTop: '1px solid var(--ifm-color-emphasis-200)',
  '@media (max-width: 996px)': {
    display: 'none',
  },
});

const VersionSidebarAllLink = styled(Link)({
  display: 'inline-flex',
  alignItems: 'center',
  gap: '0.35rem',
  color: 'var(--ifm-color-emphasis-600)',
  fontSize: '0.78rem',
  textDecoration: 'none',
  '&:hover': {
    color: 'var(--ifm-font-color-base)',
    textDecoration: 'none',
  },
});

const ReleaseCardRoot = styled('article')({
  width: '100%',
  minWidth: 0,
});

const ReleaseHeader = styled('div')({
  display: 'flex',
  justifyContent: 'space-between',
  gap: '1rem',
  '@media (max-width: 996px)': {
    flexDirection: 'column',
  },
});

const ReleaseHeaderDescription = styled('p')({
  color: 'var(--ifm-color-emphasis-700)',
});

const ReleaseTitleRow = styled('div')({
  display: 'flex',
  alignItems: 'center',
  flexWrap: 'wrap',
  gap: '0.75rem',
  marginBottom: '0.5rem',
});

const ReleaseHeading = styled('h2')({
  margin: 0,
  fontFamily: 'var(--ifm-font-family-monospace, monospace)',
  fontSize: '2rem',
  fontWeight: 600,
  letterSpacing: '-0.02em',
  lineHeight: 1.1,
  '@media (max-width: 996px)': {
    fontSize: '1.7rem',
  },
});

const ReleaseBadge = styled('span')<{ownerState: {tone: 'latest' | 'prerelease'}}>(({ownerState}) => ({
  display: 'inline-flex',
  alignItems: 'center',
  padding: '0.22rem 0.65rem',
  borderRadius: '999px',
  border: '1px solid transparent',
  fontSize: '0.75rem',
  fontWeight: 700,
  letterSpacing: '0.01em',
  ...(ownerState.tone === 'latest'
    ? {
        background: 'var(--ifm-color-primary)',
        color: '#fff',
      }
    : {
        borderColor: 'color-mix(in srgb, var(--ifm-color-warning) 40%, transparent)',
        background: 'color-mix(in srgb, var(--ifm-color-warning) 15%, transparent)',
        color: 'var(--ifm-color-warning)',
      }),
}));

const SectionBlock = styled('section')({
  marginTop: '2rem',
});

const ChangeGrid = styled('div')({
  display: 'grid',
  gap: '1.4rem',
  marginTop: '2rem',
});

const ChangeGroupSection = styled('section')<{ownerState: {tone: ChangeTone}}>(({ownerState}) => ({
  '--releases-change-tone': CHANGE_GROUP_TONE_COLORS[ownerState.tone],
}));

const ChangeGroupHeader = styled('div')({
  display: 'flex',
  alignItems: 'center',
  gap: '0.55rem',
  marginBottom: '0.75rem',
});

const ChangeGroupIcon = styled('span')({
  display: 'inline-flex',
  alignItems: 'center',
  justifyContent: 'center',
  width: '1.4rem',
  height: '1.4rem',
  borderRadius: '0.35rem',
  background: 'color-mix(in srgb, var(--releases-change-tone) 18%, transparent)',
  color: 'var(--releases-change-tone)',
  flexShrink: 0,
});

const ChangeGroupLabel = styled('span')({
  color: 'var(--releases-change-tone)',
  fontSize: '0.78rem',
  fontWeight: 700,
  letterSpacing: '0.03em',
  textTransform: 'uppercase',
});

const ChangeGroupList = styled('ul')({
  margin: 0,
  paddingLeft: 0,
  listStyle: 'none',
});

const ChangeGroupListItem = styled('li')({
  position: 'relative',
  padding: '0.5rem 0 0.5rem 1.1rem',
  borderBottom: '1px solid var(--ifm-color-emphasis-200)',
  fontSize: '1rem',
  fontWeight: 400,
  lineHeight: 1.55,
  overflowWrap: 'anywhere',
  '&:last-child': {
    borderBottom: 'none',
  },
  '&::before': {
    content: '""',
    position: 'absolute',
    top: '0.95rem',
    left: 0,
    width: '5px',
    height: '5px',
    borderRadius: '50%',
    background: 'var(--releases-change-tone)',
  },
});

const ContributorsList = styled('div')({
  display: 'flex',
  flexWrap: 'wrap',
  gap: '0.5rem',
});

const ContributorPill = styled('a')<{ownerState: {isNew: boolean}}>(({ownerState}) => ({
  display: 'inline-flex',
  alignItems: 'center',
  gap: '0.45rem',
  padding: '0.35rem 0.7rem 0.35rem 0.35rem',
  borderRadius: '999px',
  fontSize: '0.82rem',
  fontWeight: 500,
  textDecoration: 'none',
  transition: 'background 0.15s, border-color 0.15s',
  border: ownerState.isNew
    ? '1px solid rgb(var(--oxygen-palette-primary-mainChannel) / 0.3)'
    : '1px solid var(--ifm-color-emphasis-200)',
  background: ownerState.isNew
    ? 'rgb(var(--oxygen-palette-primary-mainChannel) / 0.08)'
    : 'var(--oxygen-palette-background-paper)',
  color: ownerState.isNew ? 'var(--ifm-color-primary)' : 'var(--ifm-color-emphasis-700)',
  '&:hover': {
    borderColor: 'rgb(var(--oxygen-palette-primary-mainChannel) / 0.4)',
    background: 'rgb(var(--oxygen-palette-primary-mainChannel) / 0.08)',
  },
}));

const NewContributorsBox = styled('div')({
  marginTop: '1rem',
  padding: '0.85rem 1rem',
  borderRadius: '0.75rem',
  border: '1px solid rgb(var(--oxygen-palette-primary-mainChannel) / 0.2)',
  background: 'rgb(var(--oxygen-palette-primary-mainChannel) / 0.06)',
});

const NewContributorsLabel = styled('div')({
  marginBottom: '0.6rem',
  color: 'var(--ifm-color-primary)',
  fontSize: '0.72rem',
  fontWeight: 700,
  letterSpacing: '0.04em',
  textTransform: 'uppercase',
});

const FullChangelogLink = styled('a')({
  display: 'inline-flex',
  alignItems: 'center',
  gap: '0.5rem',
  marginTop: '2rem',
  padding: '0.6rem 1.1rem',
  borderRadius: '0.6rem',
  border: '1px solid var(--ifm-color-emphasis-200)',
  background: 'var(--oxygen-palette-background-paper)',
  color: 'var(--ifm-color-emphasis-700)',
  fontSize: '0.85rem',
  textDecoration: 'none',
  transition: 'border-color 0.15s, color 0.15s, background 0.15s',
  '&:hover': {
    borderColor: 'rgb(var(--oxygen-palette-primary-mainChannel) / 0.4)',
    background: 'rgb(var(--oxygen-palette-primary-mainChannel) / 0.06)',
    color: 'var(--ifm-font-color-base)',
  },
});

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

function ContributorsPillList({
  contributors,
  isNew = false,
}: {
  contributors: Contributor[];
  isNew?: boolean;
}) {
  return (
    <ContributorsList>
      {contributors.map((contributor) => (
        <ContributorPill key={contributor.username} ownerState={{isNew}} href={contributor.profileUrl} target="_blank" rel="noreferrer">
          <Avatar
            sx={{width: '1.4rem', height: '1.4rem', fontSize: '0.7rem'}}
            alt={contributor.username}
            src={contributor.avatarUrl ?? undefined}
          >
            {contributor.username.charAt(0).toUpperCase()}
          </Avatar>
          <span>{contributor.username}</span>
        </ContributorPill>
      ))}
    </ContributorsList>
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

function ChangeGroup({tone, items}: {items: string[]; tone: ChangeTone}) {
  const filteredItems = items
    .map(normalizeReleaseChangeItem)
    .filter((item) => item && !/made their first contribution/i.test(item));

  if (filteredItems.length === 0) {
    return null;
  }

  return (
    <ChangeGroupSection ownerState={{tone}}>
      <ChangeGroupHeader>
        <ChangeGroupIcon aria-hidden="true">{CHANGE_GROUP_ICONS[tone]}</ChangeGroupIcon>
        <ChangeGroupLabel>{CHANGE_GROUP_LABELS[tone]}</ChangeGroupLabel>
      </ChangeGroupHeader>
      <ChangeGroupList>
        {filteredItems.map((item) => (
          <ChangeGroupListItem key={item}>{item}</ChangeGroupListItem>
        ))}
      </ChangeGroupList>
    </ChangeGroupSection>
  );
}

function ReleaseCard({release}: {release: ReleaseEntry}) {
  const distributionAssets = useMemo(
    () => release.assets.map(getDistributionAsset).filter((asset): asset is DistributionAsset => asset !== null),
    [release.assets],
  );
  const platform = usePlatform();
  const recommendedAssetId = useMemo(
    () =>
      distributionAssets.find((asset) => asset.os === platform?.os && asset.architecture === platform?.arch)?.id ?? null,
    [platform, distributionAssets],
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
  const groupedAssetsByOs = useMemo(() => {
    const osOrder: DistributionAsset['os'][] = ['macos', 'linux', 'win'];

    return osOrder
      .map((os) => ({
        assets: distributionAssets.filter((asset) => asset.os === os),
        os,
      }))
      .filter((group) => group.assets.length > 0);
  }, [distributionAssets]);

  return (
    <ReleaseCardRoot>
      <ReleaseHeader>
        <div>
          <ReleaseTitleRow>
            <ReleaseHeading>{release.tagName}</ReleaseHeading>
            {release.isLatest ? <ReleaseBadge ownerState={{tone: 'latest'}}>Latest release</ReleaseBadge> : null}
            {release.isPrerelease ? <ReleaseBadge ownerState={{tone: 'prerelease'}}>Pre-release</ReleaseBadge> : null}
          </ReleaseTitleRow>
          <ReleaseHeaderDescription>Released on {release.publishedDateLabel}</ReleaseHeaderDescription>
        </div>
      </ReleaseHeader>

      {distributionAssets.length > 0 ? (
        <SectionBlock>
          <h2>Downloads</h2>
          <OtherDownloadsGrid ownerState={{fixedColumns: 3}}>
            {groupedAssetsByOs.map(({assets, os}) => (
              <OtherDownloadsCard key={os}>
                <OtherDownloadsHeader>
                  <OtherDownloadsOsIcon aria-hidden="true">{OS_ICONS[os]}</OtherDownloadsOsIcon>
                  <OtherDownloadsHeaderTitle>{OS_LABELS[os]}</OtherDownloadsHeaderTitle>
                </OtherDownloadsHeader>
                <OtherDownloadsArchitectures>
                  {assets.map((asset) => {
                    const isRecommended = recommendedAssetId === asset.id;

                    return (
                      <OtherDownloadsArchitecture
                        key={asset.id}
                        ownerState={{recommended: isRecommended}}
                        href={asset.downloadUrl}
                        target="_blank"
                        rel="noreferrer"
                      >
                        <OtherDownloadsArchitectureInfo>
                          <OtherDownloadsArchitectureTitle>{asset.architectureLabel}</OtherDownloadsArchitectureTitle>
                          <OtherDownloadsArchitectureMeta>{asset.sizeLabel}</OtherDownloadsArchitectureMeta>
                          {isRecommended ? <OtherDownloadsArchitectureBadge>Recommended for your device</OtherDownloadsArchitectureBadge> : null}
                        </OtherDownloadsArchitectureInfo>
                        <OtherDownloadsActionIcon />
                      </OtherDownloadsArchitecture>
                    );
                  })}
                </OtherDownloadsArchitectures>
              </OtherDownloadsCard>
            ))}
          </OtherDownloadsGrid>
        </SectionBlock>
      ) : null}

      {hasRenderedChanges ? (
        <SectionBlock>
          <h2>What&apos;s Changed</h2>
          <ChangeGrid>
            <ChangeGroup tone="breaking" items={release.changes.breaking} />
            <ChangeGroup tone="features" items={release.changes.features} />
            <ChangeGroup tone="improvements" items={release.changes.improvements} />
            <ChangeGroup tone="bugs" items={release.changes.bugs} />
          </ChangeGrid>
        </SectionBlock>
      ) : null}

      {release.contributors.length > 0 ? (
        <SectionBlock>
          <h2>Contributors ({release.contributors.length})</h2>
          <ContributorsPillList contributors={release.contributors} />

          {release.newContributors.length > 0 ? (
            <NewContributorsBox>
              <NewContributorsLabel>First-time Contributors</NewContributorsLabel>
              <ContributorsPillList contributors={release.newContributors} isNew />
            </NewContributorsBox>
          ) : null}
        </SectionBlock>
      ) : null}

      <FullChangelogLink href={release.htmlUrl} target="_blank" rel="noreferrer">
        <span style={{display: 'inline-flex', alignItems: 'center'}} aria-hidden="true">
          <GithubIcon size={14} />
        </span>
        <span>Full changelog on GitHub</span>
        <svg width="12" height="12" aria-hidden="true" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
          <path d="M7 17L17 7M7 7h10v10" />
        </svg>
      </FullChangelogLink>
    </ReleaseCardRoot>
  );
}

export default function ReleasesPage() {
  const {siteConfig} = useDocusaurusContext();
  const project = siteConfig.customFields?.product as DocusaurusProductConfig | undefined;
  const productName = project?.project?.name ?? siteConfig.title;
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

        const requestedTag = new URLSearchParams(window.location.search).get('tag');
        const requestedRelease = requestedTag
          ? payload.releases.find((release) => release.tagName === requestedTag)
          : undefined;
        setSelectedReleaseId(requestedRelease?.id ?? payload.latestRelease?.id ?? payload.releases[0]?.id ?? null);
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
  const releases = data?.releases ?? [];
  const selectedRelease =
    releases.find((release: ReleaseEntry) => release.id === selectedReleaseId) ?? data?.latestRelease ?? null;
  const topReleases = releases.slice(0, 5);
  const visibleReleases =
    selectedRelease && !topReleases.some((release) => release.id === selectedRelease.id)
      ? [...topReleases, selectedRelease]
      : topReleases;
  const hasMoreReleases = releases.length > topReleases.length;

  return (
    <Layout title="Releases" description={`Explore ${productName} releases, changelogs, and downloads.`}>
      <Shell>
        <Box sx={{pt: {xs: 1, md: 3}, pb: 2, mb: {xs: 5, md: 7}}}>
          <Box
            sx={{
              display: 'inline-flex',
              alignItems: 'center',
              gap: 1,
              mb: 2.5,
              fontFamily: 'monospace',
              fontSize: '10.5px',
              fontWeight: 600,
              letterSpacing: '0.18em',
              textTransform: 'uppercase',
              color: '#8bf9fa',
            }}
          >
            <Box component="span" sx={{width: 5, height: 5, borderRadius: '50%', bgcolor: '#8bf9fa', boxShadow: '0 0 10px #8bf9fa'}} />
            Downloads &amp; Changelog
          </Box>

          <Typography
            variant="h1"
            sx={{
              fontSize: {xs: '2.25rem', sm: '2.75rem', md: '3.5rem'},
              fontWeight: 700,
              letterSpacing: '-0.04em',
              lineHeight: 1.04,
              color: 'text.primary',
              mb: 2.5,
            }}
          >
            Releases
          </Typography>

          <Typography sx={{fontSize: '16.5px', lineHeight: 1.65, color: 'text.secondary', maxWidth: 560, mb: 0}}>
            Every release with detailed changelogs, download options, and the people building {productName}.
          </Typography>
        </Box>

        {error ? <StatusText>{error}</StatusText> : null}
        {!data && !error ? <StatusText>Loading releases...</StatusText> : null}
        {data && !selectedRelease ? <StatusText>No releases are available right now. Please check back soon.</StatusText> : null}

        {selectedRelease ? (
          <LatestSection>
            <VersionSidebar>
              <VersionSidebarLabel>Versions</VersionSidebarLabel>
              <VersionList>
                {visibleReleases.map((release) => (
                  <VersionListItem
                    key={release.id}
                    type="button"
                    ownerState={{active: release.id === selectedRelease.id}}
                    onClick={() => setSelectedReleaseId(release.id)}
                  >
                    <VersionListItemTag>{release.tagName}</VersionListItemTag>
                    {release.isLatest ? <VersionListItemBadge>Latest</VersionListItemBadge> : null}
                  </VersionListItem>
                ))}
              </VersionList>
              {hasMoreReleases ? (
                <VersionSidebarFooter>
                  <VersionSidebarAllLink to="/releases/archive">
                    <span>Browse all {releases.length} releases</span>
                    <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                      <path d="M5 12h14M13 6l6 6-6 6" />
                    </svg>
                  </VersionSidebarAllLink>
                </VersionSidebarFooter>
              ) : null}
            </VersionSidebar>
            <ReleaseCard release={selectedRelease} />
          </LatestSection>
        ) : null}
      </Shell>
    </Layout>
  );
}
