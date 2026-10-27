// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {ThemeProviderProps} from '@thunderid/react';
import {createOxygenTheme} from '@wso2/oxygen-ui';
import type {AppTheme, AppThemeColors} from '../models/app-config';

/**
 * A color scheme of the app theme.
 *
 * @public
 */
export type AppColorScheme = 'light' | 'dark';

/**
 * An app theme with every value filled in.
 *
 * @public
 */
export interface ResolvedAppTheme {
  borderRadius: number;
  colorSchemes: Record<AppColorScheme, Required<AppThemeColors>>;
  fontFamily: string;
}

/**
 * The apps' own look, used for every value an app theme leaves out: a soft page, white outlined
 * surfaces and quiet controls, with ThunderID blue for actions.
 *
 * @public
 */
export const DEFAULT_APP_THEME: ResolvedAppTheme = {
  borderRadius: 8,
  colorSchemes: {
    dark: {
      background: '#0f1115',
      border: 'rgba(255, 255, 255, 0.08)',
      primary: '#3688FF',
      surface: '#181b22',
      textPrimary: '#e8eaed',
      textSecondary: 'rgba(232, 234, 237, 0.68)',
    },
    light: {
      background: '#f5f5f7',
      border: 'rgba(15, 23, 42, 0.08)',
      primary: '#3688FF',
      surface: '#ffffff',
      textPrimary: '#1f1f1f',
      textSecondary: 'rgba(31, 31, 31, 0.66)',
    },
  },
  fontFamily: "'Inter Variable', 'Inter', system-ui, sans-serif",
};

/**
 * The colors the server accepts in an app theme: hex, or `rgb()`/`rgba()`.
 *
 * @public
 */
export const APP_THEME_COLOR = /^(#[0-9a-f]{3,4}|#[0-9a-f]{6}|#[0-9a-f]{8}|rgba?\(\s*[0-9.%,\s/]+\))$/i;

const DISABLED_BACKGROUNDS: Record<AppColorScheme, string> = {
  dark: 'rgba(255, 255, 255, 0.04)',
  light: 'rgba(15, 23, 42, 0.04)',
};

/**
 * Fills in what an app theme leaves out with the apps' own look.
 *
 * @param theme - The saved app theme, if any.
 * @returns The theme with every value set.
 *
 * @public
 */
export const resolveAppTheme = (theme?: AppTheme): ResolvedAppTheme => {
  const scheme = (name: AppColorScheme): Required<AppThemeColors> => {
    const saved: AppThemeColors = theme?.colorSchemes?.[name] ?? {};
    const fallback: Required<AppThemeColors> = DEFAULT_APP_THEME.colorSchemes[name];

    return {
      background: saved.background ?? fallback.background,
      border: saved.border ?? fallback.border,
      primary: saved.primary ?? fallback.primary,
      surface: saved.surface ?? fallback.surface,
      textPrimary: saved.textPrimary ?? fallback.textPrimary,
      textSecondary: saved.textSecondary ?? fallback.textSecondary,
    };
  };

  return {
    borderRadius: theme?.borderRadius ?? DEFAULT_APP_THEME.borderRadius,
    colorSchemes: {dark: scheme('dark'), light: scheme('light')},
    fontFamily: theme?.fontFamily ?? DEFAULT_APP_THEME.fontFamily,
  };
};

/**
 * Reads a hex or `rgb()` color into its red, green and blue channels.
 */
const toRgb = (color: string): [number, number, number] | null => {
  const hex: string | undefined = /^#([0-9a-f]{3,8})$/i.exec(color)?.[1];

  if (hex) {
    const full: string = hex.length <= 4 ? [...hex.slice(0, 3)].map((c: string) => c + c).join('') : hex.slice(0, 6);

    return [0, 2, 4].map((i: number) => parseInt(full.slice(i, i + 2), 16)) as [number, number, number];
  }

  const channels: number[] | undefined = /^rgba?\(([^)]+)\)$/i
    .exec(color)?.[1]
    .split(/[\s,/]+/)
    .filter(Boolean)
    .slice(0, 3)
    .map(Number);

  return channels?.length === 3 && channels.every((channel: number) => Number.isFinite(channel))
    ? (channels as [number, number, number])
    : null;
};

const toHex = (channels: number[]): string =>
  `#${channels
    .map((channel: number) =>
      Math.round(Math.min(255, Math.max(0, channel)))
        .toString(16)
        .padStart(2, '0'),
    )
    .join('')}`;

