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
import {styled, Typography} from '@wso2/oxygen-ui';
import React, {useEffect, useState} from 'react';

interface ArchiveRelease {
  htmlUrl: string;
  id: number;
  isLatest: boolean;
  isPrerelease: boolean;
  publishedDateLabel: string;
  tagName: string;
}

interface ArchiveData {
  releases: ArchiveRelease[];
}

const RELEASES_UNAVAILABLE_MESSAGE = 'Release information is currently unavailable. Please check back soon.';

const Shell = styled('main')({
  minWidth: 0,
  overflowX: 'hidden',
  width: '100%',
  maxWidth: '900px',
  margin: '0 auto',
  padding: '4rem 1.5rem 5rem',
  '@media (max-width: 996px)': {
    padding: '3rem 1rem 4rem',
  },
});

const BackLink = styled(Link)({
  display: 'inline-flex',
  alignItems: 'center',
  gap: '0.35rem',
  marginBottom: '1.5rem',
  color: 'var(--ifm-color-emphasis-600)',
  fontSize: '0.85rem',
  textDecoration: 'none',
  '&:hover': {
    color: 'var(--ifm-font-color-base)',
    textDecoration: 'none',
  },
});

const HeroDescription = styled('p')({
  marginTop: '1rem',
  marginBottom: '3rem',
  maxWidth: '560px',
  fontSize: '16.5px',
  lineHeight: 1.65,
  color: 'var(--ifm-color-emphasis-700)',
});

const StatusText = styled('p')({
  margin: '0 0 2rem',
  color: 'var(--ifm-color-emphasis-700)',
});

const ReleaseTable = styled('div')({
  display: 'flex',
  flexDirection: 'column',
  borderRadius: '12px',
  border: '1px solid var(--ifm-color-emphasis-200)',
  overflow: 'hidden',
  '[data-theme="dark"] &': {
    borderColor: 'rgba(255, 255, 255, 0.09)',
  },
});

const ReleaseRow = styled(Link)({
  display: 'flex',
  alignItems: 'center',
  gap: '1rem',
  padding: '0.95rem 1.25rem',
  color: 'inherit',
  textDecoration: 'none',
  borderBottom: '1px solid var(--ifm-color-emphasis-200)',
  transition: 'background 0.15s ease',
  '&:last-child': {
    borderBottom: 'none',
  },
  '[data-theme="dark"] &': {
    borderBottomColor: 'rgba(255, 255, 255, 0.07)',
  },
  '&:hover': {
    textDecoration: 'none',
    background: 'var(--ifm-hover-overlay)',
  },
  '[data-theme="dark"] &:hover': {
    background: 'rgba(255, 255, 255, 0.04)',
  },
});

const ReleaseTag = styled('span')({
  fontFamily: 'var(--ifm-font-family-monospace, monospace)',
  fontSize: '0.95rem',
  fontWeight: 600,
  color: 'var(--ifm-color-emphasis-900)',
});

const ReleaseBadge = styled('span')<{ownerState: {tone: 'latest' | 'prerelease'}}>(({ownerState}) => ({
  display: 'inline-flex',
  alignItems: 'center',
  padding: '0.1rem 0.55rem',
  borderRadius: '999px',
  fontSize: '0.66rem',
  fontWeight: 700,
  letterSpacing: '0.02em',
  ...(ownerState.tone === 'latest'
    ? {
        background: 'var(--ifm-color-primary)',
        color: '#fff',
      }
    : {
        border: '1px solid color-mix(in srgb, var(--ifm-color-warning) 40%, transparent)',
        background: 'color-mix(in srgb, var(--ifm-color-warning) 15%, transparent)',
        color: 'var(--ifm-color-warning)',
      }),
}));

const ReleaseDate = styled('span')({
  marginLeft: 'auto',
  color: 'var(--ifm-color-emphasis-600)',
  fontSize: '0.82rem',
  whiteSpace: 'nowrap',
});

const ReleaseChevron = styled('span')({
  display: 'inline-flex',
  color: 'var(--ifm-color-emphasis-400)',
});

export default function ReleaseArchivePage(): React.ReactElement {
  const {siteConfig} = useDocusaurusContext();
  const productName = siteConfig.title;
  const {withBaseUrl} = useBaseUrlUtils();
  const [releases, setReleases] = useState<ArchiveRelease[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const controller = new AbortController();

    async function loadReleases() {
      try {
        const response = await fetch(withBaseUrl('/data/releases.json'), {signal: controller.signal});

        if (!response.ok) {
          throw new Error(RELEASES_UNAVAILABLE_MESSAGE);
        }

        const payload = JSON.parse(await response.text()) as ArchiveData;

        if (!payload || typeof payload !== 'object' || !Array.isArray(payload.releases)) {
          throw new Error(RELEASES_UNAVAILABLE_MESSAGE);
        }

        setError(null);
        setReleases(payload.releases);
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

  return (
    <Layout title="Release Archive" description={`Browse every ${productName} release.`}>
      <Shell>
        <BackLink to="/docs/next/releases">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <path d="M19 12H5M11 18l-6-6 6-6" />
          </svg>
          <span>Releases</span>
        </BackLink>

        <Typography
          variant="h1"
          sx={{
            fontSize: {xs: '2.25rem', sm: '2.75rem', md: '3.5rem'},
            fontWeight: 700,
            letterSpacing: '-0.04em',
            lineHeight: 1.04,
            color: 'text.primary',
            margin: 0,
          }}
        >
          Release Archive
        </Typography>
        <HeroDescription>Every {productName} release, newest first.</HeroDescription>

        {error ? <StatusText>{error}</StatusText> : null}
        {!releases && !error ? <StatusText>Loading releases...</StatusText> : null}

        {releases && releases.length > 0 ? (
          <ReleaseTable>
            {releases.map((release) => (
              <ReleaseRow key={release.id} to={`/docs/next/releases?tag=${encodeURIComponent(release.tagName)}`}>
                <ReleaseTag>{release.tagName}</ReleaseTag>
                {release.isLatest ? <ReleaseBadge ownerState={{tone: 'latest'}}>Latest</ReleaseBadge> : null}
                {release.isPrerelease ? <ReleaseBadge ownerState={{tone: 'prerelease'}}>Pre-release</ReleaseBadge> : null}
                <ReleaseDate>{release.publishedDateLabel}</ReleaseDate>
                <ReleaseChevron aria-hidden="true">
                  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                    <path d="M9 6l6 6-6 6" />
                  </svg>
                </ReleaseChevron>
              </ReleaseRow>
            ))}
          </ReleaseTable>
        ) : null}
      </Shell>
    </Layout>
  );
}
