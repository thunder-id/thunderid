/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

import {Theme, ThemeConfig, ThemeImage, ThemeMode, ThemeVars} from './types';
import VendorConstants from '../constants/VendorConstants';
import {RecursivePartial} from '../models/utility-types';

const lightTheme: ThemeConfig = {
  borderRadius: {
    large: '16px',
    medium: '8px',
    small: '4px',
  },
  colors: {
    action: {
      activatedOpacity: 0.12,
      active: 'rgba(0, 0, 0, 0.54)',
      disabled: 'rgba(0, 0, 0, 0.26)',
      disabledBackground: 'rgba(0, 0, 0, 0.12)',
      disabledOpacity: 0.38,
      focus: 'rgba(0, 0, 0, 0.12)',
      focusOpacity: 0.12,
      hover: 'rgba(0, 0, 0, 0.04)',
      hoverOpacity: 0.04,
      selected: 'rgba(0, 0, 0, 0.08)',
      selectedOpacity: 0.08,
    },
    background: {
      body: {
        dark: '#212121',
        main: '#1a1a1a',
      },
      dark: '#212121',
      disabled: '#f0f0f0',
      surface: '#ffffff',
    },
    border: '#e0e0e0',
    error: {
      contrastText: '#d52828',
      dark: '#b71c1c',
      light: '#fef2f2',
      main: '#d32f2f',
    },
    info: {
      contrastText: '#43aeda',
      dark: '#01579b',
      light: '#eff6ff',
      main: '#bbebff',
    },
    primary: {
      contrastText: '#ffffff',
      dark: '#174ea6',
      main: '#1a73e8',
    },
    secondary: {
      contrastText: '#ffffff',
      dark: '#212121',
      light: '#f3f4f6',
      main: '#424242',
    },
    success: {
      contrastText: '#00a807',
      dark: '#388e3c',
      light: '#f0fdf4',
      main: '#4caf50',
    },
    text: {
      dark: '#212121',
      primary: '#1a1a1a',
      secondary: '#666666',
    },
    warning: {
      contrastText: '#be7100',
      dark: '#f57c00',
      light: '#fffbeb',
      main: '#ff9800',
    },
  },
  images: {
    favicon: {},
    logo: {},
  },
  shadows: {
    large: '0 8px 32px rgba(0, 0, 0, 0.2)',
    medium: '0 4px 16px rgba(0, 0, 0, 0.15)',
    small: '0 2px 8px rgba(0, 0, 0, 0.1)',
  },
  spacing: {
    unit: 8,
  },
  typography: {
    fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif',
    fontSizes: {
      '2xl': '1.5rem', // 24px
      '3xl': '2.125rem', // 34px
      lg: '1.125rem', // 18px
      md: '1rem', // 16px
      sm: '0.875rem', // 14px
      xl: '1.25rem', // 20px
      xs: '0.75rem', // 12px
    },
    fontWeights: {
      bold: 700,
      medium: 500,
      normal: 400,
      semibold: 600,
    },
    lineHeights: {
      normal: 1.4,
      relaxed: 1.6,
      tight: 1.2,
    },
  },
};

