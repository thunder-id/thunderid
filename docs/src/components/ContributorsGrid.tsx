// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useBaseUrlUtils} from '@docusaurus/useBaseUrl';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import {Box, Button, Typography} from '@wso2/oxygen-ui';
import {ArrowRight, Search} from '@wso2/oxygen-ui-icons-react';
import React, {type ChangeEvent, useEffect, useMemo, useState} from 'react';
import type {DocusaurusProductConfig} from '@site/docusaurus.product.config';
import useIsDarkMode from '@site/src/hooks/useIsDarkMode';

interface Contributor {
  avatarUrl: string;
  contributions: number;
  htmlUrl: string;
  login: string;
}

interface ContributorsData {
  contributors: Contributor[];
  generatedAt: string;
  totalCommits: number;
  totalContributors: number;
}

type SortKey = 'commits' | 'name';

const AVATAR_SIZE = 60;
const PAGE_SIZE = 100;
const SORTS: {key: SortKey; label: string}[] = [
  {key: 'commits', label: 'Commits'},
  {key: 'name', label: 'Name'},
];

export default function ContributorsGrid(): React.ReactElement | null {
  const {withBaseUrl} = useBaseUrlUtils();
  const {siteConfig} = useDocusaurusContext();
  const config = siteConfig.customFields?.product as DocusaurusProductConfig;
  const repoUrl = config?.project?.source?.github?.url ?? 'https://github.com/thunder-id/thunderid';
  const issuesUrl = config?.project?.source?.github?.issuesUrl ?? `${repoUrl}/issues`;
  const goodFirstIssueUrl = `${issuesUrl}?q=${encodeURIComponent('is:issue state:open label:"good first issue"')}`;

  const isLight = !useIsDarkMode();
  const ink = (light: number, dark: number): string => (isLight ? `rgba(0,0,0,${light})` : `rgba(255,255,255,${dark})`);

  const [data, setData] = useState<ContributorsData | null>(null);
  const [query, setQuery] = useState('');
  const [sort, setSort] = useState<SortKey>('commits');
  const [visibleCount, setVisibleCount] = useState(PAGE_SIZE);

  useEffect(() => {
    fetch(withBaseUrl('/data/contributors.json'))
      .then((r) => r.json())
      .then((d: ContributorsData) => setData(d))
      .catch(() => undefined);
  }, [withBaseUrl]);

  const contributors = useMemo(() => {
    if (!data) {
      return [];
    }
    const q = query.trim().toLowerCase();
    const list = q ? data.contributors.filter((c) => c.login.toLowerCase().includes(q)) : data.contributors.slice();
    if (sort === 'name') {
      list.sort((a, b) => a.login.toLowerCase().localeCompare(b.login.toLowerCase()));
    }
    return list;
  }, [data, query, sort]);

  const visibleContributors = contributors.slice(0, visibleCount);
  const remaining = contributors.length - visibleContributors.length;

  if (!data) {
    return null;
  }

  const statusLabel = query.trim()
    ? `${contributors.length} of ${data.totalContributors} shown`
    : `${data.totalContributors} contributors`;

  return (
    <Box>
      {/* Stats row */}
      <Box sx={{display: 'flex', flexWrap: 'wrap', alignItems: 'baseline', gap: 4, mb: 5}}>
        <Box sx={{display: 'flex', alignItems: 'baseline', gap: 1}}>
          <Typography
            component="span"
            sx={{fontSize: '2.125rem', fontWeight: 700, letterSpacing: '-0.03em', lineHeight: 1}}
          >
            {data.totalContributors}
          </Typography>
          <Typography component="span" sx={{fontSize: '0.84rem', color: 'text.secondary'}}>
            contributors
          </Typography>
        </Box>
        <Box sx={{width: '1px', height: 26, bgcolor: 'divider'}} />
        <Box sx={{display: 'flex', alignItems: 'baseline', gap: 1}}>
          <Typography
            component="span"
            sx={{fontSize: '2.125rem', fontWeight: 700, letterSpacing: '-0.03em', lineHeight: 1}}
          >
            {data.totalCommits.toLocaleString()}
          </Typography>
          <Typography component="span" sx={{fontSize: '0.84rem', color: 'text.secondary'}}>
            commits
          </Typography>
        </Box>
      </Box>

      {/* Filter + sort controls */}
      <Box sx={{display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: 1.5, mb: 2.75}}>
        <Box sx={{position: 'relative', flex: '1 1 220px', maxWidth: 340}}>
          <Box
            sx={{
              position: 'absolute',
              left: 13,
              top: '50%',
              transform: 'translateY(-50%)',
              display: 'inline-flex',
              color: ink(0.35, 0.35),
              pointerEvents: 'none',
            }}
          >
            <Search size={14} />
          </Box>
          <Box
            component="input"
            type="text"
            value={query}
            aria-label="Filter contributors"
            placeholder="Filter contributors"
            onChange={(e: ChangeEvent<HTMLInputElement>) => {
              setQuery(e.target.value);
              setVisibleCount(PAGE_SIZE);
            }}
            sx={{
              width: '100%',
              fontFamily: 'inherit',
              fontSize: '13.5px',
              color: 'text.primary',
              bgcolor: ink(0.02, 0.035),
              border: '1px solid',
              borderColor: ink(0.09, 0.09),
              borderRadius: '9px',
              py: '9px',
              pl: '34px',
              pr: '13px',
              outline: 'none',
              transition: 'border-color 0.18s, background 0.18s',
              '&:focus': {borderColor: 'rgba(54,136,255,0.5)'},
              '&::placeholder': {color: ink(0.4, 0.4)},
            }}
          />
        </Box>

        <Box
          sx={{
            display: 'flex',
            gap: 0.5,
            p: '3px',
            borderRadius: '9px',
            bgcolor: ink(0.02, 0.035),
            border: '1px solid',
            borderColor: ink(0.09, 0.09),
          }}
        >
          {SORTS.map(({key, label}) => (
            <Box
              key={key}
              component="button"
              type="button"
              aria-pressed={sort === key}
              onClick={() => {
                setSort(key);
                setVisibleCount(PAGE_SIZE);
              }}
              sx={{
                px: 1.75,
                py: 0.75,
                borderRadius: '7px',
                border: 0,
                cursor: 'pointer',
                fontFamily: 'inherit',
                fontSize: '12.5px',
                fontWeight: 600,
                bgcolor: sort === key ? 'rgba(54,136,255,0.16)' : 'transparent',
                color: sort === key ? 'primary.main' : 'text.secondary',
              }}
            >
              {label}
            </Box>
          ))}
        </Box>

        <Box sx={{flex: 1}} />

        <Typography
          component="span"
          sx={{fontFamily: 'monospace', fontSize: '11.5px', color: ink(0.45, 0.45), whiteSpace: 'nowrap'}}
        >
          {statusLabel}
        </Typography>
      </Box>

      {/* Avatar wall */}
      {contributors.length > 0 ? (
        <Box
          sx={{
            display: 'grid',
            justifyContent: 'start',
            gap: '14px',
            gridTemplateColumns: `repeat(auto-fill, ${AVATAR_SIZE}px)`,
          }}
        >
          {visibleContributors.map((contributor) => (
            <Box
              key={contributor.login}
              component="a"
              href={contributor.htmlUrl}
              target="_blank"
              rel="noopener noreferrer"
              title={`${contributor.login} — ${contributor.contributions} commits`}
              sx={{
                position: 'relative',
                display: 'block',
                width: AVATAR_SIZE,
                aspectRatio: '1',
                borderRadius: '50%',
                transition: 'transform 0.18s cubic-bezier(.2,.8,.3,1)',
                '&:hover': {transform: 'scale(1.14)', zIndex: 5},
                '&:hover .contributor-ring': {boxShadow: '0 0 0 2px rgba(54,136,255,0.9)'},
              }}
            >
              <Box
                component="img"
                src={contributor.avatarUrl}
                alt={contributor.login}
                loading="lazy"
                sx={{
                  width: '100%',
                  height: '100%',
                  borderRadius: '50%',
                  objectFit: 'cover',
                  display: 'block',
                  border: '1px solid',
                  borderColor: ink(0.12, 0.12),
                  boxShadow: '0 2px 10px rgba(0,0,0,0.25)',
                }}
              />
              <Box
                className="contributor-ring"
                sx={{
                  position: 'absolute',
                  inset: 0,
                  borderRadius: '50%',
                  boxShadow: '0 0 0 0 rgba(54,136,255,0)',
                  transition: 'box-shadow 0.18s ease',
                }}
              />
            </Box>
          ))}
        </Box>
      ) : (
        <Typography sx={{fontSize: '13.5px', color: 'text.secondary', py: 4}}>
          No contributors match &ldquo;{query}&rdquo;.
        </Typography>
      )}

      {/* Load more */}
      {remaining > 0 && (
        <Box sx={{mt: 2.5, display: 'flex', justifyContent: 'center'}}>
          <Box
            component="button"
            type="button"
            onClick={() => setVisibleCount((count) => count + PAGE_SIZE)}
            sx={{
              px: 2.25,
              py: 1,
              borderRadius: '9px',
              border: '1px solid',
              borderColor: ink(0.09, 0.09),
              bgcolor: ink(0.02, 0.035),
              color: 'text.secondary',
              cursor: 'pointer',
              fontFamily: 'inherit',
              fontSize: '12.5px',
              fontWeight: 600,
              transition: 'border-color 0.18s, color 0.18s',
              '&:hover': {borderColor: 'rgba(54,136,255,0.5)', color: 'primary.main'},
            }}
          >
            Load {Math.min(PAGE_SIZE, remaining)} more · {remaining} left
          </Box>
        </Box>
      )}

      {/* Call to action */}
      <Box
        sx={{
          mt: 5.5,
          display: 'flex',
          flexWrap: 'wrap',
          alignItems: 'center',
          justifyContent: 'space-between',
          gap: 2.25,
          px: {xs: 2.5, sm: 3.25},
          py: 2.75,
          borderRadius: '14px',
          border: '1px solid rgba(54,136,255,0.2)',
          background: isLight
            ? 'linear-gradient(120deg, rgba(54,136,255,0.06), rgba(123,92,255,0.04))'
            : 'linear-gradient(120deg, rgba(76,141,255,0.1), rgba(123,92,255,0.06))',
        }}
      >
        <Box>
          <Typography sx={{fontSize: '16px', fontWeight: 600, color: 'text.primary'}}>
            Your name belongs here.
          </Typography>
          <Typography sx={{fontSize: '13.5px', color: 'text.secondary', mt: 0.5}}>
            Good first issues are labelled and waiting. Docs fixes count too.
          </Typography>
        </Box>
        <Button
          variant="contained"
          color="primary"
          href={goodFirstIssueUrl}
          target="_blank"
          rel="noopener noreferrer"
          endIcon={<ArrowRight size={14} strokeWidth={2.4} />}
        >
          Start contributing
        </Button>
      </Box>
    </Box>
  );
}
