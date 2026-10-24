// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Container, Typography} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {cardSx, DISPLAY_FONT, useHomeTokens, HOME_MAX_WIDTH} from './homeTheme';
import {MaskLogo, Reveal, Section, SectionHeading} from './shared';

const FOUNDATIONS = [
  {
    category: 'Code',
    title: 'Open source',
    description:
      'Apache 2.0 licensed. Use it, ship it inside your product, and modify it, with no proprietary licensing to pass on.',
    logo: {name: 'osi', width: 64, height: 64, label: 'Open Source Initiative'},
  },
  {
    category: 'Governance',
    title: 'Open governance',
    description:
      'An LF Decentralized Trust project. Design decisions, RFCs, and the roadmap happen in public on GitHub.',
    logo: {name: 'lfdt-full', width: 'min(300px, 80%)', height: 17, label: 'LF Decentralized Trust'},
  },
  {
    category: 'Standards',
    title: 'Open standards',
    description:
      'Built on OAuth 2.1, OpenID Connect, SAML, SCIM, and AuthZEN, and on OpenID4VCI and OpenID4VP for wallets.',
    logo: {name: 'oauth', width: 64, height: 64, label: 'OAuth'},
  },
];

export default function OpenFoundationsSection(): JSX.Element {
  const t = useHomeTokens();

  return (
    <Section id="open">
      <Container maxWidth={false} sx={{maxWidth: HOME_MAX_WIDTH, px: {xs: 2, sm: 4}}}>
        <SectionHeading eyebrow="Open foundations" title="Open in code, governance," titleMuted="and standards." />
        <Box sx={{display: 'grid', gridTemplateColumns: {xs: '1fr', sm: 'repeat(2, 1fr)', md: 'repeat(3, 1fr)'}, gap: 1.75}}>
          {FOUNDATIONS.map((f, i) => (
            <Reveal
              key={f.title}
              component="article"
              delay={i * 90}
              sx={[
                cardSx(t),
                {
                  p: '14px 14px 22px',
                  '&:hover': {transform: 'translateY(-4px)', bgcolor: t.surfaceStrong},
                },
              ]}
            >
              <Box
                aria-hidden
                sx={{
                  position: 'relative',
                  height: 136,
                  borderRadius: '12px',
                  overflow: 'hidden',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  color: 'text.primary',
                  bgcolor: t.inset,
                  border: '1px solid',
                  borderColor: t.line,
                  /* Faint grid backdrop, faded at the edges. */
                  '&::before': {
                    content: '""',
                    position: 'absolute',
                    inset: 0,
                    backgroundImage: `linear-gradient(${t.line} 1px, transparent 1px), linear-gradient(90deg, ${t.line} 1px, transparent 1px)`,
                    backgroundSize: '16px 16px',
                    maskImage: 'radial-gradient(closest-side, #000, transparent)',
                    WebkitMaskImage: 'radial-gradient(closest-side, #000, transparent)',
                  },
                }}
              >
                <MaskLogo name={f.logo.name} width={f.logo.width} height={f.logo.height} sx={{position: 'relative'}} />
              </Box>
              <Box sx={{px: 1}}>
                <Typography
                  component="span"
                  sx={{
                    display: 'block',
                    mt: 2,
                    fontFamily: 'var(--ifm-font-family-monospace)',
                    fontSize: '0.72rem',
                    letterSpacing: '0.06em',
                    textTransform: 'uppercase',
                    color: t.cyan,
                  }}
                >
                  {f.category}
                </Typography>
                <Typography component="h3" sx={{m: 0, mt: 0.75, fontFamily: DISPLAY_FONT, fontSize: '1.0625rem', fontWeight: 600}}>
                  {f.title}
                </Typography>
                <Typography sx={{mt: 0.75, fontSize: '0.9rem', lineHeight: 1.55, color: 'text.secondary'}}>
                  {f.description}
                </Typography>
              </Box>
            </Reveal>
          ))}
        </Box>
      </Container>
    </Section>
  );
}