const darkTheme: ThemeConfig = {
  borderRadius: {
    large: '16px',
    medium: '8px',
    small: '4px',
  },
  colors: {
    action: {
      activatedOpacity: 0.12,
      active: '#1c1c1c',
      disabled: 'rgba(255, 255, 255, 0.26)',
      disabledBackground: 'rgba(255, 255, 255, 0.12)',
      disabledOpacity: 0.38,
      focus: '#1c1c1c',
      focusOpacity: 0.12,
      hover: '#1c1c1c',
      hoverOpacity: 0.04,
      selected: '#1c1c1c',
      selectedOpacity: 0.08,
    },
    background: {
      body: {
        dark: '#212121',
        main: '#ffffff',
      },
      dark: '#212121',
      disabled: '#1f1f1f',
      surface: '#121212',
    },
    border: '#404040',
    error: {
      contrastText: '#d52828',
      dark: '#b71c1c',
      light: '#2d1515',
      main: '#d32f2f',
    },
    info: {
      contrastText: '#43aeda',
      dark: '#01579b',
      light: '#0f1f35',
      main: '#bbebff',
    },
    primary: {
      contrastText: '#ffffff',
      dark: '#174ea6',
      main: '#1a73e8',
    },
    secondary: {
      contrastText: '#ffffff',
      dark: '#212121',
      light: '#2a2a2a',
      main: '#8b8b8b',
    },
    success: {
      contrastText: '#00a807',
      dark: '#388e3c',
      light: '#132d1a',
      main: '#4caf50',
    },
    text: {
      dark: '#212121',
      primary: '#ffffff',
      secondary: '#b3b3b3',
    },
    warning: {
      contrastText: '#be7100',
      dark: '#f57c00',
      light: '#2d2310',
      main: '#ff9800',
    },
  },
  images: {
    favicon: {},
    logo: {},
  },
  shadows: {
    large: '0 8px 32px rgba(0, 0, 0, 0.5)',
    medium: '0 4px 16px rgba(0, 0, 0, 0.4)',
    small: '0 2px 8px rgba(0, 0, 0, 0.3)',
  },
  spacing: {
    unit: 8,
  },
  typography: {
    fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif',
    fontSizes: {
      '2xl': '1.5rem', // 24px
      '3xl': '2.125rem', // 34px
      lg: '1.125rem', // 18px
      md: '1rem', // 16px
      sm: '0.875rem', // 14px
      xl: '1.25rem', // 20px
      xs: '0.75rem', // 12px
    },
    fontWeights: {
      bold: 700,
      medium: 500,
      normal: 400,
      semibold: 600,
    },
    lineHeights: {
      normal: 1.4,
      relaxed: 1.6,
      tight: 1.2,
    },
  },
};

