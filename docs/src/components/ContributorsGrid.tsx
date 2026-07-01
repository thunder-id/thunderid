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
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import {Avatar, Box, Card, Divider, Typography} from '@wso2/oxygen-ui';
import {ExternalLink} from '@wso2/oxygen-ui-icons-react';
import React, {useEffect, useState} from 'react';
import type {DocusaurusProductConfig} from '@site/docusaurus.product.config';

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

const STRIPE_COLORS = ['#3688ff', '#8b5cf6', '#10b981', '#f59e0b', '#ef4444', '#ec4899', '#06b6d4', '#84cc16'];

function ContributorCard({contributor, index}: {contributor: Contributor; index: number}) {
  return (
    <Card
      component="a"
      href={contributor.htmlUrl}
      target="_blank"
      rel="noopener noreferrer"
      sx={{
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        gap: 1,
        p: 2.5,
        pt: 3,
        border: '1px solid',
        borderColor: 'divider',
        borderRadius: 3,
        textDecoration: 'none',
        cursor: 'pointer',
        position: 'relative',
        overflow: 'hidden',
        transition: 'border-color 0.2s, box-shadow 0.2s, transform 0.2s',
        '&:hover': {
          borderColor: 'primary.main',
          transform: 'translateY(-4px)',
          boxShadow: 4,
        },
      }}
    >
      {/* Coloured top stripe */}
      <Box
        sx={{
          position: 'absolute',
          top: 0,
          left: 0,
          right: 0,
          height: 3,
          bgcolor: STRIPE_COLORS[index % STRIPE_COLORS.length],
          opacity: 0.3,
        }}
      />

      <Avatar
        src={contributor.avatarUrl}
        alt={contributor.login}
        sx={{width: 52, height: 52, border: '2px solid', borderColor: 'divider'}}
      />

      <Box sx={{textAlign: 'center'}}>
        <Typography
          variant="body2"
          sx={{fontWeight: 600, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis', maxWidth: 100}}
        >
          {contributor.login}
        </Typography>
        <Typography variant="caption" sx={{color: 'text.secondary', fontFamily: 'monospace'}}>
          {contributor.contributions} commits
        </Typography>
      </Box>
    </Card>
  );
}

export default function ContributorsGrid(): React.ReactElement | null {
  const {withBaseUrl} = useBaseUrlUtils();
  const {siteConfig} = useDocusaurusContext();
  const config = siteConfig.customFields?.product as DocusaurusProductConfig;
  const repoUrl = config?.project?.source?.github?.url ?? 'https://github.com/thunder-id/thunderid';
  const [data, setData] = useState<ContributorsData | null>(null);

  useEffect(() => {
    fetch(withBaseUrl('/data/contributors.json'))
      .then((r) => r.json())
      .then((d: ContributorsData) => setData(d))
      .catch(() => undefined);
  }, [withBaseUrl]);

  if (!data) {
    return null;
  }

  return (
    <Box>
      {/* Stats row */}
      <Box sx={{display: 'flex', flexWrap: 'wrap', gap: 3.5, mb: 5, alignItems: 'center'}}>
        <Box>
          <Typography variant="h4" sx={{fontWeight: 700, letterSpacing: '-0.03em', lineHeight: 1}}>
            {data.totalContributors}
          </Typography>
          <Typography variant="caption" sx={{color: 'text.secondary', fontFamily: 'monospace'}}>
            contributors
          </Typography>
        </Box>

        <Divider orientation="vertical" flexItem />

        <Box>
          <Typography variant="h4" sx={{fontWeight: 700, letterSpacing: '-0.03em', lineHeight: 1}}>
            {data.totalCommits.toLocaleString()}
          </Typography>
          <Typography variant="caption" sx={{color: 'text.secondary', fontFamily: 'monospace'}}>
            total commits
          </Typography>
        </Box>

        <Divider orientation="vertical" flexItem />

        <Box>
          <Typography
            variant="h4"
            sx={{fontWeight: 700, letterSpacing: '-0.03em', lineHeight: 1, color: 'primary.main'}}
          >
            Apache 2.0
          </Typography>
          <Typography variant="caption" sx={{color: 'text.secondary', fontFamily: 'monospace'}}>
            open source license
          </Typography>
        </Box>
      </Box>

      {/* Contributors grid */}
      <Box
        sx={{
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fill, minmax(130px, 1fr))',
          gap: 1.75,
          mb: 3,
        }}
      >
        {data.contributors.map((contributor, index) => (
          <ContributorCard contributor={contributor} index={index} key={contributor.login} />
        ))}
      </Box>

      {/* View all link */}
      <Box
        component="a"
        href={`${repoUrl}/graphs/contributors`}
        target="_blank"
        rel="noopener noreferrer"
        sx={{
          display: 'inline-flex',
          alignItems: 'center',
          gap: 0.75,
          color: 'text.secondary',
          textDecoration: 'none',
          fontSize: '0.875rem',
          transition: 'color 0.15s',
          '&:hover': {color: 'primary.main'},
        }}
      >
        View all contributors on GitHub
        <ExternalLink size={14} />
      </Box>
    </Box>
  );
}
