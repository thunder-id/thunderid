// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useCallback} from 'react';
import useIsDarkMode from '../../../hooks/useIsDarkMode';
import type {EcosystemTone} from '@site/src/types/ecosystem';

/** Default accent when an entry does not set one of its own. */
export const DEFAULT_ACCENT = '#3688ff';

/**
 * Translucent ink that works on both themes.
 *
 * The detail pages lean on many low-contrast greys, and spelling each one out
 * as a light/dark pair at every call site buried the layout. This keeps the two
 * alphas together so a tweak stays in one place.
 */
export function useInk(): (light: number, dark: number) => string {
  const isLight = !useIsDarkMode();
  return useCallback(
    (light: number, dark: number) => (isLight ? `rgba(0,0,0,${light})` : `rgba(255,255,255,${dark})`),
    [isLight],
  );
}

const TONE_COLOURS: Record<EcosystemTone, string> = {
  success: '#4ade80',
  danger: '#f87171',
  warning: '#fbbf24',
  accent: '#8bf9fa',
  primary: '#3688ff',
  purple: '#a78bfa',
  neutral: '#94a3b8',
};

export function toneColour(tone: EcosystemTone | undefined, fallback: string): string {
  return tone ? TONE_COLOURS[tone] : fallback;
}
