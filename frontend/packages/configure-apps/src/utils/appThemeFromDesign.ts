// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {createThemeFromDesign, type Theme} from '@thunderid/design';
import type {AppColorScheme, AppTheme, AppThemeColors} from '@thunderid/system-apps';

type Palette = Theme['palette'];

/**
 * The App theme an app takes when it shares the Gate theme of the application it signs in with,
 * read from that Design theme as the app reads it: the palette of each color scheme, its corner
 * radius and its font.
 *
 * @param designTheme - The Gate's Design theme, as the server stores it.
 * @returns The App theme, or `undefined` when there is no Gate theme, so the app keeps its own look.
 */
export default function appThemeFromDesign(designTheme: unknown): AppTheme | undefined {
  const theme: Theme | undefined = createThemeFromDesign(designTheme);

  if (!theme) {
    return undefined;
  }

  const schemes = (theme as unknown as {colorSchemes?: Partial<Record<AppColorScheme, {palette?: Palette}>>})
    .colorSchemes;
  const colorsFor = (scheme: AppColorScheme): AppThemeColors => {
    const palette: Palette = schemes?.[scheme]?.palette ?? theme.palette;

    return {
      background: palette.background.default,
      border: palette.divider,
      primary: palette.primary.main,
      surface: palette.background.paper,
      textPrimary: palette.text.primary,
      textSecondary: palette.text.secondary,
    };
  };

  return {
    borderRadius: Number(theme.shape?.borderRadius ?? 8),
    colorSchemes: {dark: colorsFor('dark'), light: colorsFor('light')},
    fontFamily: String(theme.typography.fontFamily),
  };
}
