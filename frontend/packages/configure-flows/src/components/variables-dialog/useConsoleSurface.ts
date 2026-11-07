// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {ThemeProviderProps} from '@thunderid/react';
import {useColorScheme, useTheme} from '@wso2/oxygen-ui';
import {useMemo} from 'react';

/**
 * What a full-screen view needs to look like the rest of the console, in the color scheme the console is in.
 */
export interface ConsoleSurface {
  /**
   * Styles for the background of a full-screen dialog: the console's own page background, the one its theme paints on
   * the body, so the dialog covers the page without turning flatter or darker than it.
   */
  pageSx: Record<string, unknown>;
  /**
   * The color scheme the console is in.
   */
  scheme: 'dark' | 'light';
  /**
   * The console's colors as a theme for the SDK components, which keep a color scheme of their own and would otherwise
   * stay light in a dark console, with white cards on a dark page.
   */
  sdkTheme: ThemeProviderProps['theme'];
}

const BODY_STYLES = ['backgroundAttachment', 'backgroundBlendMode', 'backgroundImage'];

/**
 * Reads the console's color scheme and palette, so a full-screen view and the SDK components inside it follow them.
 *
 * @returns The page background styles, the color scheme and an SDK theme.
 */
export default function useConsoleSurface(): ConsoleSurface {
  const theme = useTheme();
  const {mode, systemMode} = useColorScheme();
  const scheme: 'dark' | 'light' = (mode === 'system' ? systemMode : mode) === 'dark' ? 'dark' : 'light';

  const pageSx: Record<string, unknown> = useMemo(() => {
    // The theme paints the page through its CssBaseline overrides on the body, and a dialog sits above the body.
    const overrides = theme.components?.MuiCssBaseline?.styleOverrides as Record<string, unknown> | undefined;
    const body = overrides?.[`html[data-color-scheme='${scheme}'] body`] as Record<string, unknown> | undefined;
    const styles: Record<string, unknown> = {};

    BODY_STYLES.forEach((key: string) => {
      if (body?.[key] !== undefined) {
        styles[key] = body[key];
      }
    });

    return {backgroundColor: 'background.default', ...styles};
  }, [theme, scheme]);

  const sdkTheme: ThemeProviderProps['theme'] = useMemo(() => {
    const palette = theme.vars?.palette;

    if (!palette) {
      return {};
    }

    // The CSS variables follow the scheme by themselves. Whole color groups are given, since the SDK replaces a group
    // that is overridden instead of merging into it.
    return {
      colors: {
        background: {
          body: {main: palette.background.default},
          disabled: palette.action.disabledBackground,
          // Cards and inputs sit on the page as it is, the way the console's own panels do.
          surface: 'transparent',
        },
        border: palette.divider,
        primary: {contrastText: palette.primary.contrastText, dark: palette.primary.dark, main: palette.primary.main},
        text: {primary: palette.text.primary, secondary: palette.text.secondary},
      },
    } as ThemeProviderProps['theme'];
  }, [theme]);

  return {pageSx, scheme, sdkTheme};
}
