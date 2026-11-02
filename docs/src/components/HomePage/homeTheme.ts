// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {SxProps, Theme} from '@wso2/oxygen-ui';
import useIsDarkMode from '../../hooks/useIsDarkMode';

/* ─── Design tokens ──────────────────────────────────────────────────────── */

export const ACCENTS = {
  electric: '#3688FF',
  violet: '#7B5CF6',
  cyan: '#00D5FF',
  green: '#3fb950',
} as const;

export type AccentKey = 'electric' | 'violet' | 'cyan';

export interface HomeTokens {
  isDark: boolean;
  /** Translucent card surface. */
  surface: string;
  /** Stronger card surface (hover / emphasised cards). */
  surfaceStrong: string;
  /** Recessed panel used for card visuals. */
  inset: string;
  line: string;
  lineStrong: string;
  /** De-emphasised text, one step below `text.secondary`. */
  muted2: string;
  /** Link / sub-heading accent. */
  link: string;
  cyan: string;
  shadow: string;
}

export function useHomeTokens(): HomeTokens {
  const isDark = useIsDarkMode();
  return isDark
    ? {
        isDark,
        surface: 'rgba(5,33,63,0.5)',
        surfaceStrong: 'rgba(8,40,74,0.64)',
        inset: 'rgba(3,8,18,0.62)',
        line: 'rgba(160,200,255,0.10)',
        lineStrong: 'rgba(160,200,255,0.18)',
        muted2: '#5b6b85',
        link: '#7cb4ff',
        cyan: ACCENTS.cyan,
        shadow: '0 32px 90px -30px rgba(2,8,20,0.9)',
      }
    : {
        isDark,
        surface: 'rgba(255,255,255,0.78)',
        surfaceStrong: '#ffffff',
        inset: 'rgba(54,136,255,0.04)',
        line: 'rgba(15,40,80,0.09)',
        lineStrong: 'rgba(15,40,80,0.16)',
        muted2: 'rgba(15,30,60,0.48)',
        link: '#2a74e0',
        cyan: '#0092c2',
        shadow: '0 24px 60px -30px rgba(15,40,80,0.25)',
      };
}

export const BRAND_GRADIENT = 'linear-gradient(90deg, #2d78e0 0%, #3688FF 55%, #3688FF 100%)';

export const gradientTextSx: SxProps<Theme> = {
  background: BRAND_GRADIENT,
  WebkitBackgroundClip: 'text',
  WebkitTextFillColor: 'transparent',
  backgroundClip: 'text',
};

export const DISPLAY_FONT =
  "'Plus Jakarta Sans', ui-sans-serif, system-ui, -apple-system, 'Segoe UI', Roboto, sans-serif";

/** Content width of the home page sections (1232px) plus the container's side padding. */
export const HOME_MAX_WIDTH = 1296;

export const EASE = 'cubic-bezier(0.22, 0.7, 0.2, 1)';

/** Glass card styling shared by most home page cards. */
export function cardSx(t: HomeTokens) {
  return {
    position: 'relative',
    overflow: 'hidden',
    borderRadius: '20px',
    bgcolor: t.surface,
    border: '1px solid',
    borderColor: t.line,
    backdropFilter: 'blur(14px)',
    boxShadow: t.isDark ? 'none' : '0 1px 2px rgba(15,40,80,0.04)',
    transition: `opacity 0.7s ${EASE}, transform 0.35s ${EASE}, background-color 0.3s, border-color 0.3s, filter 0.3s`,
  };
}

/* ─── sx helpers ─────────────────────────────────────────────────────────── */

type SxArray = Extract<SxProps<Theme>, readonly unknown[]>;

/** Normalises an optional `sx` prop to an array so it can be appended to a base style. */
export function sxList(sx?: SxProps<Theme>): SxArray {
  if (!sx) return [];
  return (Array.isArray(sx) ? sx : [sx]) as SxArray;
}
