// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import Link from '@docusaurus/Link';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import {Box, Container} from '@wso2/oxygen-ui';
import {ArrowRight, Bug, GitPullRequest, Map as MapIcon, MessageSquare} from '@wso2/oxygen-ui-icons-react';
import type {JSX, ReactNode} from 'react';
import {DISPLAY_FONT, useHomeTokens, HOME_MAX_WIDTH} from './homeTheme';
import {Reveal, Section, SectionHeading} from './shared';
import type {DocusaurusProductConfig} from '@site/docusaurus.product.config';

interface CommunityLink {
  icon: ReactNode;
  title: string;
  description: string;
  href: string;
}

function CommunityCard({link, delay}: {link: CommunityLink; delay: number}): JSX.Element {
  const t = useHomeTokens();
  const external = /^https?:\/\//.test(link.href);

  return (
    <Reveal component="li" delay={delay}>
      <Box
        component={Link}
        to={link.href}
        {...(external ? {target: '_blank', rel: 'noopener noreferrer'} : {})}
        sx={{
          position: 'relative',
          height: '100%',
          minHeight: 128,
          display: 'flex',
          alignItems: 'center',
          gap: {xs: 2, sm: 2.75},
          p: {xs: 2.75, sm: '30px 32px'},
          borderRadius: '20px',
          border: '1px solid',
          borderColor: t.line,
          bgcolor: t.surface,
          backdropFilter: 'blur(14px)',
          color: 'text.primary',
          textDecoration: 'none',
          transition: 'border-color 0.2s, transform 0.2s, background-color 0.2s',
          '&:hover': {
            borderColor: 'rgba(54,136,255,0.55)',
            transform: 'translateY(-2px)',
            bgcolor: t.isDark ? 'rgba(8,32,64,0.7)' : '#fff',
            color: 'text.primary',
            textDecoration: 'none',
          },
          '&:hover .community-go': {opacity: 1, transform: 'translateX(3px)'},
        }}
      >
        <Box
          component="span"
          sx={{
            flex: 'none',
            width: {xs: 48, sm: 56},
            height: {xs: 48, sm: 56},
            borderRadius: '14px',
            display: 'grid',
            placeItems: 'center',
            color: t.cyan,
            bgcolor: 'rgba(0,213,255,0.08)',
            border: '1px solid rgba(0,213,255,0.25)',
          }}
        >
          {link.icon}
        </Box>
        <Box
          component="span"
          sx={{flex: 1, minWidth: 0, pr: 4, fontSize: '1.03rem', lineHeight: 1.55, color: 'text.secondary'}}
        >
          <Box
            component="strong"
            sx={{
              display: 'block',
              mb: 0.5,
              fontFamily: DISPLAY_FONT,
              fontSize: {xs: '1.125rem', sm: '1.25rem'},
              fontWeight: 600,
              letterSpacing: '-0.01em',
              color: 'text.primary',
            }}
          >
            {link.title}
          </Box>
          {link.description}
        </Box>
        <Box
          component="span"
          className="community-go"
          aria-hidden
          sx={{
            position: 'absolute',
            right: 26,
            top: '50%',
            mt: '-10px',
            display: 'flex',
            color: t.link,
            opacity: 0.55,
            transition: 'opacity 0.2s, transform 0.2s',
          }}
        >
          <ArrowRight size={20} strokeWidth={2} />
        </Box>
      </Box>
    </Reveal>
  );
}

export default function CommunitySection(): JSX.Element {
  const {siteConfig} = useDocusaurusContext();
  const product = siteConfig.customFields?.product as DocusaurusProductConfig;
  const productName = product.project.name;
  const {url: githubUrl, discussionsUrl, issuesUrl} = product.project.source.github;

  const links: CommunityLink[] = [
    {
      icon: <GitPullRequest size={26} strokeWidth={1.6} />,
      title: 'Contribute.',
      description: 'Pick up a good first issue, or propose a feature.',
      href: '/community/contributing/propose-a-feature',
    },
    {
      icon: <MessageSquare size={26} strokeWidth={1.6} />,
      title: 'Discuss.',
      description: 'Ask questions and share ideas on GitHub Discussions.',
      href: discussionsUrl,
    },
    {
      icon: <Bug size={26} strokeWidth={1.6} />,
      title: 'Report an issue.',
      description: 'Help find bugs and suggest improvements.',
      href: issuesUrl,
    },
    {
      icon: <MapIcon size={26} strokeWidth={1.6} />,
      title: 'Follow the roadmap.',
      description: "See what's planned and weigh in.",
      href: `${githubUrl}/milestones`,
    },
  ];

  return (
    <Section id="community">
      <Container maxWidth={false} sx={{maxWidth: HOME_MAX_WIDTH, px: {xs: 2, sm: 4}}}>
        <SectionHeading
          eyebrow="Get involved"
          title={`Join the ${productName} community`}
          description={`${productName} is built in the open, with an ever-growing community of contributors. Shape where it goes next.`}
        />
        <Box
          component="ul"
          sx={{
            listStyle: 'none',
            m: 0,
            p: 0,
            display: 'grid',
            gridTemplateColumns: {xs: '1fr', md: 'repeat(2, 1fr)'},
            gap: 2,
          }}
        >
          {links.map((l, i) => (
            <CommunityCard key={l.title} link={l} delay={i * 80} />
          ))}
        </Box>
      </Container>
    </Section>
  );
}
