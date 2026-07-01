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
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import {Box, Card, CardContent, Chip, Divider, Typography} from '@wso2/oxygen-ui';
import {AlertCircle, ArrowRight, Code2, MessageSquare} from '@wso2/oxygen-ui-icons-react';
import React, {type ReactNode} from 'react';
import ContributorCloud from './ContributorCloud';
import type {DocusaurusProductConfig} from '@site/docusaurus.product.config';

interface InvolvementCard {
  description: string;
  href: string;
  icon: ReactNode;
  label: string;
  title: string;
}

function IconBox({children}: {children: ReactNode}) {
  return (
    <Box
      sx={{
        width: 44,
        height: 44,
        borderRadius: 2.5,
        bgcolor: (theme) =>
          theme.palette.mode === 'dark'
            ? 'rgba(54,136,255,0.12)'
            : 'rgba(54,136,255,0.08)',
        border: '1px solid',
        borderColor: (theme) =>
          theme.palette.mode === 'dark'
            ? 'rgba(54,136,255,0.22)'
            : 'rgba(54,136,255,0.18)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        color: 'primary.main',
        flexShrink: 0,
        mb: 2,
      }}
    >
      {children}
    </Box>
  );
}

function InvolvementCard({card}: {card: InvolvementCard}) {
  return (
    <Card
      component={Link}
      href={card.href}
      target={card.href.startsWith('http') ? '_blank' : undefined}
      rel={card.href.startsWith('http') ? 'noopener noreferrer' : undefined}
      sx={{
        height: '100%',
        display: 'flex',
        flexDirection: 'column',
        border: '1px solid',
        borderColor: 'divider',
        textDecoration: 'none',
        transition: 'border-color 0.2s, transform 0.2s, box-shadow 0.2s',
        '&:hover': {
          borderColor: 'primary.main',
          transform: 'translateY(-3px)',
          boxShadow: 3,
          textDecoration: 'none',
        },
      }}
    >
      <CardContent sx={{p: 3, flex: 1, display: 'flex', flexDirection: 'column'}}>
        <IconBox>{card.icon}</IconBox>
        <Typography variant="h6" sx={{fontWeight: 700, mb: 0.75, letterSpacing: '-0.01em'}}>
          {card.title}
        </Typography>
        <Typography variant="body2" sx={{color: 'text.secondary', lineHeight: 1.65, flex: 1}}>
          {card.description}
        </Typography>
        <Box
          sx={{
            display: 'inline-flex',
            alignItems: 'center',
            gap: 0.5,
            mt: 2,
            color: 'primary.main',
            fontSize: '0.875rem',
            fontWeight: 500,
          }}
        >
          {card.label}
          <ArrowRight size={14} />
        </Box>
      </CardContent>
    </Card>
  );
}

