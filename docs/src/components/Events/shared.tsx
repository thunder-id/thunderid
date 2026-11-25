// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/* Components shared by the events hub and the individual event pages. */

import {Box, Container, keyframes, Typography} from '@wso2/oxygen-ui';
import type {JSX, ReactNode} from 'react';
import {CONTENT_MAX_WIDTH, DOT_GRID, REDUCED_MOTION} from './eventsTheme';
import {ACCENTS, DISPLAY_FONT, gradientTextSx, useHomeTokens} from '@site/src/components/HomePage/homeTheme';

const drift = keyframes`
  0%, 100% { transform: translate3d(0, 0, 0) scale(1); }
  50% { transform: translate3d(6%, 8%, 0) scale(1.12); }
`;

export function Content({children}: {children: ReactNode}): JSX.Element {
  return (
    <Container maxWidth={false} sx={{maxWidth: CONTENT_MAX_WIDTH, px: {xs: 2, sm: 4}}}>
      {children}
    </Container>
  );
}

/** A full-width page section. Give it an `id` to make it a jump target for in-page links, which also move focus to it. */
export function Band({
  children,
  id = undefined,
  tinted = false,
}: {
  children: ReactNode;
  id?: string;
  tinted?: boolean;
}): JSX.Element {
  const t = useHomeTokens();
  return (
    <Box
      component="section"
      id={id}
      tabIndex={id ? -1 : undefined}
      sx={{
        py: {xs: 6, md: 9},
        borderTop: '1px solid',
        borderColor: t.line,
        bgcolor: tinted ? t.inset : 'transparent',
        scrollMarginTop: 'var(--ifm-navbar-height)',
        '&:focus': {outline: 'none'},
      }}
    >
      <Content>{children}</Content>
    </Box>
  );
}

/** `flush` drops the bottom margin, for headings that share a row with other content. */
export function SectionTitle({
  eyebrow,
  flush = false,
  children,
}: {
  eyebrow: string;
  flush?: boolean;
  children: ReactNode;
}): JSX.Element {
  return (
    <>
      <Typography
        component="div"
        sx={{fontSize: '0.875rem', fontWeight: 600, mb: 1.25, ...gradientTextSx, width: 'fit-content'}}
      >
        {eyebrow}
      </Typography>
      <Typography
        variant="h2"
        sx={{
          fontFamily: DISPLAY_FONT,
          fontSize: {xs: '1.75rem', md: '2.25rem'},
          fontWeight: 700,
          letterSpacing: '-0.025em',
          lineHeight: 1.15,
          color: 'text.primary',
          mt: 0,
          mb: flush ? 0 : {xs: 3, md: 4},
        }}
      >
        {children}
      </Typography>
    </>
  );
}

/** Slow-drifting, blurred brand-colour glows over a faint dot grid, behind the hero. */
export function Aurora(): JSX.Element {
  const t = useHomeTokens();
  const blob = (color: string, size: string, position: Record<string, string>, delay: string) => ({
    position: 'absolute',
    width: size,
    height: size,
    borderRadius: '50%',
    background: color,
    filter: 'blur(90px)',
    opacity: t.isDark ? 0.4 : 0.32,
    animation: `${drift} 18s ease-in-out ${delay} infinite`,
    [REDUCED_MOTION]: {animation: 'none'},
    ...position,
  });

  return (
    <Box aria-hidden sx={{position: 'absolute', inset: 0, overflow: 'hidden', pointerEvents: 'none'}}>
      <Box sx={blob(ACCENTS.electric, '46vw', {top: '-18vw', left: '-8vw'}, '0s')} />
      <Box sx={blob(ACCENTS.cyan, '36vw', {top: '-12vw', right: '-10vw'}, '-6s')} />
      <Box sx={blob(ACCENTS.violet, '30vw', {top: '22vw', left: '38vw'}, '-12s')} />
      <Box
        sx={{
          position: 'absolute',
          inset: 0,
          ...DOT_GRID(t.isDark ? 'rgba(160,200,255,0.12)' : 'rgba(54,136,255,0.16)'),
          maskImage: 'radial-gradient(ellipse 65% 55% at 50% 30%, #000 20%, transparent 75%)',
          WebkitMaskImage: 'radial-gradient(ellipse 65% 55% at 50% 30%, #000 20%, transparent 75%)',
        }}
      />
    </Box>
  );
}