const toCssVariables = (theme: ThemeConfig): Record<string, string> => {
  const cssVars: Record<string, string> = {};
  const prefix: string = theme.cssVarPrefix || VendorConstants.VENDOR_PREFIX;

  // Colors - Action
  if (theme.colors?.action?.active) {
    cssVars[`--${prefix}-color-action-active`] = theme.colors.action.active;
  }
  if (theme.colors?.action?.hover) {
    cssVars[`--${prefix}-color-action-hover`] = theme.colors.action.hover;
  }
  if (theme.colors?.action?.hoverOpacity !== undefined) {
    cssVars[`--${prefix}-color-action-hoverOpacity`] = theme.colors.action.hoverOpacity.toString();
  }
  if (theme.colors?.action?.selected) {
    cssVars[`--${prefix}-color-action-selected`] = theme.colors.action.selected;
  }
  if (theme.colors?.action?.selectedOpacity !== undefined) {
    cssVars[`--${prefix}-color-action-selectedOpacity`] = theme.colors.action.selectedOpacity.toString();
  }
  if (theme.colors?.action?.disabled) {
    cssVars[`--${prefix}-color-action-disabled`] = theme.colors.action.disabled;
  }
  if (theme.colors?.action?.disabledBackground) {
    cssVars[`--${prefix}-color-action-disabledBackground`] = theme.colors.action.disabledBackground;
  }
  if (theme.colors?.action?.disabledOpacity !== undefined) {
    cssVars[`--${prefix}-color-action-disabledOpacity`] = theme.colors.action.disabledOpacity.toString();
  }
  if (theme.colors?.action?.focus) {
    cssVars[`--${prefix}-color-action-focus`] = theme.colors.action.focus;
  }
  if (theme.colors?.action?.focusOpacity !== undefined) {
    cssVars[`--${prefix}-color-action-focusOpacity`] = theme.colors.action.focusOpacity.toString();
  }
  if (theme.colors?.action?.activatedOpacity !== undefined) {
    cssVars[`--${prefix}-color-action-activatedOpacity`] = theme.colors.action.activatedOpacity.toString();
  }

  // Colors - Primary
  if (theme.colors?.primary?.main) {
    cssVars[`--${prefix}-color-primary-main`] = theme.colors.primary.main;
  }
  if (theme.colors?.primary?.contrastText) {
    cssVars[`--${prefix}-color-primary-contrastText`] = theme.colors.primary.contrastText;
  }

  // Colors - Secondary
  if (theme.colors?.secondary?.main) {
    cssVars[`--${prefix}-color-secondary-main`] = theme.colors.secondary.main;
  }
  if (theme.colors?.secondary?.contrastText) {
    cssVars[`--${prefix}-color-secondary-contrastText`] = theme.colors.secondary.contrastText;
  }
  if (theme.colors?.secondary?.light) {
    cssVars[`--${prefix}-color-secondary-light`] = theme.colors.secondary.light;
  }

  // Colors - Background
  if (theme.colors?.background?.surface) {
    cssVars[`--${prefix}-color-background-surface`] = theme.colors.background.surface;
  }
  if (theme.colors?.background?.disabled) {
    cssVars[`--${prefix}-color-background-disabled`] = theme.colors.background.disabled;
  }
  if (theme.colors?.background?.body?.main) {
    cssVars[`--${prefix}-color-background-body-main`] = theme.colors.background.body.main;
  }

  // Colors - Error
  if (theme.colors?.error?.main) {
    cssVars[`--${prefix}-color-error-main`] = theme.colors.error.main;
  }
  if (theme.colors?.error?.contrastText) {
    cssVars[`--${prefix}-color-error-contrastText`] = theme.colors.error.contrastText;
  }
  if (theme.colors?.error?.light) {
    cssVars[`--${prefix}-color-error-light`] = theme.colors.error.light;
  }

  // Colors - Success
  if (theme.colors?.success?.main) {
    cssVars[`--${prefix}-color-success-main`] = theme.colors.success.main;
  }
  if (theme.colors?.success?.contrastText) {
    cssVars[`--${prefix}-color-success-contrastText`] = theme.colors.success.contrastText;
  }
  if (theme.colors?.success?.light) {
    cssVars[`--${prefix}-color-success-light`] = theme.colors.success.light;
  }

  // Colors - Warning
  if (theme.colors?.warning?.main) {
    cssVars[`--${prefix}-color-warning-main`] = theme.colors.warning.main;
  }
  if (theme.colors?.warning?.contrastText) {
    cssVars[`--${prefix}-color-warning-contrastText`] = theme.colors.warning.contrastText;
  }
  if (theme.colors?.warning?.light) {
    cssVars[`--${prefix}-color-warning-light`] = theme.colors.warning.light;
  }

  // Colors - Info
  if (theme.colors?.info?.main) {
    cssVars[`--${prefix}-color-info-main`] = theme.colors.info.main;
  }
  if (theme.colors?.info?.contrastText) {
    cssVars[`--${prefix}-color-info-contrastText`] = theme.colors.info.contrastText;
  }
  if (theme.colors?.info?.light) {
    cssVars[`--${prefix}-color-info-light`] = theme.colors.info.light;
  }

  // Colors - Text
  if (theme.colors?.text?.primary) {
    cssVars[`--${prefix}-color-text-primary`] = theme.colors.text.primary;
  }
  if (theme.colors?.text?.secondary) {
    cssVars[`--${prefix}-color-text-secondary`] = theme.colors.text.secondary;
  }

  // Colors - Border
  if (theme.colors?.border) {
    cssVars[`--${prefix}-color-border`] = theme.colors.border;
  }

  // Spacing
  if (theme.spacing?.unit !== undefined) {
    cssVars[`--${prefix}-spacing-unit`] = `${theme.spacing.unit}px`;
  }

  // Border Radius
  if (theme.borderRadius?.small) {
    cssVars[`--${prefix}-border-radius-small`] = theme.borderRadius.small;
  }
  if (theme.borderRadius?.medium) {
    cssVars[`--${prefix}-border-radius-medium`] = theme.borderRadius.medium;
  }
  if (theme.borderRadius?.large) {
    cssVars[`--${prefix}-border-radius-large`] = theme.borderRadius.large;
  }

  // Shadows
  if (theme.shadows?.small) {
    cssVars[`--${prefix}-shadow-small`] = theme.shadows.small;
  }
  if (theme.shadows?.medium) {
    cssVars[`--${prefix}-shadow-medium`] = theme.shadows.medium;
  }
  if (theme.shadows?.large) {
    cssVars[`--${prefix}-shadow-large`] = theme.shadows.large;
  }

  // Typography - Font Family
  if (theme.typography?.fontFamily) {
    cssVars[`--${prefix}-typography-fontFamily`] = theme.typography.fontFamily;
  }

  // Typography - Font Sizes
  if (theme.typography?.fontSizes?.xs) {
    cssVars[`--${prefix}-typography-fontSize-xs`] = theme.typography.fontSizes.xs;
  }
  if (theme.typography?.fontSizes?.sm) {
    cssVars[`--${prefix}-typography-fontSize-sm`] = theme.typography.fontSizes.sm;
  }
  if (theme.typography?.fontSizes?.md) {
    cssVars[`--${prefix}-typography-fontSize-md`] = theme.typography.fontSizes.md;
  }
  if (theme.typography?.fontSizes?.lg) {
    cssVars[`--${prefix}-typography-fontSize-lg`] = theme.typography.fontSizes.lg;
  }
  if (theme.typography?.fontSizes?.xl) {
    cssVars[`--${prefix}-typography-fontSize-xl`] = theme.typography.fontSizes.xl;
  }
  if (theme.typography?.fontSizes?.['2xl']) {
    cssVars[`--${prefix}-typography-fontSize-2xl`] = theme.typography.fontSizes['2xl'];
  }
  if (theme.typography?.fontSizes?.['3xl']) {
    cssVars[`--${prefix}-typography-fontSize-3xl`] = theme.typography.fontSizes['3xl'];
  }

  // Typography - Font Weights
  if (theme.typography?.fontWeights?.normal !== undefined) {
    cssVars[`--${prefix}-typography-fontWeight-normal`] = theme.typography.fontWeights.normal.toString();
  }
  if (theme.typography?.fontWeights?.medium !== undefined) {
    cssVars[`--${prefix}-typography-fontWeight-medium`] = theme.typography.fontWeights.medium.toString();
  }
  if (theme.typography?.fontWeights?.semibold !== undefined) {
    cssVars[`--${prefix}-typography-fontWeight-semibold`] = theme.typography.fontWeights.semibold.toString();
  }
  if (theme.typography?.fontWeights?.bold !== undefined) {
    cssVars[`--${prefix}-typography-fontWeight-bold`] = theme.typography.fontWeights.bold.toString();
  }

  // Typography - Line Heights
  if (theme.typography?.lineHeights?.tight !== undefined) {
    cssVars[`--${prefix}-typography-lineHeight-tight`] = theme.typography.lineHeights.tight.toString();
  }
  if (theme.typography?.lineHeights?.normal !== undefined) {
    cssVars[`--${prefix}-typography-lineHeight-normal`] = theme.typography.lineHeights.normal.toString();
  }
  if (theme.typography?.lineHeights?.relaxed !== undefined) {
    cssVars[`--${prefix}-typography-lineHeight-relaxed`] = theme.typography.lineHeights.relaxed.toString();
  }

  // Images
  if (theme.images) {
    Object.keys(theme.images).forEach((imageKey: string) => {
      const imageConfig: ThemeImage | undefined = theme.images[imageKey];
      if (imageConfig?.url) {
        cssVars[`--${prefix}-image-${imageKey}-url`] = imageConfig.url;
      }
      if (imageConfig?.title) {
        cssVars[`--${prefix}-image-${imageKey}-title`] = imageConfig.title;
      }
      if (imageConfig?.alt) {
        cssVars[`--${prefix}-image-${imageKey}-alt`] = imageConfig.alt;
      }
    });
  }

  /* |---------------------------------------------------------------| */
  /* |                       Components                              | */
  /* |---------------------------------------------------------------| */

  // Button Overrides
  if (theme.components?.Button?.styleOverrides?.root?.borderRadius) {
    cssVars[`--${prefix}-component-button-root-borderRadius`] =
      theme.components.Button.styleOverrides.root.borderRadius;
  }

  // Field Overrides (Parent of `TextField`, `DatePicker`, `OtpField`, `Select`, etc.)
  if (theme.components?.Field?.styleOverrides?.root?.borderRadius) {
    cssVars[`--${prefix}-component-field-root-borderRadius`] = theme.components.Field.styleOverrides.root.borderRadius;
  }

  return cssVars;
};

