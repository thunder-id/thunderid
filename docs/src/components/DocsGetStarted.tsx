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

import React, {JSX} from 'react';
import {Box, Typography, useTheme} from '@wso2/oxygen-ui';
import Link from '@docusaurus/Link';
import useScrollAnimation from '../hooks/useScrollAnimation';
import UseCaseBranchCards from './UseCaseBranchCards';

// ─── Step cards ──────────────────────────────────────────────────────────────

const STEP_CARDS = [
  {
    number: '01',
    title: 'Run ThunderID',
    description: 'Start ThunderID locally with Docker or download the release artifact.',
    href: '/docs/next/guides/getting-started/get-thunderid',
    icon: (
      <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round">
        <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/>
        <polyline points="7 10 12 15 17 10"/>
        <line x1="12" x2="12" y1="15" y2="3"/>
      </svg>
    ),
  },
  {
    number: '02',
    title: 'Register an application',
    description: 'Create an application in the Console and get your client credentials.',
    href: '/docs/next/guides/getting-started/register-an-application',
    icon: (
      <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round">
        <rect width="7" height="7" x="3" y="3" rx="1"/>
        <rect width="7" height="7" x="14" y="3" rx="1"/>
        <rect width="7" height="7" x="14" y="14" rx="1"/>
        <rect width="7" height="7" x="3" y="14" rx="1"/>
      </svg>
    ),
  },
  {
    number: '03',
    title: 'Build a sign-in flow',
    description: 'Use the visual flow designer to configure how users authenticate.',
    href: '/docs/next/guides/guides/flows/build-a-flow',
    icon: (
      <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round">
        <circle cx="18" cy="18" r="3"/>
        <circle cx="6" cy="6" r="3"/>
        <path d="M13 6h3a2 2 0 0 1 2 2v7"/>
        <path d="M11 18H8a2 2 0 0 1-2-2V9"/>
      </svg>
    ),
  },
  {
    number: '04',
    title: 'Connect your app',
    description: 'Add sign-in to a React app with the Asgardeo SDK in a few lines of code.',
    href: '/docs/next/guides/quick-start/connect-your-application/react',
    icon: (
      <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round">
        <polyline points="16 18 22 12 16 6"/><polyline points="8 6 2 12 8 18"/>
      </svg>
    ),
  },
];

interface StepItemProps {
  step: (typeof STEP_CARDS)[number];
  index: number;
  isVisible: boolean;
}

function StepItem({step, index, isVisible}: StepItemProps): JSX.Element {
  const theme = useTheme();
  return (
    <Box
      sx={{
        display: 'flex',
        flexDirection: 'column',
        gap: 1.25,
        p: 2.5,
        borderRadius: '12px',
        border: '1px solid',
        borderColor: 'divider',
        bgcolor: `rgba(${theme.vars?.palette.primary.main} / 0.02)`,
        opacity: isVisible ? 1 : 0,
        transform: isVisible ? 'translateY(0)' : 'translateY(16px)',
        transitionProperty: 'opacity, transform',
        transitionDuration: '0.45s',
        transitionTimingFunction: 'cubic-bezier(0.16, 1, 0.3, 1)',
        transitionDelay: isVisible ? `${index * 0.07}s` : '0s',
      }}
    >
      <Box sx={{display: 'flex', alignItems: 'center', justifyContent: 'space-between'}}>
        <Box
          sx={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            width: 36,
            height: 36,
            borderRadius: '8px',
            bgcolor: `rgba(${theme.vars?.palette.primary.main} / 0.1)`,
            color: 'primary.main',
          }}
        >
          {step.icon}
        </Box>
        <Typography
          component="span"
          sx={{
            fontFamily: 'monospace',
            fontSize: '1.1rem',
            fontWeight: 700,
            color: 'text.disabled',
            letterSpacing: '0.02em',
          }}
        >
          {step.number}
        </Typography>
      </Box>
      <Typography variant="body2" sx={{fontWeight: 700, fontSize: '0.9rem', color: 'text.primary'}}>
        {step.title}
      </Typography>
      <Typography variant="body2" sx={{fontSize: '0.8rem', lineHeight: 1.55, color: 'text.secondary'}}>
        {step.description}
      </Typography>
    </Box>
  );
}

