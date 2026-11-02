// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Container, Link, Typography} from '@wso2/oxygen-ui';
import {JSX} from 'react';
import {BRAND_GRADIENT, DISPLAY_FONT, useHomeTokens, HOME_MAX_WIDTH} from './homeTheme';
import {MaskLogo, Reveal, Section} from './shared';
import ProductName from '@site/src/components/ProductName';

const LFDT_URL = 'https://www.lfdecentralizedtrust.org/';

export default function FooterSection(): JSX.Element {
  const t = useHomeTokens();

  return (
    <Section id="cta" sx={{pb: {xs: 6, md: 10}}}>
      <Container maxWidth={false} sx={{maxWidth: HOME_MAX_WIDTH, px: {xs: 2, sm: 4}}}>
        <Reveal
          delay={120}
          sx={{
            position: 'relative',
            px: {xs: 3, md: 8},
            py: {xs: 5, md: 9},
            borderRadius: '20px',
            textAlign: 'center',
            bgcolor: t.isDark ? 'rgba(5,33,63,0.55)' : t.surfaceStrong,
            border: '1px solid',
            borderColor: t.lineStrong,
            boxShadow: t.shadow,
            backdropFilter: 'blur(16px)',
          }}
        >
          <Typography
            sx={{
              maxWidth: 760,
              mx: 'auto',
              fontFamily: DISPLAY_FONT,
              fontSize: {xs: '1.25rem', md: '1.875rem'},
              fontWeight: 600,
              lineHeight: 1.5,
              letterSpacing: '-0.01em',
              color: 'text.secondary',
            }}
          >
            <Box component="strong" sx={{color: 'text.primary', fontWeight: 700}}>
              <ProductName />
            </Box>{' '}
            is an{' '}
            <Link
              href={LFDT_URL}
              target="_blank"
              rel="noopener noreferrer"
              sx={{
                fontWeight: 700,
                background: BRAND_GRADIENT,
                WebkitBackgroundClip: 'text',
                WebkitTextFillColor: 'transparent',
                backgroundClip: 'text',
                textDecoration: 'none',
                '&:hover': {textDecoration: 'underline', textDecorationColor: '#3688FF'},
              }}
            >
              LF Decentralized Trust
            </Link>{' '}
            project
          </Typography>

          <Box
            component="a"
            href={LFDT_URL}
            target="_blank"
            rel="noopener noreferrer"
            sx={{
              display: 'block',
              width: 'min(520px, 100%)',
              mx: 'auto',
              my: {xs: 3, md: 4.5},
              color: 'text.primary',
              '& > span': {transition: 'transform 0.3s'},
              '&:hover > span': {transform: 'scale(1.03)'},
            }}
          >
            <MaskLogo
              name="lfdt-full"
              width="100%"
              height="auto"
              label="LF Decentralized Trust"
              sx={{display: 'block', aspectRatio: '698.7 / 39.1'}}
            />
          </Box>

          <Box
            component="a"
            href="https://wso2.com/identity-platform/"
            target="_blank"
            rel="noopener noreferrer"
            sx={{
              display: 'inline-flex',
              alignItems: 'center',
              gap: 1.25,
              px: 2.5,
              py: 1.25,
              borderRadius: '999px',
              fontSize: {xs: '0.9375rem', md: '1.0625rem'},
              fontWeight: 600,
              color: 'text.primary',
              textDecoration: 'none',
              bgcolor: t.isDark ? 'rgba(4,12,26,0.6)' : 'rgba(15,40,80,0.03)',
              border: '1px solid',
              borderColor: t.lineStrong,
              '&:hover': {color: 'text.primary', textDecoration: 'none', borderColor: 'primary.main'},
              '@keyframes heartBeat': {
                '0%, 100%': {transform: 'scale(1)'},
                '15%': {transform: 'scale(1.22)'},
                '30%': {transform: 'scale(1)'},
                '45%': {transform: 'scale(1.14)'},
              },
            }}
          >
            Made with{' '}
            <Box
              component="span"
              aria-label="love"
              sx={{
                display: 'inline-block',
                animation: 'heartBeat 1.6s ease-in-out infinite',
                '@media (prefers-reduced-motion: reduce)': {animation: 'none'},
              }}
            >
              ❤️
            </Box>{' '}
            by <MaskLogo name="wso2" width={81} height={20} label="WSO2" />
          </Box>
        </Reveal>
      </Container>
    </Section>
  );
}
