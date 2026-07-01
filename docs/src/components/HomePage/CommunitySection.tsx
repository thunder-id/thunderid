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

import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import {Box, Card, Container, Typography, useTheme} from '@wso2/oxygen-ui';
import {MessagesSquareIcon} from '@wso2/oxygen-ui-icons-react';
import {JSX} from 'react';
import useIsDarkMode from '../../hooks/useIsDarkMode';
import useScrollAnimation from '../../hooks/useScrollAnimation';
import type {DocusaurusProductConfig} from '@site/docusaurus.product.config';

interface CommunityCardProps {
  icon: JSX.Element;
  iconBg: string;
  title: string;
  description: string;
  linkLabel: string;
  href: string;
}

function CommunityCard({icon, iconBg, title, description, linkLabel, href}: CommunityCardProps) {
  const isDark = useIsDarkMode();
  const theme = useTheme();

  return (
    <Card
      sx={{
        flex: 1,
        p: {xs: 3, sm: 4},
        pt: {xs: 4, sm: 5},
        pb: {xs: 3, sm: 4},
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        textAlign: 'center',
        cursor: 'pointer',
        transition: 'all 0.3s cubic-bezier(0.16, 1, 0.3, 1)',
        bgcolor: isDark ? 'rgba(255, 255, 255, 0.03)' : 'rgba(0, 0, 0, 0.02)',
        backdropFilter: 'blur(12px)',
        border: '1px solid',
        borderColor: 'divider',
        borderRadius: '16px',
        position: 'relative',
        overflow: 'hidden',
        '&::before': {
          content: '""',
          position: 'absolute',
          top: 0,
          left: 0,
          right: 0,
          height: '2px',
          background: `linear-gradient(90deg, ${theme.vars?.palette.primary.dark} 0%, ${theme.vars?.palette.primary.main} 100%)`,
          opacity: 0,
          transition: 'opacity 0.3s ease',
        },
        '&:hover::before': {opacity: 1},
        '&:hover': {
          transform: 'translateY(-6px)',
          boxShadow: isDark
            ? `0 16px 40px rgba(0, 0, 0, 0.5), 0 0 0 1px rgba(${theme.vars?.palette.primary.main} / 0.2)`
            : `0 16px 40px rgba(0, 0, 0, 0.12), 0 0 0 1px rgba(${theme.vars?.palette.primary.main} / 0.15)`,
          borderColor: `rgba(${theme.vars?.palette.primary.main} / 0.3)`,
          bgcolor: isDark ? 'rgba(255, 255, 255, 0.045)' : 'rgba(0, 0, 0, 0.025)',
        },
      }}
      onClick={() => window.open(href, '_blank', 'noopener noreferrer')}
    >
      <Box
        sx={{
          width: 60,
          height: 60,
          borderRadius: '16px',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          background: iconBg,
          color: 'common.white',
          mb: 3,
          boxShadow: '0 8px 24px rgba(0,0,0,0.2)',
          transition: 'transform 0.3s ease, box-shadow 0.3s ease',
          '.MuiCard-root:hover &': {
            transform: 'scale(1.1)',
            boxShadow: '0 12px 32px rgba(0,0,0,0.3)',
          },
        }}
      >
        {icon}
      </Box>
      <Typography variant="h6" sx={{fontWeight: 600, mb: 1, color: 'text.primary', fontSize: '1.1rem'}}>
        {title}
      </Typography>
      <Typography variant="body2" sx={{mb: 3, color: 'text.secondary', lineHeight: 1.7, fontSize: '0.9rem'}}>
        {description}
      </Typography>
      <Typography
        variant="body2"
        sx={{
          mt: 'auto',
          color: 'primary.main',
          fontWeight: 500,
          fontSize: '0.9rem',
          transition: 'color 0.2s ease',
          '&:hover': {color: 'primary.dark'},
        }}
      >
        {linkLabel} &rarr;
      </Typography>
    </Card>
  );
}