function QuickstartPanel({isVisible}: {isVisible: boolean}): JSX.Element {
  const theme = useTheme();
  return (
    <Box
      sx={{
        px: {xs: 3, md: 4},
        pt: 0,
        pb: {xs: 2.5, md: 3},
        borderRadius: '16px',
        border: '1px solid',
        borderColor: 'divider',
        bgcolor: `rgba(${theme.vars?.palette.primary.main} / 0.02)`,
      }}
    >
      <Typography
        component="h2"
        variant="h4"
        sx={{fontWeight: 800, mb: 0.25, mt: 0, fontSize: {xs: '1.4rem', md: '1.6rem'}, color: 'text.primary', letterSpacing: '-0.01em'}}
      >
        New to ThunderID?
      </Typography>
      <Typography
        variant="body1"
        sx={{fontWeight: 500, mb: 2.5, fontSize: '0.95rem', color: 'text.secondary'}}
      >
        Follow the step-by-step guide to go from zero to your first working integration.
      </Typography>
      <Box
        sx={{
          display: 'grid',
          gridTemplateColumns: {xs: '1fr', sm: 'repeat(2, 1fr)', lg: 'repeat(4, 1fr)'},
          gap: {xs: 2, lg: 2.5},
        }}
      >
        {STEP_CARDS.map((step, index) => (
          <StepItem key={step.number} step={step} index={index} isVisible={isVisible} />
        ))}
      </Box>
      <Box sx={{mt: 2}}>
        <Box
          component={Link}
          to="/docs/next/guides/getting-started/get-thunderid"
          sx={{
            display: 'inline-flex',
            alignItems: 'center',
            gap: 0.75,
            px: 3,
            py: 1.25,
            borderRadius: '8px',
            background: `linear-gradient(135deg, ${theme.vars?.palette.primary.dark} 0%, ${theme.vars?.palette.primary.main} 100%)`,
            color: '#ffffff !important',
            fontWeight: 700,
            fontSize: '0.925rem',
            textDecoration: 'none !important',
            transition: 'transform 0.2s, box-shadow 0.2s',
            '&:hover': {
              transform: 'translateY(-1px)',
              boxShadow: `0 4px 16px rgba(${theme.vars?.palette.primary.main} / 0.35)`,
              textDecoration: 'none !important',
            },
          }}
        >
          Get started →
        </Box>
      </Box>

      {/* MCP callout strip */}
      <Box
        sx={{
          mt: 2.5,
          display: 'flex',
          flexDirection: {xs: 'column', sm: 'row'},
          alignItems: {xs: 'flex-start', sm: 'center'},
          gap: 2,
          px: 2.5,
          py: 1.75,
          borderRadius: '12px',
          border: '1px solid',
          borderColor: 'rgba(123,92,246,0.25)',
          bgcolor: 'rgba(123,92,246,0.08)',
        }}
      >
        <Box
          component="span"
          sx={{
            display: 'flex',
            alignItems: 'center',
            flexShrink: 0,
            color: '#7B5CF6',
            animation: 'ai-sparkle 2.5s ease-in-out infinite',
          }}
        >
          <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
            <path d="m12 3-1.912 5.813a2 2 0 0 1-1.275 1.275L3 12l5.813 1.912a2 2 0 0 1 1.275 1.275L12 21l1.912-5.813a2 2 0 0 1 1.275-1.275L21 12l-5.813-1.912a2 2 0 0 1-1.275-1.275L12 3Z"/>
            <path d="M5 3v4"/><path d="M19 17v4"/><path d="M3 5h4"/><path d="M17 19h4"/>
          </svg>
        </Box>
        <Typography
          variant="body2"
          sx={{flex: 1, fontSize: '0.9rem', color: 'text.secondary', lineHeight: 1.6}}
        >
          <strong>Want a quicker setup?</strong> Use the ThunderID MCP server to create apps, configure flows, and connect SDKs from your editor.
        </Typography>
        <Box
          component={Link}
          to="/docs/next/guides/working-with-ai/mcp-server"
          sx={{
            flexShrink: 0,
            display: 'inline-flex',
            alignItems: 'center',
            px: 2,
            py: 0.75,
            borderRadius: '7px',
            bgcolor: 'rgba(123,92,246,0.15)',
            border: '1px solid rgba(123,92,246,0.35)',
            color: '#7B5CF6 !important',
            fontWeight: 700,
            fontSize: '0.825rem',
            textDecoration: 'none !important',
            whiteSpace: 'nowrap',
            width: {xs: '100%', sm: 'auto'},
            justifyContent: {xs: 'center', sm: 'flex-start'},
            transition: 'background-color 0.15s, border-color 0.15s',
            '&:hover': {
              bgcolor: 'rgba(123,92,246,0.25)',
              borderColor: 'rgba(123,92,246,0.55)',
              textDecoration: 'none !important',
            },
          }}
        >
          Try MCP server →
        </Box>
      </Box>
    </Box>
  );
}

