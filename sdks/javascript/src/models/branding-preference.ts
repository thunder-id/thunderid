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

/**
 * Interface for color configuration with multiple variants.
 */
export interface ColorVariants {
  contrastText?: string;
  dark?: string;
  inverted?: string;
  light?: string;
  main?: string;
}

/**
 * Interface for text color configuration.
 */
export interface TextColors {
  primary?: string;
  secondary?: string;
}

/**
 * Interface for button styling configuration.
 */
export interface ButtonStyle {
  base?: {
    background?: {
      backgroundColor?: string;
    };
    border?: {
      borderColor?: string;
      borderRadius?: string;
    };
    font?: {
      color?: string;
    };
  };
}

/**
 * Interface for buttons configuration.
 */
export interface ButtonsConfig {
  externalConnection?: ButtonStyle;
  primary?: ButtonStyle;
  secondary?: ButtonStyle;
}

/**
 * Interface for color palette configuration.
 */
export interface ColorsConfig {
  alerts?: {
    error?: ColorVariants;
    info?: ColorVariants;
    neutral?: ColorVariants;
    warning?: ColorVariants;
  };
  background?: {
    body?: ColorVariants;
    surface?: ColorVariants;
  };
  illustrations?: {
    accent1?: ColorVariants;
    accent2?: ColorVariants;
    accent3?: ColorVariants;
    primary?: ColorVariants;
    secondary?: ColorVariants;
  };
  outlined?: {
    default?: string;
  };
  primary?: ColorVariants;
  secondary?: ColorVariants;
  text?: TextColors;
}

/**
 * Interface for footer configuration.
 */
export interface FooterConfig {
  border?: {
    borderColor?: string;
  };
  font?: {
    color?: string;
  };
}

/**
 * Interface for image configuration.
 */
export interface ImageConfig {
  altText?: string;
  imgURL?: string;
  title?: string;
}

/**
 * Interface for images configuration.
 */
export interface ImagesConfig {
  favicon?: Partial<ImageConfig>;
  logo?: Partial<ImageConfig>;
  myAccountLogo?: Partial<ImageConfig>;
}

/**
 * Interface for input styling configuration.
 */
export interface InputsConfig {
  base?: {
    background?: {
      backgroundColor?: string;
    };
    border?: {
      borderColor?: string;
      borderRadius?: string;
    };
    font?: {
      color?: string;
    };
    labels?: {
      font?: {
        color?: string;
      };
    };
  };
}

/**
 * Interface for login box configuration.
 */
export interface LoginBoxConfig {
  background?: {
    backgroundColor?: string;
  };
  border?: {
    borderColor?: string;
    borderRadius?: string;
    borderWidth?: string;
  };
  font?: {
    color?: string;
  };
}

/**
 * Interface for login page configuration.
 */
export interface LoginPageConfig {
  background?: {
    backgroundColor?: string;
  };
  font?: {
    color?: string;
  };
}

/**
 * Interface for typography configuration.
 */
export interface TypographyConfig {
  font?: {
    color?: string;
    fontFamily?: string;
    importURL?: string;
  };
  heading?: {
    font?: {
      color?: string;
    };
  };
}

/**
 * Interface for theme variant configuration (LIGHT/DARK).
 */
export interface ThemeVariant {
  buttons?: ButtonsConfig;
  colors?: ColorsConfig;
  footer?: FooterConfig;
  images?: ImagesConfig;
  inputs?: InputsConfig;
  loginBox?: LoginBoxConfig;
  loginPage?: LoginPageConfig;
  typography?: TypographyConfig;
}

/**
 * Interface for branding preference layout configuration.
 */
export interface BrandingLayout {
  activeLayout?: string;
  sideImg?: {
    altText?: string;
    imgURL?: string;
  };
}

/**
 * Interface for organization details configuration.
 */
export interface BrandingOrganizationDetails {
  displayName?: string;
  supportEmail?: string;
}

/**
 * Interface for URL configurations.
 */
export interface UrlsConfig {
  cookiePolicyURL?: string;
  privacyPolicyURL?: string;
  selfSignUpURL?: string;
  termsOfUseURL?: string;
}

/**
 * Interface for branding preference theme configuration.
 */
export interface BrandingTheme {
  DARK?: ThemeVariant;
  LIGHT?: ThemeVariant;
  activeTheme?: string;
}

/**
 * Interface for branding preference configuration.
 */
export interface BrandingPreferenceConfig {
  configs?: {
    isBrandingEnabled?: boolean;
    removeDefaultBranding?: boolean;
    selfSignUpEnabled?: boolean;
  };
  layout?: BrandingLayout;
  organizationDetails?: BrandingOrganizationDetails;
  theme?: BrandingTheme;
  urls?: UrlsConfig;
}

/**
 * Interface for branding preference configuration.
 */
export interface BrandingPreference {
  locale?: string;
  name?: string;
  preference?: BrandingPreferenceConfig;
  type?: string;
}
