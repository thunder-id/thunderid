// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import {Box, Container, Typography} from '@wso2/oxygen-ui';
import {AppWindow, GitPullRequest, Heart, Package, Puzzle, ShieldCheck} from '@wso2/oxygen-ui-icons-react';
import type {JSX, ReactNode} from 'react';
import {ACCENTS, type AccentKey, cardSx, DISPLAY_FONT, useHomeTokens, HOME_MAX_WIDTH} from './homeTheme';
import {Reveal, Section, SectionHeading, TextLink} from './shared';
import type {DocusaurusProductConfig} from '@site/docusaurus.product.config';
import {useDocsUrl} from '@site/src/hooks/useDocsUrl';

interface UseCaseItem {
  lead?: string;
  text: string;
  link: {label: string; href: string};
}

interface UseCase {
  accent: AccentKey;
  icon: ReactNode;
  badge: ReactNode;
  title: string;
  subtitle: string;
  items: UseCaseItem[];
}

function UseCaseCard({useCase, delay}: {useCase: UseCase; delay: number}): JSX.Element {
  const t = useHomeTokens();
  const c = ACCENTS[useCase.accent];

  return (
    <Reveal
      component="article"
      delay={delay}
      sx={[cardSx(t), {p: {xs: 3, md: 3.75}, '&:hover': {transform: 'translateY(-4px)', bgcolor: t.surfaceStrong}}]}
    >
      <Box sx={{display: 'flex', alignItems: 'center', gap: 2.5}}>
        <Box
          component="span"
          aria-hidden
          sx={{
            position: 'relative',
            flex: 'none',
            width: 60,
            height: 60,
            borderRadius: '16px',
            display: 'grid',
            placeItems: 'center',
            color: c,
            background: `linear-gradient(145deg, color-mix(in srgb, ${c} 18%, transparent), color-mix(in srgb, ${c} 5%, transparent))`,
            border: `1px solid color-mix(in srgb, ${c} 35%, transparent)`,
          }}
        >
          {useCase.icon}
          <Box
            component="span"
            sx={{
              position: 'absolute',
              right: -7,
              bottom: -7,
              width: 26,
              height: 26,
              borderRadius: '50%',
              display: 'grid',
              placeItems: 'center',
              color: c,
              bgcolor: t.isDark ? '#0a1628' : '#fff',
              border: `1px solid color-mix(in srgb, ${c} 55%, transparent)`,
            }}
          >
            {useCase.badge}
          </Box>
        </Box>
        <Box>
          <Typography
            component="h3"
            sx={{m: 0, fontFamily: DISPLAY_FONT, fontSize: '1.3125rem', fontWeight: 600, letterSpacing: '-0.01em'}}
          >
            {useCase.title}
          </Typography>
          <Typography sx={{mt: 0.5, fontSize: '0.9375rem', color: 'text.secondary'}}>{useCase.subtitle}</Typography>
        </Box>
      </Box>

      <Box sx={{display: 'grid', gridTemplateColumns: {xs: '1fr', md: 'repeat(2, minmax(0, 1fr))'}, columnGap: 5}}>
        {useCase.items.map((item) => (
          <Box key={item.link.label} sx={{mt: 3, pt: 2.75, borderTop: '1px solid', borderColor: t.line}}>
            <Typography sx={{fontSize: '0.97rem', lineHeight: 1.6, color: 'text.secondary', maxWidth: '52ch'}}>
              {item.lead && (
                <Box component="strong" sx={{color: 'text.primary', fontWeight: 600}}>
                  {item.lead}{' '}
                </Box>
              )}
              {item.text}
            </Typography>
            <TextLink href={item.link.href} sx={{mt: 1.25, fontSize: '0.9rem'}}>
              {item.link.label}
            </TextLink>
          </Box>
        ))}
      </Box>
    </Reveal>
  );
}

export default function UseCasesSection(): JSX.Element {
  const docsUrl = useDocsUrl();
  const {siteConfig} = useDocusaurusContext();
  const productName = (siteConfig.customFields?.product as DocusaurusProductConfig).project.name;

  const useCases: UseCase[] = [
    {
      accent: 'electric',
      icon: <AppWindow size={28} strokeWidth={1.6} />,
      badge: <ShieldCheck size={13} strokeWidth={2} />,
      title: 'Secure your applications',
      subtitle: 'Add identity to the apps you already build.',
      items: [
        {
          text: 'Add sign-in, authorization, and agent identity to your app through the auth libraries and frameworks you already use, starting with Better Auth. No new SDK to learn.',
          link: {label: 'Framework guides', href: '/sdks-and-tools'},
        },
        {
          text: `Give your users an open source identity provider option. ${productName} integrates through standard OpenID Connect, OAuth 2.1, SAML, and SCIM, with ready-to-use deployment artifacts.`,
          link: {label: 'Integration guides', href: docsUrl('/docs/next/guides/protocols')},
        },
      ],
    },
    {
      accent: 'violet',
      icon: <Package size={28} strokeWidth={1.6} />,
      badge: <Puzzle size={13} strokeWidth={2} />,
      title: 'Ship it, build on it',
      subtitle: `Make ${productName} part of your own product.`,
      items: [
        {
          text: `Embed ${productName} as the identity layer of your product or distribution, so your users don't depend on an external identity provider.`,
          link: {label: 'Embedding guide', href: docsUrl('/docs/next/deployment/deployment-paths')},
        },
        {
          lead: 'Build on it.',
          text: 'Create your own specialized identity solution on an extensible core. Extend flows, authenticators, and attribute sources in Go, with full source-level control.',
          link: {label: 'Extension docs', href: docsUrl('/docs/next/guides/flows/what-are-flows')},
        },
      ],
    },
    {
      accent: 'cyan',
      icon: <GitPullRequest size={28} strokeWidth={1.6} />,
      badge: <Heart size={13} strokeWidth={2} />,
      title: 'Contribute back',
      subtitle: `Help shape where ${productName} goes next.`,
      items: [
        {
          lead: 'Improve the code.',
          text: 'Fix bugs, add features, or propose a change through GitHub issues and discussions, so what you need ships for everyone. Start with a good first issue.',
          link: {label: 'Contributing guide', href: '/community/contributing/propose-a-feature'},
        },
        {
          lead: 'Share what you know.',
          text: 'Write docs, samples, tutorials, or blog posts, or publish a connector or framework integration so others can build on your work.',
          link: {label: 'Ways to contribute', href: '/community/overview'},
        },
      ],
    },
  ];

  return (
    <Section id="use">
      <Container maxWidth={false} sx={{maxWidth: HOME_MAX_WIDTH, px: {xs: 2, sm: 4}}}>
        <SectionHeading
          eyebrow={`How you use ${productName}`}
          title="Start where you are"
          description={`Use ${productName} through the tools you already know, offer it to your users, ship it inside your product, build on it, or contribute back.`}
        />
        <Box sx={{display: 'flex', flexDirection: 'column', gap: 2}}>
          {useCases.map((u) => (
            <UseCaseCard key={u.title} useCase={u} delay={0} />
          ))}
        </Box>
      </Container>
    </Section>
  );
}
