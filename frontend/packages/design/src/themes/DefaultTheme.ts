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
        primary: {
          main: '#3688FF',
          dark: '#2d78e0',
          light: '#6ba8f5',
          contrastText: '#ffffff',
        },
        secondary: {
          main: '#5498b4',
          dark: '#2d8eac',
          light: '#85cde3',
          contrastText: '#ffffff',
        },
        warning: {
          main: '#f59e0b',
          contrastText: '#ffffff',
        },
        error: {
          main: '#ef4444',
          contrastText: '#ffffff',
        },
        info: {
          main: '#8bf9fa',
          contrastText: '#0a1628',
        },
        success: {
          main: '#10b981',
          contrastText: '#ffffff',
        },
        background: {
          default: '#ffffff',
          paper: '#bfc6cf33',
          acrylic: '#c8d1dc1f',
        },
        text: {
          primary: '#181818',
          secondary: 'rgba(24, 24, 24, 0.6)',
        },
      },
    },
    dark: {
      palette: {
        primary: {
          main: '#3688FF',
          dark: '#2d78e0',
          light: '#6ba8f5',
          contrastText: '#ffffff',
        },
        secondary: {
          main: '#5498b4',
          dark: '#2d8eac',
          light: '#85cde3',
          contrastText: '#0a2230',
        },
        warning: {
          main: '#f59e0b',
          contrastText: '#ffffff',
        },
        error: {
          main: '#ef4444',
          contrastText: '#ffffff',
        },
        info: {
          main: '#8bf9fa',
          contrastText: '#0a1628',
        },
        success: {
          main: '#10b981',
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
            'radial-gradient(circle at 65% 30%, rgb(0 127 242 / 8%) 10%, rgba(0, 0, 0, 0) 60% 40%), ' +
            'radial-gradient(circle at 15% 50%, rgb(0 213 255 / 12%) 1%, rgb(0 0 0 / 0%) 40% 70%), ' +
            'radial-gradient(circle at center, rgba(255, 255, 255, 0.6) 0%, var(--oxygen-palette-background-default) 100%)',
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
