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
import UseCaseBranchCards from './UseCaseBranchCards';
import useScrollAnimation from '../hooks/useScrollAnimation';

// ─── Capability cards ─────────────────────────────────────────────────────────

const CAPABILITIES = [
  {
    title: 'Works with your stack',
    description: 'Native SDKs for React, Vue, Next.js, Node.js, iOS, Android, Flutter, and more.',
    icon: (
      <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round">
        <polyline points="16 18 22 12 16 6" />
        <polyline points="8 6 2 12 8 18" />
      </svg>
    ),
  },
  {
    title: 'Visual flow designer',
    description: 'Build and iterate on sign-in flows without touching app code. Add MFA, passkeys, or social login from the Console.',
    icon: (
      <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round">
        <circle cx="18" cy="18" r="3" />
        <circle cx="6" cy="6" r="3" />
        <path d="M13 6h3a2 2 0 0 1 2 2v7" />
        <path d="M11 18H8a2 2 0 0 1-2-2V9" />
      </svg>
    ),
  },
  {
    title: 'Full identity stack',
    description: 'Users, organizations, SSO, MFA, social login, and API security — all in one lightweight, self-hostable server.',
    icon: (
      <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round">
        <rect width="18" height="11" x="3" y="11" rx="2" ry="2" />
        <path d="M7 11V7a5 5 0 0 1 10 0v4" />
      </svg>
    ),
  },
];

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
      <Typography variant="body1" sx={{fontWeight: 500, mb: 2.5, fontSize: '0.95rem', color: 'text.secondary'}}>
        ThunderID is a self-hostable identity server. Add sign-in, manage users and organizations, and secure APIs — without building auth from scratch.
      </Typography>
      <Box
        sx={{
          display: 'grid',
          gridTemplateColumns: {xs: '1fr', sm: 'repeat(3, 1fr)'},
          gap: {xs: 1.5, md: 2},
        }}
      >
        {CAPABILITIES.map(({title, description, icon}, index) => (
          <Box
            key={title}
            sx={{
              display: 'flex',
              flexDirection: 'column',
              gap: 1,
              p: 2.5,
              borderRadius: '12px',
              border: '1px solid',
              borderColor: 'divider',
              opacity: isVisible ? 1 : 0,
              transform: isVisible ? 'translateY(0)' : 'translateY(16px)',
              transitionProperty: 'opacity, transform',
              transitionDuration: '0.45s',
              transitionTimingFunction: 'cubic-bezier(0.16, 1, 0.3, 1)',
              transitionDelay: isVisible ? `${index * 0.08}s` : '0s',
            }}
          >
            <Box
              sx={{
                display: 'flex', alignItems: 'center', justifyContent: 'center',
                width: 36, height: 36, borderRadius: '8px',
                bgcolor: `rgba(${theme.vars?.palette.primary.main} / 0.1)`,
                color: 'primary.main', flexShrink: 0,
              }}
            >
              {icon}
            </Box>
            <Typography variant="body2" sx={{fontWeight: 700, fontSize: '0.9rem', color: 'text.primary'}}>
              {title}
            </Typography>
            <Typography variant="body2" sx={{fontSize: '0.8rem', lineHeight: 1.55, color: 'text.secondary'}}>
              {description}
            </Typography>
          </Box>
        ))}
      </Box>
      <Box sx={{mt: 2.5}}>
        <Box
          component={Link}
          to="/docs/next/guides/getting-started/get-thunderid"
          sx={{
            display: 'inline-flex', alignItems: 'center', gap: 0.75,
            px: 3, py: 1.25, borderRadius: '8px',
            background: `linear-gradient(135deg, ${theme.vars?.palette.primary.dark} 0%, ${theme.vars?.palette.primary.main} 100%)`,
            color: '#ffffff !important', fontWeight: 700, fontSize: '0.925rem',
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
          <svg
            width="24"
            height="24"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="2"
            strokeLinecap="round"
            strokeLinejoin="round"
            aria-hidden="true"
          >
            <path d="m12 3-1.912 5.813a2 2 0 0 1-1.275 1.275L3 12l5.813 1.912a2 2 0 0 1 1.275 1.275L12 21l1.912-5.813a2 2 0 0 1 1.275-1.275L21 12l-5.813-1.912a2 2 0 0 1-1.275-1.275L12 3Z" />
            <path d="M5 3v4" />
            <path d="M19 17v4" />
            <path d="M3 5h4" />
            <path d="M17 19h4" />
          </svg>
        </Box>
        <Typography variant="body2" sx={{flex: 1, fontSize: '0.9rem', color: 'text.secondary', lineHeight: 1.6}}>
          <strong>Want a quicker setup?</strong> Use the ThunderID MCP server to create apps, configure flows, and
          connect SDKs from your editor.
        </Typography>
        <Box
          component={Link}
          to="/docs/next/guides/working-with-ai/get-started-with-mcp"
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

// ─── Browse by topic ─────────────────────────────────────────────────────────

const BROWSE_TOPICS = [
  {
    label: 'Guides',
    description: 'Step-by-step how-to guides',
    href: '/docs/next/guides/getting-started/what-is-thunderid',
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
    href: '/docs/next/sdks/overview',
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