// ─── Use-case cards ───────────────────────────────────────────────────────────

const USE_CASE_CARDS = [
  {
    icon: (
      <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round">
        <rect width="20" height="16" x="2" y="4" rx="2"/>
        <path d="M10 4v4"/><path d="M2 8h20"/><path d="M6 4v4"/>
      </svg>
    ),
    label: 'Secure an application',
    description: 'Add sign-in to a web, mobile, or single-page app. Create an application, configure redirect URIs, and build a sign-in flow with OAuth 2.0 or OIDC.',
    href: '/docs/next/guides/guides/applications/manage-applications',
  },
  {
    icon: (
      <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round">
        <rect width="20" height="8" x="2" y="2" rx="2"/>
        <rect width="20" height="8" x="2" y="14" rx="2"/>
        <line x1="6" x2="6.01" y1="6" y2="6"/>
        <line x1="6" x2="6.01" y1="18" y2="18"/>
      </svg>
    ),
    label: 'Protect an API',
    description: 'Register a resource server, define granular scopes, and validate access tokens issued by ThunderID in your API or microservice.',
    href: '/docs/next/guides/guides/resource-servers',
  },
  {
    icon: (
      <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round">
        <path d="M6 22V4a2 2 0 0 1 2-2h8a2 2 0 0 1 2 2v18Z"/>
        <path d="M6 12H4a2 2 0 0 0-2 2v6a2 2 0 0 0 2 2h2"/>
        <path d="M18 9h2a2 2 0 0 1 2 2v9a2 2 0 0 1-2 2h-2"/>
        <path d="M10 6h4"/><path d="M10 10h4"/><path d="M10 14h4"/><path d="M10 18h4"/>
      </svg>
    ),
    label: 'Build B2B SaaS',
    description: 'Create organization units for each customer, configure per-tenant identity providers, and delegate admin access to your customers.',
    href: '/docs/next/use-cases/b2b/multi-tenant-saas',
  },
  {
    icon: (
      <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round">
        <path d="M12 8V4H8"/>
        <rect width="16" height="12" x="4" y="8" rx="2"/>
        <path d="M2 14h2"/><path d="M20 14h2"/><path d="M15 13v2"/><path d="M9 13v2"/>
      </svg>
    ),
    label: 'Connect AI agents',
    description: 'Secure MCP servers and issue delegated access tokens so AI agents can call APIs and act on behalf of users in autonomous workflows.',
    href: '/docs/next/use-cases/ai-agents/agent-authentication',
  },
];

interface UseCaseCardProps {
  card: (typeof USE_CASE_CARDS)[number];
  index: number;
  isVisible: boolean;
}

