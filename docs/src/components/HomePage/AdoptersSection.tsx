// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import Link from '@docusaurus/Link';
import useBaseUrl from '@docusaurus/useBaseUrl';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import {Box, Container, Typography} from '@wso2/oxygen-ui';
import {ArrowUpRight} from '@wso2/oxygen-ui-icons-react';
import type {JSX, ReactNode} from 'react';
import {cardSx, DISPLAY_FONT, HOME_MAX_WIDTH, useHomeTokens} from './homeTheme';
import {MaskLogo, Reveal, Section, SectionHeading} from './shared';
import type {DocusaurusProductConfig} from '@site/docusaurus.product.config';

const ADOPTER_LOGOS = '/assets/images/home/adopters';

/** Wordmark set next to a logo icon, for brands whose wordmark isn't available as an image. */
function Wordmark({children}: {children: ReactNode}): JSX.Element {
  return (
    <Box
      component="span"
      sx={{fontFamily: DISPLAY_FONT, fontSize: '1.25rem', fontWeight: 700, letterSpacing: '-0.02em', color: 'text.primary'}}
    >
      {children}
    </Box>
  );
}

function OpenChoreoLogo(): JSX.Element {
  const light = useBaseUrl(`${ADOPTER_LOGOS}/openchoreo-light.svg`);
  const dark = useBaseUrl(`${ADOPTER_LOGOS}/openchoreo-dark.svg`);
  // Swapped on Docusaurus's `data-theme`, which is set before first paint, so the wrong variant never flashes.
  return (
    <Box sx={{display: 'flex', alignItems: 'center', gap: 1}}>
      <Box component="img" src={light} alt="" sx={{height: 34, '[data-theme="dark"] &': {display: 'none'}}} />
      <Box component="img" src={dark} alt="" sx={{height: 34, display: 'none', '[data-theme="dark"] &': {display: 'block'}}} />
      <Wordmark>OpenChoreo</Wordmark>
    </Box>
  );
}

function LsfLogo(): JSX.Element {
  const src = useBaseUrl(`${ADOPTER_LOGOS}/lsf.jpg`);
  return <Box component="img" src={src} alt="" sx={{height: 64, borderRadius: '4px'}} />;
}

function MosipEsignetLogo(): JSX.Element {
  const t = useHomeTokens();
  const mosip = useBaseUrl(`${ADOPTER_LOGOS}/mosip.png`);
  const esignet = useBaseUrl(`${ADOPTER_LOGOS}/esignet.svg`);
  return (
    <Box sx={{display: 'flex', alignItems: 'center', gap: 2}}>
      <Box component="img" src={mosip} alt="" sx={{height: 50}} />
      <Box component="span" sx={{alignSelf: 'stretch', width: '1px', my: 0.5, bgcolor: t.lineStrong}} />
      <Box sx={{display: 'flex', alignItems: 'center', gap: 0.25}}>
        <Box component="img" src={esignet} alt="" sx={{height: 28}} />
        <Wordmark>Signet</Wordmark>
      </Box>
    </Box>
  );
}

interface Adopter {
  categories: string[];
  name: string;
  description: string;
  href: string;
  logo: ReactNode;
}