export default function CommunityOverview(): React.ReactElement {
  const {siteConfig} = useDocusaurusContext();
  const config = siteConfig.customFields?.product as DocusaurusProductConfig;
  const productName = config?.project?.name ?? 'ThunderID';
  const repoUrl = config?.project?.source?.github?.url ?? 'https://github.com/thunder-id/thunderid';

  const cards: InvolvementCard[] = [
    {
      description: 'Found a bug? Report it on GitHub so the team can track and fix it.',
      href: '../contributing/report-a-bug',
      icon: <AlertCircle size={20} />,
      label: 'Report a Bug',
      title: 'Report a Bug',
    },
    {
      description: 'Have an idea for a new feature? Share it with the community.',
      href: '../contributing/contribute-ideas',
      icon: <MessageSquare size={20} />,
      label: 'Submit an Idea',
      title: 'Contribute Ideas',
    },
    {
      description: 'Ready to write code? Follow the contributor guide to submit a pull request.',
      href: '../contributing/contributing-code/prerequisites',
      icon: <Code2 size={20} />,
      label: 'Contribution Guide',
      title: 'Contribute Code',
    },
  ];

  return (
    <Box sx={{maxWidth: 860}}>
      {/* Badge */}
      <Box
        sx={{
          display: 'inline-flex',
          alignItems: 'center',
          gap: 1,
          px: 0.5,
          py: 0.5,
          pr: 1.5,
          border: '1px solid',
          borderColor: (theme) =>
            theme.palette.mode === 'dark'
              ? 'rgba(54,136,255,0.22)'
              : 'rgba(54,136,255,0.3)',
          borderRadius: 999,
          bgcolor: (theme) =>
            theme.palette.mode === 'dark'
              ? 'rgba(54,136,255,0.08)'
              : 'rgba(54,136,255,0.05)',
          mb: 3,
        }}
      >
        <Chip
          label="OPEN SOURCE"
          size="small"
          sx={{
            background: 'linear-gradient(135deg, #1d5eb4, #3688ff)',
            color: '#fff',
            fontWeight: 600,
            fontSize: '0.65rem',
            letterSpacing: '0.04em',
            height: 22,
          }}
        />
        <Typography
          variant="caption"
          sx={{
            fontFamily: 'monospace',
            color: 'text.secondary',
            fontSize: '0.75rem',
          }}
        >
          community
        </Typography>
      </Box>

      {/* Hero */}
      <Typography
        variant="h1"
        sx={{fontWeight: 700, letterSpacing: '-0.033em', lineHeight: 1.05, mb: 2}}
      >
        Join the Community
      </Typography>
      <Typography
        variant="body1"
        sx={{color: 'text.secondary', lineHeight: 1.72, maxWidth: 560, mb: 5}}
      >
        {productName} is an open-source project that welcomes contributions from everyone. Whether you
        want to report a bug, propose a feature, improve docs, or contribute code — you&apos;re in
        the right place.
      </Typography>

      {/* Get Involved */}
      <Typography variant="h5" sx={{fontWeight: 700, mb: 2.5, letterSpacing: '-0.015em'}}>
        Get Involved
      </Typography>
      <Box
        sx={{
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fill, minmax(220px, 1fr))',
          gap: 2,
          mb: 7,
        }}
      >
        {cards.map((card) => (
          <InvolvementCard card={card} key={card.title} />
        ))}
      </Box>

      {/* Contributors */}
      <Box sx={{mb: 6}}>
        <Box
          sx={{
            display: 'flex',
            alignItems: 'flex-end',
            justifyContent: 'space-between',
            mb: 2,
            flexWrap: 'wrap',
            gap: 1,
          }}
        >
          <Box>
            <Typography variant="h5" sx={{fontWeight: 700, letterSpacing: '-0.015em', mb: 0.25}}>
              Contributors
            </Typography>
            <Typography variant="body2" sx={{color: 'text.secondary'}}>
              The people building {productName}
            </Typography>
          </Box>
          <Link
            href="../contributors"
            style={{
              fontSize: '0.875rem',
              fontWeight: 500,
              display: 'inline-flex',
              alignItems: 'center',
              gap: 4,
            }}
          >
            View all <ArrowRight size={14} />
          </Link>
        </Box>
        <ContributorCloud />
      </Box>

      <Divider sx={{mb: 4.5}} />

      {/* License */}
      <Box>
        <Typography variant="subtitle1" sx={{fontWeight: 700, mb: 1.5, letterSpacing: '-0.01em'}}>
          License
        </Typography>
        <Typography variant="body2" sx={{color: 'text.secondary', lineHeight: 1.65, mb: 1.5}}>
          {productName} is licensed under the Apache License 2.0 — free to use, modify, and
          distribute.
        </Typography>
        <Box sx={{display: 'inline-flex', alignItems: 'center', gap: 1}}>
          <Chip
            label="Apache 2.0"
            size="small"
            sx={{
              fontFamily: 'monospace',
              fontSize: '0.72rem',
              bgcolor: (theme) =>
                theme.palette.mode === 'dark'
                  ? 'rgba(54,136,255,0.1)'
                  : 'rgba(54,136,255,0.08)',
              color: 'primary.main',
              border: '1px solid',
              borderColor: (theme) =>
                theme.palette.mode === 'dark'
                  ? 'rgba(54,136,255,0.25)'
                  : 'rgba(54,136,255,0.2)',
            }}
          />
          <Link
            href={`${repoUrl}/blob/main/LICENSE`}
            target="_blank"
            rel="noopener noreferrer"
            style={{fontSize: '0.875rem', fontWeight: 500}}
          >
            View License →
          </Link>
        </Box>
      </Box>
    </Box>
  );
}