/** Mixes a color toward black (negative amount) or white (positive amount). */
const shade = (color: string, amount: number): string => {
  const rgb: [number, number, number] | null = toRgb(color);
  if (!rgb) return color;

  return toHex(
    rgb.map((channel: number) => (amount < 0 ? channel * (1 + amount) : channel + (255 - channel) * amount)),
  );
};

/** The space-separated channels Oxygen needs alongside a color for its transparent variants. */
const channelsOf = (color: string): string | undefined => toRgb(color)?.join(' ');

/** White or near-black, whichever reads better on the color. */
const contrastTextFor = (color: string): string => {
  const rgb: [number, number, number] | null = toRgb(color);
  if (!rgb) return '#ffffff';
  const [r, g, b] = rgb;

  return 0.299 * r + 0.587 * g + 0.114 * b > 170 ? '#111111' : '#ffffff';
};

const primaryPalette = (main: string): Record<string, string | undefined> => {
  const dark: string = shade(main, -0.2);
  const light: string = shade(main, 0.3);

  // Explicit transparency channels: Oxygen freezes these at its own palette during the theme merge.
  return {
    contrastText: contrastTextFor(main),
    dark,
    darkChannel: channelsOf(dark),
    light,
    lightChannel: channelsOf(light),
    main,
    mainChannel: channelsOf(main),
  };
};

/**
 * Builds the Oxygen theme an app's shell uses from its app theme.
 *
 * @param theme - The saved app theme, if any.
 * @returns The Oxygen theme for the app.
 *
 * @public
 */
export const createAppTheme = (theme?: AppTheme): ReturnType<typeof createOxygenTheme> => {
  const resolved: ResolvedAppTheme = resolveAppTheme(theme);
  const {dark, light} = resolved.colorSchemes;
  const palette = (colors: Required<AppThemeColors>) => ({
    background: {acrylic: colors.surface, default: colors.background, paper: colors.surface},
    divider: colors.border,
    primary: primaryPalette(colors.primary),
    text: {primary: colors.textPrimary, secondary: colors.textSecondary},
  });

  return createOxygenTheme({
    colorSchemes: {dark: {palette: palette(dark)}, light: {palette: palette(light)}},
    components: {
      MuiButton: {
        styleOverrides: {
          containedPrimary: {
            "html[data-color-scheme='dark'] &": {
              '&:hover': {backgroundColor: shade(dark.primary, -0.2)},
              backgroundColor: dark.primary,
            },
            '&:hover': {backgroundColor: shade(light.primary, -0.2)},
            backgroundColor: light.primary,
            boxShadow: 'none',
          },
          root: {borderRadius: resolved.borderRadius * 2.5, fontWeight: 500, textTransform: 'none'},
        },
      },
      MuiCssBaseline: {
        styleOverrides: {
          body: {backgroundImage: 'none', fontFamily: resolved.fontFamily},
        },
      },
      MuiPaper: {
        styleOverrides: {
          root: {
            "html[data-color-scheme='dark'] &": {borderColor: dark.border},
            backdropFilter: 'none',
            backgroundImage: 'none',
            borderColor: light.border,
            boxShadow: 'none',
          },
        },
      },
    },
    shape: {borderRadius: resolved.borderRadius},
    typography: {fontFamily: resolved.fontFamily},
  } as Parameters<typeof createOxygenTheme>[0]);
};

/**
 * Builds the SDK theme an app's SDK components use, for each color scheme, from its app theme.
 *
 * @param theme - The saved app theme, if any.
 * @returns The SDK theme for each color scheme.
 *
 * @public
 */
export const createAppSdkThemes = (
  theme?: AppTheme,
): Record<AppColorScheme, NonNullable<ThemeProviderProps['theme']>> => {
  const resolved: ResolvedAppTheme = resolveAppTheme(theme);
  const radius: number = resolved.borderRadius;
  const sdkTheme = (name: AppColorScheme): NonNullable<ThemeProviderProps['theme']> => {
    const colors: Required<AppThemeColors> = resolved.colorSchemes[name];

    return {
      borderRadius: {large: `${radius * 2}px`, medium: `${radius}px`, small: `${Math.round(radius * 0.75)}px`},
      colors: {
        background: {body: {main: colors.background}, disabled: DISABLED_BACKGROUNDS[name], surface: colors.surface},
        border: colors.border,
        primary: {
          contrastText: contrastTextFor(colors.primary),
          dark: shade(colors.primary, -0.2),
          main: colors.primary,
        },
        text: {primary: colors.textPrimary, secondary: colors.textSecondary},
      },
      typography: {fontFamily: resolved.fontFamily},
    };
  };

  return {dark: sdkTheme('dark'), light: sdkTheme('light')};
};
