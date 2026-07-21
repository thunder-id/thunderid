/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {createOxygenTheme, OxygenTheme, type OxygenThemeType} from '@wso2/oxygen-ui';

/**
 * DefaultTheme - The default theme for Thunder ID applications
 * Features: Electric blue primary, indigo secondary, deep purple dark backgrounds with ambient glow
 * Evokes intelligence, creativity, and cutting-edge AI aesthetics
 */
export const DefaultThemeConfig = {
  colorSchemes: {
    light: {
      palette: {
        // Explicit transparency channels: Oxygen freezes these at its own palette during the theme merge.
        primary: {
          main: '#3688FF',
          mainChannel: '54 136 255',
          dark: '#2d78e0',
          darkChannel: '45 120 224',
          light: '#6ba8f5',
          lightChannel: '107 168 245',
          contrastText: '#ffffff',
        },
        secondary: {
          main: '#5498b4',
          mainChannel: '84 152 180',
          dark: '#2d8eac',
          darkChannel: '45 142 172',
          light: '#85cde3',
          lightChannel: '133 205 227',
          contrastText: '#ffffff',
        },
        warning: {
          main: '#f59e0b',
          mainChannel: '245 158 11',
          contrastText: '#ffffff',
        },
        error: {
          main: '#ef4444',
          mainChannel: '239 68 68',
          contrastText: '#ffffff',
        },
        info: {
          main: '#8bf9fa',
          mainChannel: '139 249 250',
          contrastText: '#0a1628',
        },
        success: {
          main: '#10b981',
          mainChannel: '16 185 129',
          contrastText: '#ffffff',
        },
        background: {
          default: '#ffffff5c',
          paper: '#ffffff7f',
          acrylic: '#ffffff36',
        },
        text: {
          primary: '#181818',
          secondary: 'rgba(24, 24, 24, 0.72)',
        },
        // Explicit disabled Switch shades.
        Switch: {
          primaryDisabledColor: '#b2d1ff',
          secondaryDisabledColor: '#bed7e2',
        },
      },
    },
    dark: {
      palette: {
        // Explicit transparency channels: Oxygen freezes these at its own palette during the theme merge.
        primary: {
          main: '#3688FF',
          mainChannel: '54 136 255',
          dark: '#2d78e0',
          darkChannel: '45 120 224',
          light: '#6ba8f5',
          lightChannel: '107 168 245',
          contrastText: '#ffffff',
        },
        secondary: {
          main: '#5498b4',
          mainChannel: '84 152 180',
          dark: '#2d8eac',
          darkChannel: '45 142 172',
          light: '#85cde3',
          lightChannel: '133 205 227',
          contrastText: '#0a2230',
        },
        warning: {
          main: '#f59e0b',
          mainChannel: '245 158 11',
          contrastText: '#ffffff',
        },
        error: {
          main: '#ef4444',
          mainChannel: '239 68 68',
          contrastText: '#ffffff',
        },
        info: {
          main: '#8bf9fa',
          mainChannel: '139 249 250',
          contrastText: '#0a1628',
        },
        success: {
          main: '#10b981',
          mainChannel: '16 185 129',
          contrastText: '#ffffff',
        },
        background: {
          default: '#060d1a',
          paper: '#0a162875',
          acrylic: '#0a162875',
        },
        text: {
          primary: '#FFFFFF',
          secondary: 'rgba(255, 255, 255, 0.7)',
        },
        // Explicit disabled Switch shades.
        Switch: {
          primaryDisabledColor: '#183d72',
          secondaryDisabledColor: '#254450',
        },
      },
    },
  },
  shape: {
    borderRadius: 8,
  },
  blur: {
    none: 'none',
    light: 'blur(5px)',
    medium: 'blur(10px)',
    heavy: 'blur(15px)',
  },
  gradient: {
    primary: 'linear-gradient(90deg, #3688FF 0%, #1d5eb4 100%)',
    secondary: 'linear-gradient(90deg, #3688FF 0%, #1d5eb4 100%)',
  },
  components: {
    MuiCssBaseline: {
      styleOverrides: {
        "html[data-color-scheme='dark'] body": {
          backgroundAttachment: 'fixed',
          backgroundImage:
            'radial-gradient(circle at 15% 50%, rgb(0 136 255 / 13%) 0%, rgb(6 13 26 / 0%) 40% 70%), ' +
            'radial-gradient(circle at 65% 30%, rgb(0 127 242 / 22%) 10%, rgba(6, 13, 26, 0%) 60% 40%), ' +
            'radial-gradient(circle at center, rgba(0, 0, 0, 0.6) 0%, var(--oxygen-palette-background-default) 100%)',
          backgroundBlendMode: 'screen',
        },
        "html[data-color-scheme='light'] body": {
          backgroundAttachment: 'fixed',
          backgroundImage:
            'radial-gradient(circle at 65% 30%, rgb(0 127 242 / 18%) 10%, rgba(0, 0, 0, 0) 60% 40%), ' +
            'radial-gradient(circle at 15% 50%, rgb(0 213 255 / 26%) 1%, rgb(0 0 0 / 0%) 40% 70%), ' +
            'radial-gradient(circle at center, rgba(255, 255, 255, 0.6) 0%, var(--oxygen-palette-background-default) 100%)',
        },
      },
    },
    MuiTypography: {
      styleOverrides: {
        h5: {
          fontWeight: 500,
        },
      },
    },
    MuiPaper: {
      styleOverrides: {
        root: ({theme}: {theme: OxygenThemeType}) => ({
          backgroundColor: theme.vars.palette.background.paper,
          WebkitBackdropFilter: theme.blur.medium,
          backdropFilter: theme.blur.medium,
          backgroundImage: 'none',
          // In light mode, a Paper nested inside another Paper compounds the translucent
          // grey tint into a muddy double-tinted box, so nested surfaces get a lighter,
          // near-white tint instead of re-applying the same grey. Dark mode already reads
          // well when the tint stacks, so it's left untouched.
          "html[data-color-scheme='light'] .MuiPaper-root &:not(.MuiAlert-root)": {
            backgroundColor: 'rgba(255, 255, 255, 0.22)',
            WebkitBackdropFilter: 'none',
            backdropFilter: 'none',
            border: 'none',
          },
        }),
      },
    },
    MuiButton: {
      styleOverrides: {
        root: {
          transition: 'all 0.3s ease-in-out',
        },
        contained: ({ownerState}: {theme: OxygenThemeType; ownerState: {color?: string}}) => {
          if (ownerState.color && ownerState.color !== 'primary') {
            return {};
          }

          return {
            color: '#ffffff',
            background: 'inherit',
            '&:hover': {
              background: 'inherit',
            },
          };
        },
        containedSecondary: ({theme}: {theme: OxygenThemeType}) => ({
          '&:hover': {
            backgroundColor: theme.palette.secondary.dark,
          },
        }),
        outlined: ({theme, ownerState}: {theme: OxygenThemeType; ownerState: {color?: string}}) => {
          if (ownerState.color && ownerState.color !== 'primary') {
            return {};
          }

          return {
            color: theme.palette.primary.main,
            borderColor: theme.palette.primary.main,
            '&:hover': {
              backgroundColor: `${theme.palette.primary.main}10`,
              borderColor: theme.palette.primary.main,
              color: theme.palette.primary.main,
            },
          };
        },
        outlinedSecondary: ({theme}: {theme: OxygenThemeType}) => ({
          color: theme.palette.secondary.main,
          borderColor: theme.palette.secondary.main,
          '&:hover': {
            backgroundColor: `${theme.palette.secondary.main}10`,
            borderColor: theme.palette.secondary.main,
          },
        }),
        text: ({theme}: {theme: OxygenThemeType}) => ({
          color: theme.vars.palette.text.primary,
          '&:hover': {
            backgroundColor: `${theme.palette.primary.main}10`,
            color: theme.vars.palette.text.primary,
          },
        }),
        textSecondary: ({theme}: {theme: OxygenThemeType}) => ({
          color: theme.palette.secondary.main,
          '&:hover': {
            backgroundColor: `${theme.palette.secondary.main}10`,
          },
        }),
      },
    },
    MuiChip: {
      styleOverrides: {
        outlined: {
          borderColor: 'currentColor',
        },
      },
    },
    MuiAlert: {
      styleOverrides: {
        standardError: {
          "html[data-color-scheme='light'] &": {
            border: '1px solid rgba(239, 68, 68, 0.3)',
          },
        },
        standardWarning: {
          "html[data-color-scheme='light'] &": {
            border: '1px solid rgba(245, 158, 11, 0.3)',
          },
        },
        standardSuccess: {
          "html[data-color-scheme='light'] &": {
            border: '1px solid rgba(16, 185, 129, 0.3)',
          },
        },
        standardInfo: {
          "html[data-color-scheme='light'] &": {
            backgroundColor: '#e3f2fd',
            color: '#0d3b66',
            border: '1px solid rgba(45, 120, 224, 0.3)',
            '& .MuiAlert-icon': {
              color: '#2d78e0',
            },
          },
        },
      },
    },
    MuiLinearProgress: {
      defaultProps: {
        color: 'primary',
      },
      styleOverrides: {
        root: ({theme}: {theme: OxygenThemeType}) => ({
          '&.MuiLinearProgress-colorPrimary': {
            backgroundColor: `${theme.palette.primary.main}33`,
          },
        }),
        bar: ({theme}: {theme: OxygenThemeType}) => ({
          '&.MuiLinearProgress-barColorPrimary': {
            backgroundColor: theme.palette.primary.main,
          },
        }),
      },
    },
    MuiLink: {
      styleOverrides: {
        root: ({theme}: {theme: OxygenThemeType}) => ({
          color: theme.palette.primary.main,
          textDecoration: 'underline',
        }),
      },
    },
    MuiTextField: {
      defaultProps: {
        size: 'small',
      },
    },
    MuiFormHelperText: {
      styleOverrides: {
        root: {
          fontStyle: 'italic',
          margin: '4px 2px 0',
        },
      },
    },
    MuiOutlinedInput: {
      styleOverrides: {
        root: ({theme}: {theme: OxygenThemeType}) => ({
          // Read-only fields aren't editable, so their border should stay a plain neutral
          // divider color at all times — idle, hovered, or focused (readOnly, unlike
          // disabled, can still receive focus) — instead of MUI's default accent-colored
          // highlight. Applies in both color schemes.
          '&.MuiInputBase-readOnly .MuiOutlinedInput-notchedOutline, &.MuiInputBase-readOnly:hover .MuiOutlinedInput-notchedOutline, &.MuiInputBase-readOnly.Mui-focused .MuiOutlinedInput-notchedOutline':
            {
              borderColor: theme.vars.palette.divider,
              borderWidth: '1px',
            },
          // In light mode, a transparent input blends into the tinted card behind it and
          // reads as disabled. Give it a solid near-white fill and a firmer border so it's
          // clearly a distinct, interactive surface. Dark mode already has enough contrast
          // and is left untouched.
          "html[data-color-scheme='light'] &": {
            backgroundColor: 'rgba(255, 255, 255, 0.7)',
            '& .MuiOutlinedInput-notchedOutline': {
              borderColor: 'rgba(54, 136, 255, 0.35)',
            },
            '&:hover:not(.Mui-disabled):not(.MuiInputBase-readOnly) .MuiOutlinedInput-notchedOutline': {
              borderColor: 'rgba(54, 136, 255, 0.6)',
            },
            // Disabled and read-only are both non-editable, so they share the same muted
            // treatment: no blue tint, just a plain neutral border that doesn't invite input.
            '&.Mui-disabled, &.MuiInputBase-readOnly': {
              backgroundColor: 'rgba(255, 255, 255, 0.3)',
              '& .MuiOutlinedInput-notchedOutline': {
                borderColor: 'rgba(24, 24, 24, 0.15)',
              },
            },
          },
          // Mirror of the light-mode treatment above: give editable fields a translucent
          // black fill (instead of white) so they read as distinct surfaces against dark
          // backgrounds too, with the same disabled/read-only muting.
          "html[data-color-scheme='dark'] &": {
            backgroundColor: 'rgba(0, 0, 0, 0.25)',
            '& .MuiOutlinedInput-notchedOutline': {
              borderColor: 'rgba(54, 136, 255, 0.35)',
            },
            '&:hover:not(.Mui-disabled):not(.MuiInputBase-readOnly) .MuiOutlinedInput-notchedOutline': {
              borderColor: 'rgba(54, 136, 255, 0.6)',
            },
            '&.Mui-disabled, &.MuiInputBase-readOnly': {
              backgroundColor: 'rgba(0, 0, 0, 0.15)',
            },
          },
        }),
      },
    },
    MuiSelect: {
      defaultProps: {
        size: 'small',
      },
    },
    MuiAutocomplete: {
      defaultProps: {
        size: 'small',
      },
      styleOverrides: {
        option: {
          '&.Mui-focused, &[data-focus="true"]': {
            backgroundColor: 'var(--oxygen-palette-action-hover) !important',
          },
          '&[aria-selected="true"]': {
            backgroundColor: 'var(--oxygen-palette-action-selected) !important',
          },
          '&[aria-selected="true"].Mui-focused, &[aria-selected="true"][data-focus="true"]': {
            backgroundColor: 'var(--oxygen-palette-action-selected) !important',
          },
        },
      },
    },
    MuiDataGrid: {
      styleOverrides: {
        panelContent: {
          "html[data-color-scheme='dark'] &": {
            '--DataGrid-t-color-background-overlay': '#091522f0',
          },
          "html[data-color-scheme='light'] &": {
            '--DataGrid-t-color-background-overlay': '#d6dce3eb',
          },
        },
      },
    },
    MuiPopover: {
      styleOverrides: {
        paper: ({theme}: {theme: OxygenThemeType}) => ({
          backgroundColor: theme.vars.palette.background.paper,
          WebkitBackdropFilter: theme.blur.medium,
          backdropFilter: theme.blur.medium,
          backgroundImage: 'none',
        }),
      },
    },
    MuiDialog: {
      styleOverrides: {
        root: ({theme}: {theme: OxygenThemeType}) => ({
          '& .MuiBackdrop-root': {
            WebkitBackdropFilter: theme.blur.light,
            backdropFilter: theme.blur.light,
          },
        }),
        paper: ({theme}: {theme: OxygenThemeType}) => ({
          backgroundColor: theme.vars.palette.background.default,
          WebkitBackdropFilter: 'none',
          backdropFilter: 'none',
        }),
      },
    },
  },
};

const DefaultTheme = createOxygenTheme(DefaultThemeConfig, OxygenTheme);

export default DefaultTheme;