const toThemeVars = (theme: ThemeConfig): ThemeVars => {
  const prefix: string = theme.cssVarPrefix || VendorConstants.VENDOR_PREFIX;

  const componentVars: ThemeVars['components'] = {};
  if (theme.components?.Button?.styleOverrides?.root?.borderRadius) {
    componentVars.Button = {
      root: {
        borderRadius: `var(--${prefix}-component-button-root-borderRadius)`,
      },
    };
  }
  if (theme.components?.Field?.styleOverrides?.root?.borderRadius) {
    componentVars.Field = {
      root: {
        borderRadius: `var(--${prefix}-component-field-root-borderRadius)`,
      },
    };
  }

  const themeVars: ThemeVars = {
    borderRadius: {
      large: `var(--${prefix}-border-radius-large)`,
      medium: `var(--${prefix}-border-radius-medium)`,
      small: `var(--${prefix}-border-radius-small)`,
    },
    colors: {
      action: {
        activatedOpacity: `var(--${prefix}-color-action-activatedOpacity)`,
        active: `var(--${prefix}-color-action-active)`,
        disabled: `var(--${prefix}-color-action-disabled)`,
        disabledBackground: `var(--${prefix}-color-action-disabledBackground)`,
        disabledOpacity: `var(--${prefix}-color-action-disabledOpacity)`,
        focus: `var(--${prefix}-color-action-focus)`,
        focusOpacity: `var(--${prefix}-color-action-focusOpacity)`,
        hover: `var(--${prefix}-color-action-hover)`,
        hoverOpacity: `var(--${prefix}-color-action-hoverOpacity)`,
        selected: `var(--${prefix}-color-action-selected)`,
        selectedOpacity: `var(--${prefix}-color-action-selectedOpacity)`,
      },
      background: {
        body: {
          main: `var(--${prefix}-color-background-body-main)`,
        },
        disabled: `var(--${prefix}-color-background-disabled)`,
        surface: `var(--${prefix}-color-background-surface)`,
      },
      border: `var(--${prefix}-color-border)`,
      error: {
        contrastText: `var(--${prefix}-color-error-contrastText)`,
        main: `var(--${prefix}-color-error-main)`,
      },
      info: {
        contrastText: `var(--${prefix}-color-info-contrastText)`,
        main: `var(--${prefix}-color-info-main)`,
      },
      primary: {
        contrastText: `var(--${prefix}-color-primary-contrastText)`,
        main: `var(--${prefix}-color-primary-main)`,
      },
      secondary: {
        contrastText: `var(--${prefix}-color-secondary-contrastText)`,
        main: `var(--${prefix}-color-secondary-main)`,
      },
      success: {
        contrastText: `var(--${prefix}-color-success-contrastText)`,
        main: `var(--${prefix}-color-success-main)`,
      },
      text: {
        primary: `var(--${prefix}-color-text-primary)`,
        secondary: `var(--${prefix}-color-text-secondary)`,
      },
      warning: {
        contrastText: `var(--${prefix}-color-warning-contrastText)`,
        main: `var(--${prefix}-color-warning-main)`,
      },
    },
    shadows: {
      large: `var(--${prefix}-shadow-large)`,
      medium: `var(--${prefix}-shadow-medium)`,
      small: `var(--${prefix}-shadow-small)`,
    },
    spacing: {
      unit: `var(--${prefix}-spacing-unit)`,
    },
    typography: {
      fontFamily: `var(--${prefix}-typography-fontFamily)`,
      fontSizes: {
        '2xl': `var(--${prefix}-typography-fontSize-2xl)`,
        '3xl': `var(--${prefix}-typography-fontSize-3xl)`,
        lg: `var(--${prefix}-typography-fontSize-lg)`,
        md: `var(--${prefix}-typography-fontSize-md)`,
        sm: `var(--${prefix}-typography-fontSize-sm)`,
        xl: `var(--${prefix}-typography-fontSize-xl)`,
        xs: `var(--${prefix}-typography-fontSize-xs)`,
      },
      fontWeights: {
        bold: `var(--${prefix}-typography-fontWeight-bold)`,
        medium: `var(--${prefix}-typography-fontWeight-medium)`,
        normal: `var(--${prefix}-typography-fontWeight-normal)`,
        semibold: `var(--${prefix}-typography-fontWeight-semibold)`,
      },
      lineHeights: {
        normal: `var(--${prefix}-typography-lineHeight-normal)`,
        relaxed: `var(--${prefix}-typography-lineHeight-relaxed)`,
        tight: `var(--${prefix}-typography-lineHeight-tight)`,
      },
    },
  };

  // Add images if they exist
  if (theme.images) {
    themeVars.images = {};
    Object.keys(theme.images).forEach((imageKey: string) => {
      const imageConfig: ThemeImage | undefined = theme.images[imageKey];
      themeVars.images[imageKey] = {
        alt: imageConfig?.alt ? `var(--${prefix}-image-${imageKey}-alt)` : undefined,
        title: imageConfig?.title ? `var(--${prefix}-image-${imageKey}-title)` : undefined,
        url: imageConfig?.url ? `var(--${prefix}-image-${imageKey}-url)` : undefined,
      };
    });
  }

  if (Object.keys(componentVars).length > 0) {
    themeVars.components = componentVars;
  }

  return themeVars;
};