function GitForkIcon() {
  return (
    <svg
      width="26"
      height="26"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <circle cx="12" cy="18" r="3" />
      <circle cx="6" cy="6" r="3" />
      <circle cx="18" cy="6" r="3" />
      <path d="M18 9v1a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2V9" />
      <path d="M12 12v3" />
    </svg>
  );
}

function IssueIcon() {
  return (
    <svg
      width="26"
      height="26"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <circle cx="12" cy="12" r="10" />
      <line x1="12" y1="8" x2="12" y2="12" />
      <line x1="12" y1="16" x2="12.01" y2="16" />
    </svg>
  );
}

export default function CommunitySection(): JSX.Element {
  const theme = useTheme();
  const {ref, isVisible} = useScrollAnimation({threshold: 0.15});
  const {siteConfig} = useDocusaurusContext();
  const productName = (siteConfig.customFields?.product as DocusaurusProductConfig).project.name;
  const discussionsUrl = (siteConfig.customFields?.product as DocusaurusProductConfig).project.source.github
    .discussionsUrl;
  const issuesUrl = (siteConfig.customFields?.product as DocusaurusProductConfig).project.source.github.issuesUrl;

  return (
    <Box component="section" sx={{py: {xs: 8, lg: 12}, borderTop: '1px solid', borderColor: 'divider'}}>
      <Container maxWidth="lg" sx={{px: {xs: 2, sm: 4}}}>
        <Box
          ref={ref}
          sx={{
            display: 'flex',
            flexDirection: 'column',
            alignItems: 'center',
            textAlign: 'center',
            opacity: isVisible ? 1 : 0,
            transform: isVisible ? 'translateY(0)' : 'translateY(32px)',
            transition: 'opacity 0.7s cubic-bezier(0.16, 1, 0.3, 1), transform 0.7s cubic-bezier(0.16, 1, 0.3, 1)',
          }}
        >
          <Typography
            variant="h3"
            sx={{
              mb: 2,
              fontSize: {xs: '1.75rem', sm: '2.25rem', md: '2.5rem'},
              fontWeight: 700,
              color: 'text.primary',
            }}
          >
            Join the {productName}{' '}
            <Box
              component="span"
              sx={{
                background: `linear-gradient(90deg, ${theme.vars?.palette.primary.dark} 0%, ${theme.vars?.palette.primary.main} 100%)`,
                WebkitBackgroundClip: 'text',
                WebkitTextFillColor: 'transparent',
                backgroundClip: 'text',
              }}
            >
              community
            </Box>
          </Typography>
          <Typography
            variant="body1"
            sx={{
              mb: 6,
              fontSize: {xs: '0.95rem', sm: '1.05rem'},
              color: 'text.secondary',
              lineHeight: 1.7,
              maxWidth: '600px',
            }}
          >
            We&apos;re building {productName} with you. Engage with our ever-growing community to get the latest
            updates, product support, and more.
          </Typography>

          <Box
            sx={{
              display: 'grid',
              gridTemplateColumns: {xs: '1fr', sm: 'repeat(2, 1fr)', md: 'repeat(3, 1fr)'},
              width: '100%',
              maxWidth: 900,
              gap: 3,
            }}
          >
            <CommunityCard
              icon={<GitForkIcon />}
              iconBg="rgba(59,130,246,0.10)"
              title="Contribute"
              description={`Help shape ${productName} by submitting features, fixes, or improvements.`}
              linkLabel="Start Contributing"
              href="/docs/next/community/contributing/contribute-ideas"
            />
            <CommunityCard
              icon={<IssueIcon />}
              iconBg="rgba(59,130,246,0.10)"
              title="Report issues"
              description={`Identify bugs and suggest enhancements to make ${productName} better for everyone.`}
              linkLabel="Open an Issue"
              href={issuesUrl}
            />
            <CommunityCard
              icon={<MessagesSquareIcon />}
              iconBg="rgba(59,130,246,0.10)"
              title="Join the Discussions"
              description="Ask questions, share ideas, and connect with the community through GitHub Discussions"
              linkLabel="Open Discussions"
              href={discussionsUrl}
            />
          </Box>
        </Box>
      </Container>
    </Box>
  );
}