function UseCaseCard({card, index, isVisible}: UseCaseCardProps): JSX.Element {
  const theme = useTheme();
  return (
    <Box
      component={Link}
      to={card.href}
      sx={{
        display: 'flex',
        flexDirection: 'column',
        gap: 2,
        p: 3,
        borderRadius: '14px',
        textDecoration: 'none !important',
        border: '1px solid',
        borderColor: 'divider',
        bgcolor: `rgba(${theme.vars?.palette.primary.main} / 0.02)`,
        color: 'inherit',
        opacity: isVisible ? 1 : 0,
        transform: isVisible ? 'translateY(0)' : 'translateY(24px)',
        transitionProperty: 'opacity, transform, border-color, box-shadow',
        transitionDuration: '0.5s, 0.5s, 0.2s, 0.2s',
        transitionTimingFunction: 'cubic-bezier(0.16, 1, 0.3, 1)',
        transitionDelay: isVisible ? `${index * 0.07}s` : '0s',
        '&:hover': {
          borderColor: `rgba(${theme.vars?.palette.primary.main} / 0.4)`,
          boxShadow: `0 4px 16px rgba(${theme.vars?.palette.primary.main} / 0.08)`,
          textDecoration: 'none !important',
        },
      }}
    >
      <Box
        sx={{
          p: 1,
          borderRadius: '8px',
          bgcolor: `rgba(${theme.vars?.palette.primary.main} / 0.1)`,
          color: 'primary.main',
          display: 'inline-flex',
        }}
      >
        {card.icon}
      </Box>
      <Box sx={{flex: 1}}>
        <Typography variant="h6" sx={{fontWeight: 700, fontSize: '1rem', mb: 0.75, color: 'text.primary'}}>
          {card.label}
        </Typography>
        <Typography variant="body2" sx={{fontSize: '0.875rem', lineHeight: 1.6, color: 'text.secondary'}}>
          {card.description}
        </Typography>
      </Box>
    </Box>
  );
}

// ─── Browse by topic ─────────────────────────────────────────────────────────

const BROWSE_TOPICS = [
  {
    label: 'Guides',
    description: 'Step-by-step how-to guides',
    href: '/docs/next/guides/getting-started/what-is-thunderid',
    icon: (
      <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        <path d="M2 3h6a4 4 0 0 1 4 4v14a3 3 0 0 0-3-3H2z"/><path d="M22 3h-6a4 4 0 0 0-4 4v14a3 3 0 0 1 3-3h7z"/>
      </svg>
    ),
  },
  {
    label: 'Deployment',
    description: 'Run ThunderID in production',
    href: '/docs/next/guides/getting-started/get-thunderid',
    icon: (
      <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        <path d="M4 14.899A7 7 0 1 1 15.71 8h1.79a4.5 4.5 0 0 1 2.5 8.242"/>
        <path d="M12 12v9"/><path d="m8 17 4-5 4 5"/>
      </svg>
    ),
  },
  {
    label: 'APIs',
    description: 'Full REST API reference',
    href: '/docs/next/apis',
    icon: (
      <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        <polyline points="16 18 22 12 16 6"/><polyline points="8 6 2 12 8 18"/>
      </svg>
    ),
  },
  {
    label: 'SDKs',
    description: 'Client libraries and integrations',
    href: '/docs/next/sdks/overview',
    icon: (
      <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        <path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"/>
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
        <Typography component="h1" variant="h1" sx={{fontWeight: 800, mb: 1, color: 'text.primary', letterSpacing: '-0.02em', fontSize: {xs: '2.4rem', sm: '3rem'}}}>
          ThunderID Docs
        </Typography>
        <Typography variant="body1" sx={{color: 'text.secondary', fontSize: '1rem', whiteSpace: 'normal', overflowWrap: 'anywhere'}}>
          Learn how to add sign-in, secure APIs, manage organizations, and connect AI agents with ThunderID.
        </Typography>
      </Box>

      {/* Quickstart panel */}
      <Box ref={quickstartRef}>
        <QuickstartPanel isVisible={quickstartVisible} />
      </Box>

      {/* Use cases */}
      <Box>
        <Typography component="h2" variant="h5" sx={{fontWeight: 700, mb: 2.5, fontSize: '1.2rem', color: 'text.primary'}}>
          Know your use case?
        </Typography>
        <UseCaseBranchCards />
      </Box>

      {/* Browse by topic */}
      <BrowseTopics />

    </Box>
  );
}
