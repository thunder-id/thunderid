// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {ThemeProvider, type ThemeProviderProps} from '@thunderid/react';
import {useColorScheme, useTheme, type Theme} from '@wso2/oxygen-ui';
import {useMemo, type JSX, type ReactNode} from 'react';

type ColorScheme = 'dark' | 'light';
type SdkThemeConfig = NonNullable<ThemeProviderProps['theme']>;

type Palette = Theme['palette'];

/**
 * The palette of a color scheme: the scheme's own when the theme defines one, else the theme's.
 */
const paletteFor = (theme: Theme, scheme: ColorScheme): Palette =>
  (theme as unknown as {colorSchemes?: Partial<Record<ColorScheme, {palette?: Palette}>>}).colorSchemes?.[scheme]
    ?.palette ?? theme.palette;

/**
 * Themes the SDK components the manage-* pages render with the app's Oxygen theme, in whichever
 * color scheme the app shows, so they read as part of the app.
 */
export default function SdkTheme({children}: {children: ReactNode}): JSX.Element {
  const theme: Theme = useTheme();
  const {mode, systemMode} = useColorScheme();
  const scheme: ColorScheme = (mode === 'system' ? systemMode : mode) === 'dark' ? 'dark' : 'light';

  const sdkTheme: SdkThemeConfig = useMemo((): SdkThemeConfig => {
    const palette: Palette = paletteFor(theme, scheme);
    const radius = Number(theme.shape?.borderRadius ?? 8);

    return {
      borderRadius: {large: `${radius * 2}px`, medium: `${radius}px`, small: `${Math.round(radius * 0.75)}px`},
      colors: {
        background: {body: {main: palette.background.default}, surface: palette.background.paper},
        border: palette.divider,
        primary: {
          contrastText: palette.primary.contrastText,
          dark: palette.primary.dark,
          main: palette.primary.main,
        },
        text: {primary: palette.text.primary, secondary: palette.text.secondary},
      },
      typography: {fontFamily: String(theme.typography.fontFamily)},
    } as SdkThemeConfig;
  }, [theme, scheme]);

  return (
    <ThemeProvider mode={scheme} theme={sdkTheme}>
      {children}
    </ThemeProvider>
  );
}
