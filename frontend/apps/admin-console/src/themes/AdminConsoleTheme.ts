// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {createOxygenTheme, OxygenTheme} from '@wso2/oxygen-ui';

/**
 * The corner radius every surface derives from.
 */
const RADIUS = 2;

const FONT_FAMILY = "'Inter Variable', 'Inter', system-ui, sans-serif";

/**
 * Each color scheme's colors. The primary red is deeper in the light scheme and lighter in the dark
 * one, so a filled button's label, red text on a surface and secondary text all meet WCAG AAA
 * contrast.
 */
const COLORS = {
  dark: {
    background: '#0f1115',
    border: 'rgba(255, 255, 255, 0.08)',
    primary: {
      contrastText: '#1f1f1f',
      dark: '#e57373',
      darkChannel: '229 115 115',
      light: '#f8b4ae',
      lightChannel: '248 180 174',
      main: '#f28b82',
      mainChannel: '242 139 130',
    },
    surface: '#181b22',
    textPrimary: '#e8eaed',
    textSecondary: 'rgba(232, 234, 237, 0.72)',
  },
  light: {
    background: '#f5f5f7',
    border: 'rgba(15, 23, 42, 0.08)',
    primary: {
      contrastText: '#ffffff',
      dark: '#7f1010',
      darkChannel: '127 16 16',
      light: '#c62828',
      lightChannel: '198 40 40',
      main: '#a31515',
      mainChannel: '163 21 21',
    },
    surface: '#ffffff',
    textPrimary: '#1f1f1f',
    textSecondary: 'rgba(31, 31, 31, 0.82)',
  },
};

const palette = (colors: (typeof COLORS)['dark']) => ({
  background: {acrylic: colors.surface, default: colors.background, paper: colors.surface},
  divider: colors.border,
  primary: colors.primary,
  text: {primary: colors.textPrimary, secondary: colors.textSecondary},
});

/**
 * AdminConsoleTheme - the Admin Console's theme: neutral surfaces with a single red primary, flat
 * panels with hairline borders, and near-square corners, so the pages read as an end-user product
 * rather than an operator tool.
 */
export const AdminConsoleThemeConfig = {
  colorSchemes: {
    dark: {palette: palette(COLORS.dark)},
    light: {palette: palette(COLORS.light)},
  },
  components: {
    MuiButton: {
      styleOverrides: {
        containedPrimary: {boxShadow: 'none'},
        root: {borderRadius: RADIUS, fontWeight: 500, textTransform: 'none' as const},
      },
    },
    MuiCssBaseline: {
      styleOverrides: {
        body: {backgroundImage: 'none', fontFamily: FONT_FAMILY},
      },
    },
    MuiPaper: {
      styleOverrides: {
        root: {
          "html[data-color-scheme='dark'] &": {borderColor: COLORS.dark.border},
          backdropFilter: 'none',
          backgroundImage: 'none',
          borderColor: COLORS.light.border,
          boxShadow: 'none',
        },
      },
    },
  },
  shape: {borderRadius: RADIUS},
  typography: {fontFamily: FONT_FAMILY},
};

const AdminConsoleTheme = createOxygenTheme(AdminConsoleThemeConfig, OxygenTheme);

export default AdminConsoleTheme;
