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
import {Box, Typography, useTheme} from '@wso2/oxygen-ui';
import {JSX} from 'react';
import DeveloperShortcut from './DeveloperShortcut';
import UseCaseBranchCards from './UseCaseBranchCards';
import useScrollAnimation from '../hooks/useScrollAnimation';

function QuickstartPanel({isVisible}: {isVisible: boolean}): JSX.Element {
  return (
    <Box
      sx={{
        opacity: isVisible ? 1 : 0,
        transform: isVisible ? 'translateY(0)' : 'translateY(16px)',
        transitionProperty: 'opacity, transform',
        transitionDuration: '0.45s',
        transitionTimingFunction: 'cubic-bezier(0.16, 1, 0.3, 1)',
      }}
    >
      {/* Developer shortcut — primary path, with install OR section */}
      <DeveloperShortcut showInstallPath />

    </Box>
  );
}

// ─── Browse by topic ─────────────────────────────────────────────────────────

const BROWSE_TOPICS = [
  {
    label: 'Guides',
    description: 'Step-by-step how-to guides',
    href: '/docs/next/guides/getting-started/get-thunderid',
    icon: (
      <svg
        width="18"
        height="18"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
      >
        <path d="M2 3h6a4 4 0 0 1 4 4v14a3 3 0 0 0-3-3H2z" />
        <path d="M22 3h-6a4 4 0 0 0-4 4v14a3 3 0 0 1 3-3h7z" />
      </svg>
    ),
  },
  {
    label: 'Deployment',
    description: 'Run ThunderID in production',
    href: '/docs/next/guides/deployment-patterns',
    icon: (
      <svg
        width="18"
        height="18"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
      >
        <path d="M4 14.899A7 7 0 1 1 15.71 8h1.79a4.5 4.5 0 0 1 2.5 8.242" />
        <path d="M12 12v9" />
        <path d="m8 17 4-5 4 5" />
      </svg>
    ),
  },
  {
    label: 'APIs',
    description: 'Full REST API reference',
    href: '/docs/next/apis',
    icon: (
      <svg
        width="18"
        height="18"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
      >
        <polyline points="16 18 22 12 16 6" />
        <polyline points="8 6 2 12 8 18" />
      </svg>
    ),
  },
  {
    label: 'SDKs',
    description: 'Client libraries and integrations',
    href: '/sdks',
    icon: (
      <svg
        width="18"
        height="18"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
      >
        <path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z" />
      </svg>
    ),
  },
];

function BrowseTopics(): JSX.Element {
  const theme = useTheme();
  return (
    <Box>
      <Typography component="h2" variant="h5" sx={{fontWeight: 700, mb: 2, fontSize: '1.2rem', color: 'text.primary'}}>
        Explore the platform
      </Typography>
      <Box sx={{display: 'grid', gridTemplateColumns: {xs: '1fr 1fr', md: 'repeat(4, 1fr)'}, gap: 1.5}}>
        {BROWSE_TOPICS.map((topic) => (
          <Box
            key={topic.label}
            component={Link}
            to={topic.href}
            sx={{
              display: 'flex',
              flexDirection: 'column',
              gap: 1,
              px: 2,
              py: 1.75,
              borderRadius: '10px',
              textDecoration: 'none !important',
              fontSize: '0.875rem',
              fontWeight: 500,
              color: 'text.secondary',
              border: '1px solid',
              borderColor: 'divider',
              transition: 'color 0.15s, border-color 0.15s, background-color 0.15s',
              '&:hover': {
                color: 'primary.main',
                borderColor: `rgba(${theme.vars?.palette.primary.main} / 0.35)`,
                bgcolor: `rgba(${theme.vars?.palette.primary.main} / 0.04)`,
                textDecoration: 'none !important',
              },
            }}
          >
            <Box sx={{display: 'flex', alignItems: 'center', gap: 1}}>
              <Box component="span" sx={{color: 'primary.main', display: 'flex', alignItems: 'center', flexShrink: 0}}>
                {topic.icon}
              </Box>
              <Typography component="span" sx={{fontWeight: 700, fontSize: '0.9rem', color: 'text.primary'}}>
                {topic.label}
              </Typography>
            </Box>
            <Typography variant="body2" sx={{fontSize: '0.78rem', lineHeight: 1.4, color: 'text.secondary'}}>
              {topic.description}
            </Typography>
          </Box>
        ))}
      </Box>
    </Box>
  );
}

// ─── Main export ──────────────────────────────────────────────────────────────

export default function DocsGetStarted(): JSX.Element {
  const {ref: quickstartRef, isVisible: quickstartVisible} = useScrollAnimation({threshold: 0.05});

  return (
    <Box className="docs-home-page" sx={{display: 'flex', flexDirection: 'column', gap: 2}}>
      {/* Page title */}
      <Box sx={{pt: 2, pb: 1}}>
        <Typography
          component="h1"
          variant="h1"
          sx={{
            fontWeight: 800,
            mb: 1,
            color: 'text.primary',
            letterSpacing: '-0.02em',
            fontSize: {xs: '2.4rem', sm: '3rem'},
          }}
        >
          ThunderID Docs
        </Typography>
        <Typography
          variant="body1"
          sx={{color: 'text.secondary', fontSize: '1rem', whiteSpace: 'normal', overflowWrap: 'anywhere'}}
        >
          Learn how to add sign-in, secure APIs, manage organizations, and connect AI agents with ThunderID.
        </Typography>
      </Box>

      {/* Quickstart panel */}
      <Box ref={quickstartRef}>
        <QuickstartPanel isVisible={quickstartVisible} />
      </Box>

      {/* Use cases */}
      <Box>
        <Typography
          component="h2"
          variant="h5"
          sx={{fontWeight: 700, mb: 2.5, fontSize: '1.2rem', color: 'text.primary'}}
        >
          Know your use case?
        </Typography>
        <UseCaseBranchCards />
      </Box>

      {/* Browse by topic */}
      <BrowseTopics />
    </Box>
  );
}
