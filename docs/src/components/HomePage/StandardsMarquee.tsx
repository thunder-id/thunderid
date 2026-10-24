// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Typography} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {DISPLAY_FONT, useHomeTokens} from './homeTheme';
import {MaskLogo, Reveal} from './shared';

interface Standard {
  label: string;
  /** Logo file under `static/assets/images/home/logos`. */
  logo?: string;
  /** Wordmark logos replace the text label entirely. */
  wordmark?: {width: number};
}

const STANDARDS: Standard[] = [
  {label: 'OAuth 2.1', logo: 'oauth'},
  {label: 'OpenID Connect', logo: 'openid'},
  {label: 'SAML', logo: 'saml'},
  {label: 'SCIM', logo: 'scim', wordmark: {width: 60}},
  {label: 'AuthZEN', logo: 'openid'},
  {label: 'OpenID4VCI', logo: 'openid'},
  {label: 'OpenID4VP', logo: 'openid'},
  {label: 'ID-JAG'},
  {label: 'CIBA', logo: 'ciba', wordmark: {width: 55}},
  {label: 'MCP', logo: 'mcp'},
  {label: 'A2A', logo: 'a2a'},
];

function StandardItem({label, logo = undefined, wordmark = undefined}: Standard): JSX.Element {
  return (
    <Box
      component="span"
      sx={{
        display: 'inline-flex',
        alignItems: 'center',
        gap: 1.125,
        fontFamily: DISPLAY_FONT,
        fontSize: {xs: '0.9375rem', sm: '1.125rem'},
        fontWeight: 700,
        letterSpacing: '-0.01em',
        whiteSpace: 'nowrap',
        color: 'text.secondary',
        transition: 'color 0.2s',
        '&:hover': {color: 'text.primary'},
      }}
    >
      {logo && wordmark ? (
        <MaskLogo name={logo} width={wordmark.width} height={20} label={label} />
      ) : (
        <>
          {logo && <MaskLogo name={logo} width={24} height={24} />}
          {label}
        </>
      )}
    </Box>
  );
}

export default function StandardsMarquee(): JSX.Element {
  const t = useHomeTokens();
  const group = (hidden: boolean): JSX.Element => (
    <Box
      aria-hidden={hidden || undefined}
      sx={{display: 'flex', gap: {xs: 4.5, md: 8}, pr: {xs: 4.5, md: 8}}}
    >
      {STANDARDS.map((s) => (
        <StandardItem key={s.label} {...s} />
      ))}
    </Box>
  );

  return (
    <Reveal
      delay={420}
      sx={{
        position: 'relative',
        zIndex: 1,
        textAlign: 'center',
        px: 3,
        pt: 3.5,
        pb: {xs: 5, md: 8},
        background: t.isDark
          ? 'linear-gradient(180deg, transparent, rgba(4,10,22,0.72) 45%, rgba(4,10,22,0.72))'
          : 'linear-gradient(180deg, transparent, rgba(54,136,255,0.05) 45%, rgba(54,136,255,0.05))',
      }}
    >
      <Typography sx={{fontSize: '0.875rem', fontWeight: 600, color: 'primary.main', mb: 3.5}}>
        Built on open standards
      </Typography>
      <Box
        sx={{
          overflow: 'hidden',
          maxWidth: 980,
          mx: 'auto',
          maskImage: 'linear-gradient(90deg, transparent, #000 12%, #000 88%, transparent)',
          WebkitMaskImage: 'linear-gradient(90deg, transparent, #000 12%, #000 88%, transparent)',
          '@keyframes standardsMarquee': {to: {transform: 'translateX(-50%)'}},
          '&:hover > div': {animationPlayState: 'paused'},
        }}
      >
        <Box
          sx={{
            display: 'flex',
            width: 'max-content',
            animation: 'standardsMarquee 36s linear infinite',
            '@media (prefers-reduced-motion: reduce)': {animation: 'none'},
          }}
        >
          {group(false)}
          {group(true)}
        </Box>
      </Box>
    </Reveal>
  );
}
