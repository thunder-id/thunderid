// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/* Design tokens shared by the events hub and the individual event pages, so they read as one family. */

import {ACCENTS, type HomeTokens} from '@site/src/components/HomePage/homeTheme';

export const MONO_LABEL_SX = {
  fontFamily: 'var(--ifm-font-family-monospace)',
  fontSize: '0.6875rem',
  fontWeight: 500,
  letterSpacing: '0.16em',
  textTransform: 'uppercase',
  color: 'text.secondary',
} as const;

export const CONTENT_MAX_WIDTH = 1008;

export const REDUCED_MOTION = '@media (prefers-reduced-motion: reduce)';

/** Solid brand-blue fill for icon badges. */
export const BADGE_GRADIENT = `linear-gradient(140deg, color-mix(in srgb, ${ACCENTS.electric} 70%, #fff), ${ACCENTS.electric} 45%, color-mix(in srgb, ${ACCENTS.electric} 75%, #000))`;

export const DOT_GRID = (color: string) => ({
  backgroundImage: `radial-gradient(${color} 1px, transparent 1px)`,
  backgroundSize: '22px 22px',
});

/** Primary call-to-action button: brand blue with a soft glow. */
export const primaryButtonSx = {
  height: 52,
  px: 3.5,
  width: {xs: '100%', sm: 'auto'},
  fontWeight: 600,
  textTransform: 'none',
  fontSize: '1rem',
  borderRadius: '8px',
  transition: 'transform 0.18s ease, background-color 0.2s ease, box-shadow 0.2s ease',
  boxShadow: 'inset 0 1px 0 rgba(255,255,255,0.12), 0 12px 32px -12px rgba(54,136,255,0.75)',
  '&:hover': {
    transform: 'translateY(-2px)',
    color: 'common.white',
    boxShadow: 'inset 0 1px 0 rgba(255,255,255,0.12), 0 16px 40px -12px rgba(54,136,255,0.9)',
  },
  [REDUCED_MOTION]: {transition: 'none', '&:hover': {transform: 'none'}},
} as const;

export function surfaceCardSx(t: HomeTokens) {
  return {
    borderRadius: '20px',
    bgcolor: t.surfaceStrong,
    border: '1px solid',
    borderColor: t.line,
    boxShadow: t.isDark ? 'none' : '0 1px 2px rgba(15,40,80,0.04)',
  };
}

/** Feature card: blue-tinted surface with an accent edge along the top, lifting on hover. */
export function accentCardSx(t: HomeTokens) {
  const blue = ACCENTS.electric;
  return {
    ...surfaceCardSx(t),
    position: 'relative',
    overflow: 'hidden',
    background: `linear-gradient(150deg, color-mix(in srgb, ${blue} ${t.isDark ? 16 : 10}%, transparent), transparent 60%), ${t.surfaceStrong}`,
    borderColor: `color-mix(in srgb, ${blue} 22%, transparent)`,
    transition: 'transform 0.35s ease, border-color 0.35s ease, box-shadow 0.35s ease',
    // Accent edge along the top of the card.
    '&::before': {
      content: '""',
      position: 'absolute',
      top: 0,
      left: 0,
      right: 0,
      height: 3,
      background: `linear-gradient(90deg, ${blue}, color-mix(in srgb, ${blue} 35%, transparent) 70%, transparent)`,
    },
    '&:hover': {
      transform: 'translateY(-6px)',
      borderColor: `color-mix(in srgb, ${blue} 55%, transparent)`,
      boxShadow: `0 24px 48px -24px color-mix(in srgb, ${blue} 55%, transparent)`,
    },
    [REDUCED_MOTION]: {'&:hover': {transform: 'none'}},
  } as const;
}

/**
 * Badge colours per event type, for light and dark mode. Hosted uses the brand blue; presence and talk use two
 * quieter tones. Each foreground meets WCAG AA (4.5:1) on its own background.
 */
export const EVENT_TYPE_TONES = {
  light: {
    hosted: {fg: '#1652b8', bg: 'rgba(54,136,255,0.12)', border: 'rgba(54,136,255,0.38)'},
    presence: {fg: '#5236c0', bg: 'rgba(123,92,246,0.10)', border: 'rgba(123,92,246,0.32)'},
    talk: {fg: '#3b4658', bg: 'rgba(15,40,80,0.05)', border: 'rgba(15,40,80,0.16)'},
  },
  dark: {
    hosted: {fg: '#a9ccff', bg: 'rgba(54,136,255,0.18)', border: 'rgba(54,136,255,0.5)'},
    presence: {fg: '#cdbfff', bg: 'rgba(123,92,246,0.18)', border: 'rgba(123,92,246,0.45)'},
    talk: {fg: '#c5cfdd', bg: 'rgba(160,200,255,0.06)', border: 'rgba(160,200,255,0.2)'},
  },
} as const;

/** Hides content visually while keeping it available to screen readers. */
export const VISUALLY_HIDDEN_SX = {
  position: 'absolute',
  width: '1px',
  height: '1px',
  p: 0,
  m: '-1px',
  overflow: 'hidden',
  clip: 'rect(0 0 0 0)',
  whiteSpace: 'nowrap',
  border: 0,
} as const;

/** Link blue. The site's light-mode link colour sits just under 4.5:1 on white, so light mode uses a deeper blue. */
export function linkColor(t: HomeTokens): string {
  return t.isDark ? t.link : '#1d5fc9';
}

/** Secondary outlined button. Light mode uses a deeper blue so the label meets WCAG AA on white. */
export function outlinedButtonSx(t: HomeTokens) {
  return {
    minHeight: 44,
    px: 2.25,
    borderRadius: '8px',
    textTransform: 'none',
    fontWeight: 600,
    color: linkColor(t),
    bgcolor: t.surfaceStrong,
    borderColor: `color-mix(in srgb, ${ACCENTS.electric} 45%, transparent)`,
    transition: 'transform 0.18s ease, background-color 0.2s ease, border-color 0.2s ease',
    '&:hover': {
      transform: 'translateY(-2px)',
      borderColor: ACCENTS.electric,
      bgcolor: `color-mix(in srgb, ${ACCENTS.electric} 8%, ${t.surfaceStrong})`,
    },
    [REDUCED_MOTION]: {transition: 'none', '&:hover': {transform: 'none'}},
  } as const;
}

/** Background of the events hub's fixed left panel. */
export const SPLIT_PANE_BG = {light: '#e8f2ff', dark: '#020c1b'} as const;