const createTheme = (config: RecursivePartial<ThemeConfig> = {}, isDark = false): Theme => {
  const baseTheme: ThemeConfig = isDark ? darkTheme : lightTheme;

  const mergedConfig: ThemeConfig = {
    ...baseTheme,
    ...config,
    borderRadius: {
      ...baseTheme.borderRadius,
      ...config.borderRadius,
    },
    colors: {
      ...baseTheme.colors,
      ...config.colors,
      action: {
        ...baseTheme.colors.action,
        ...(config.colors?.action || {}),
      },
      secondary: {
        ...baseTheme.colors.secondary,
        ...(config.colors?.secondary || {}),
      },
    },
    images: {
      ...baseTheme.images,
      ...config.images,
    },
    shadows: {
      ...baseTheme.shadows,
      ...config.shadows,
    },
    spacing: {
      ...baseTheme.spacing,
      ...config.spacing,
    },
    typography: {
      ...baseTheme.typography,
      ...config.typography,
      fontSizes: {
        ...baseTheme.typography.fontSizes,
        ...(config.typography?.fontSizes || {}),
      },
      fontWeights: {
        ...baseTheme.typography.fontWeights,
        ...(config.typography?.fontWeights || {}),
      },
      lineHeights: {
        ...baseTheme.typography.lineHeights,
        ...(config.typography?.lineHeights || {}),
      },
    },
  } as ThemeConfig;

  return {
    ...mergedConfig,
    cssVariables: toCssVariables(mergedConfig),
    vars: toThemeVars(mergedConfig),
  };
};

export const DEFAULT_THEME: ThemeMode = 'light';

export default createTheme;