function AdopterCard({adopter, delay}: {adopter: Adopter; delay: number}): JSX.Element {
  const t = useHomeTokens();

  return (
    <Reveal component="li" delay={delay}>
      <Box
        component={Link}
        to={adopter.href}
        target="_blank"
        rel="noopener noreferrer"
        aria-label={`${adopter.name} (opens in a new tab)`}
        sx={[
          cardSx(t),
          {
            display: 'flex',
            flexDirection: 'column',
            height: '100%',
            p: '10px 10px 18px',
            color: 'text.primary',
            textDecoration: 'none',
            '&:hover': {transform: 'translateY(-3px)', bgcolor: t.surfaceStrong, color: 'text.primary', textDecoration: 'none'},
            '&:hover .adopter-go': {opacity: 1, transform: 'translate(2px, -2px)'},
          },
        ]}
      >
        <Box
          aria-hidden
          sx={{
            height: 96,
            borderRadius: '12px',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            px: 2,
            bgcolor: t.inset,
            border: '1px solid',
            borderColor: t.line,
          }}
        >
          {adopter.logo}
        </Box>
        <Box sx={{px: 1}}>
          <Box sx={{display: 'flex', alignItems: 'flex-start', gap: 1, mt: 1.75}}>
            <Typography
              component="span"
              sx={{
                fontFamily: 'var(--ifm-font-family-monospace)',
                fontSize: '0.68rem',
                letterSpacing: '0.06em',
                textTransform: 'uppercase',
                color: t.cyan,
              }}
            >
              {adopter.categories.map((c, i) => (
                <Box key={c} component="span" sx={{whiteSpace: 'nowrap'}}>
                  {i > 0 && ' · '}
                  {c}
                </Box>
              ))}
            </Typography>
            <Box
              component="span"
              className="adopter-go"
              sx={{display: 'inline-flex', ml: 'auto', color: t.link, opacity: 0.5, transition: 'opacity 0.2s, transform 0.2s'}}
            >
              <ArrowUpRight size={16} strokeWidth={2} />
            </Box>
          </Box>
          <Typography component="h3" sx={{m: 0, mt: 0.5, fontFamily: DISPLAY_FONT, fontSize: '1rem', fontWeight: 600}}>
            {adopter.name}
          </Typography>
          <Typography sx={{mt: 0.5, fontSize: '0.85rem', lineHeight: 1.5, color: 'text.secondary'}}>
            {adopter.description}
          </Typography>
        </Box>
      </Box>
    </Reveal>
  );
}

export default function AdoptersSection(): JSX.Element {
  const {siteConfig} = useDocusaurusContext();
  const productName = (siteConfig.customFields?.product as DocusaurusProductConfig).project.name;

  const adopters: Adopter[] = [
    {
      categories: ['Institutional contributor', 'Gov projects'],
      name: 'MOSIP',
      description: `MOSIP contributes to ${productName} and is building eSignet 2.0 on top of it. eSignet is MOSIP's open-source, vendor-neutral digital identity authentication platform for secure, standardized access to online services.`,
      href: 'https://docs.esignet.io',
      logo: <MosipEsignetLogo />,
    },
    {
      categories: ['Gov projects'],
      name: 'Lanka Software Foundation (LSF)',
      description: `LSF works on multiple open-source digital public infrastructure projects for use by any country, and uses ${productName} as the identity provider for its platforms.`,
      href: 'https://opensource.lk',
      logo: <LsfLogo />,
    },
    {
      categories: ['Developer platform'],
      name: 'OpenChoreo',
      description: `OpenChoreo uses ${productName} as the identity provider for its open-source internal developer platform.`,
      href: 'https://openchoreo.dev',
      logo: <OpenChoreoLogo />,
    },
    {
      categories: ['Institutional contributor', 'Developer products'],
      name: 'WSO2',
      description: `WSO2 is the founding contributor to ${productName} and uses it as the identity provider for WSO2 Cloud and as the Key Manager within WSO2 API Platform.`,
      href: 'https://wso2.com',
      logo: <MaskLogo name="wso2" width={110} height={28} label="WSO2" sx={{color: 'text.primary'}} />,
    },
  ];

  return (
    <Section id="adopters">
      <Container maxWidth={false} sx={{maxWidth: HOME_MAX_WIDTH, px: {xs: 2, sm: 4}}}>
        <SectionHeading
          eyebrow="Adopters"
          title="Institutional contributors"
          titleMuted="and early adopters."
          sx={{mb: {xs: 4, md: 5}}}
        />
        <Box
          component="ul"
          sx={{
            listStyle: 'none',
            m: 0,
            p: 0,
            display: 'grid',
            gridTemplateColumns: {xs: '1fr', sm: 'repeat(2, 1fr)'},
            gap: 2,
          }}
        >
          {adopters.map((a, i) => (
            <AdopterCard key={a.name} adopter={a} delay={i * 80} />
          ))}
        </Box>
      </Container>
    </Section>
  );
}
