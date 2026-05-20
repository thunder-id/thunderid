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

export interface ThemeTypography {
  fontFamily: string;
  fontSizes: {
    '2xl': string;
    '3xl': string;
    lg: string;
    md: string;
    sm: string;
    xl: string;
    xs: string;
  };
  fontWeights: {
    bold: number;
    medium: number;
    normal: number;
    semibold: number;
  };
  lineHeights: {
    normal: number;
    relaxed: number;
    tight: number;
  };
}

export interface ThemeColors {
  action: {
    activatedOpacity: number;
    active: string;
    disabled: string;
    disabledBackground: string;
    disabledOpacity: number;
    focus: string;
    focusOpacity: number;
    hover: string;
    hoverOpacity: number;
    selected: string;
    selectedOpacity: number;
  };
  background: {
    body: {
      dark?: string;
      main: string;
    };
    dark?: string;
    disabled: string;
    surface: string;
  };
  border: string;
  error: {
    contrastText: string;
    dark?: string;
    light?: string;
    main: string;
  };
  info: {
    contrastText: string;
    dark?: string;
    light?: string;
    main: string;
  };
  primary: {
    contrastText: string;
    dark?: string;
    main: string;
  };
  secondary: {
    contrastText: string;
    dark?: string;
    light?: string;
    main: string;
  };
  success: {
    contrastText: string;
    dark?: string;
    light?: string;
    main: string;
  };
  text: {
    dark?: string;
    primary: string;
    secondary: string;
  };
  warning: {
    contrastText: string;
    dark?: string;
    light?: string;
    main: string;
  };
}

export interface ThemeComponentStyleOverrides {
  [slot: string]: Record<string, any> | undefined;
  /**
   * Style overrides for the root element or slots.
   * Example: { root: { borderRadius: '8px' } }
   */
  root?: Record<string, any>;
}

export interface ThemeComponents {
  [componentName: string]:
    | {
        defaultProps?: Record<string, any>;
        styleOverrides?: ThemeComponentStyleOverrides;
        variants?: Record<string, any>[];
      }
    | undefined;
  Button?: {
    defaultProps?: Record<string, any>;
    styleOverrides?: {
      [slot: string]: Record<string, any> | undefined;
      root?: {
        [key: string]: any;
        borderRadius?: string;
      };
    };
    variants?: Record<string, any>[];
  };
  Field?: {
    defaultProps?: Record<string, any>;
    styleOverrides?: {
      [slot: string]: Record<string, any> | undefined;
      root?: {
        [key: string]: any;
        borderRadius?: string;
      };
    };
    variants?: Record<string, any>[];
  };
}

export interface ThemeConfig {
  borderRadius: {
    large: string;
    medium: string;
    small: string;
  };
  colors: ThemeColors;
  /**
   * Component style overrides
   */
  components?: ThemeComponents;
  /**
   * The prefix used for CSS variables.
   * @default 'thunderid' (from VendorConstants.VENDOR_PREFIX)
   */
  cssVarPrefix?: string;
  /**
   * The text direction for the UI.
   * @default 'ltr'
   */
  direction?: 'ltr' | 'rtl';
  /**
   * Image assets configuration
   */
  images?: ThemeImages;
  shadows: {
    large: string;
    medium: string;
    small: string;
  };
  spacing: {
    unit: number;
  };
  typography: {
    fontFamily: string;
    fontSizes: {
      '2xl': string;
      '3xl': string;
      lg: string;
      md: string;
      sm: string;
      xl: string;
      xs: string;
    };
    fontWeights: {
      bold: number;
      medium: number;
      normal: number;
      semibold: number;
    };
    lineHeights: {
      normal: number;
      relaxed: number;
      tight: number;
    };
  };
}

export interface ThemeComponentVars {
  [componentName: string]: Record<string, Record<string, any> | undefined> | undefined;
  Button?: {
    [slot: string]: Record<string, any> | undefined;
    root?: {
      [key: string]: any;
      borderRadius?: string;
    };
  };
  Field?: {
    [slot: string]: Record<string, any> | undefined;
    root?: {
      [key: string]: any;
      borderRadius?: string;
    };
  };
}

export interface ThemeVars {
  borderRadius: {
    large: string;
    medium: string;
    small: string;
  };
  colors: {
    action: {
      activatedOpacity: string;
      active: string;
      disabled: string;
      disabledBackground: string;
      disabledOpacity: string;
      focus: string;
      focusOpacity: string;
      hover: string;
      hoverOpacity: string;
      selected: string;
      selectedOpacity: string;
    };
    background: {
      body: {
        dark?: string;
        main: string;
      };
      dark?: string;
      disabled: string;
      surface: string;
    };
    border: string;
    error: {
      contrastText: string;
      dark?: string;
      main: string;
    };
    info: {
      contrastText: string;
      dark?: string;
      main: string;
    };
    primary: {
      contrastText: string;
      dark?: string;
      main: string;
    };
    secondary: {
      contrastText: string;
      dark?: string;
      main: string;
    };
    success: {
      contrastText: string;
      dark?: string;
      main: string;
    };
    text: {
      dark?: string;
      primary: string;
      secondary: string;
    };
    warning: {
      contrastText: string;
      dark?: string;
      main: string;
    };
  };
  /**
   * Component CSS variable references (e.g., for overrides)
   */
  components?: ThemeComponentVars;
  images?: {
    [key: string]:
      | {
          alt?: string;
          title?: string;
          url?: string;
        }
      | undefined;
    favicon?: {
      alt?: string;
      title?: string;
      url?: string;
    };
    logo?: {
      alt?: string;
      title?: string;
      url?: string;
    };
  };
  shadows: {
    large: string;
    medium: string;
    small: string;
  };
  spacing: {
    unit: string;
  };
  typography: {
    fontFamily: string;
    fontSizes: {
      '2xl': string;
      '3xl': string;
      lg: string;
      md: string;
      sm: string;
      xl: string;
      xs: string;
    };
    fontWeights: {
      bold: string;
      medium: string;
      normal: string;
      semibold: string;
    };
    lineHeights: {
      normal: string;
      relaxed: string;
      tight: string;
    };
  };
}

export interface Theme extends ThemeConfig {
  cssVariables: Record<string, string>;
  vars: ThemeVars;
}

export type ThemeMode = 'light' | 'dark' | 'system' | 'class';

export interface ThemeDetection {
  /**
   * The CSS class name to detect for dark mode (without the dot)
   * @default 'dark'
   */
  darkClass?: string;
  /**
   * The CSS class name to detect for light mode (without the dot)
   * @default 'light'
   */
  lightClass?: string;
}

export interface ThemeImage {
  /**
   * Alternative text for accessibility
   */
  alt?: string;
  /**
   * The title/alt text for the image
   */
  title?: string;
  /**
   * The URL of the image
   */
  url?: string;
}

export interface ThemeImages {
  /**
   * Allow for additional custom images
   */
  [key: string]: ThemeImage | undefined;
  /**
   * Favicon configuration
   */
  favicon?: ThemeImage;
  /**
   * Logo configuration
   */
  logo?: ThemeImage;
}
