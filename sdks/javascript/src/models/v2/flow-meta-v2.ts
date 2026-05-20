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

/**
 * The type of entity to retrieve flow metadata for.
 *
 * @example
 * ```typescript
 * const config: GetFlowMetaRequestConfig = {
 *   baseUrl: 'https://localhost:8090',
 *   type: FlowMetaType.App,
 *   id: '60a9b38b-6eba-9f9e-55f9-267067de4680',
 * };
 * ```
 *
 * @experimental This API may change in future versions
 */
export enum FlowMetaType {
  /** Retrieve metadata scoped to a specific application */
  App = 'APP',

  /** Retrieve metadata scoped to a specific organization unit */
  Ou = 'OU',
}

/**
 * Application metadata returned when `type=APP`.
 *
 * @experimental This API may change in future versions
 */
export interface ApplicationMetadata {
  /** Application UUID */
  id: string;

  /** URL of the application logo */
  logoUrl?: string;

  /** Human-readable application name */
  name: string;

  /** Privacy Policy URI */
  policyUri?: string;

  /** Terms of Service URI */
  tosUri?: string;

  /** Application home URL */
  url?: string;
}

/**
 * Organization unit metadata.
 *
 * Always present when `type=OU`. For `type=APP`, only included when the
 * deployment has exactly one organization unit.
 *
 * @experimental This API may change in future versions
 */
export interface OUMetadata {
  /** Cookie Policy URI */
  cookiePolicyUri?: string;

  /** Optional description of the organization unit */
  description?: string;

  /** Unique handle / slug for the organization unit */
  handle: string;

  /** Organization unit UUID */
  id: string;

  /** URL of the organization unit logo */
  logoUrl?: string;

  /** Human-readable organization unit name */
  name: string;

  /** Privacy Policy URI */
  policyUri?: string;

  /** Terms of Service URI */
  tosUri?: string;
}

/**
 * A single color entry in the v2 theme color scheme.
 *
 * @experimental This API may change in future versions
 */
export interface FlowMetaThemeColorSet {
  /** Text color that contrasts against this color */
  contrastText?: string;

  /** The darker variant of the color */
  dark?: string;

  /** The main/primary variant of the color */
  main: string;
}

/**
 * Background colors in a v2 theme color scheme.
 *
 * @experimental This API may change in future versions
 */
export interface FlowMetaThemeBackground {
  /** Default background color (maps to body background) */
  default?: string;

  /** Surface / paper background color */
  paper?: string;
}

/**
 * Text colors in a v2 theme color scheme.
 *
 * @experimental This API may change in future versions
 */
export interface FlowMetaThemeTextColors {
  /** Primary text color */
  primary?: string;

  /** Secondary / muted text color */
  secondary?: string;
}

/**
 * All colors defined for a single v2 theme color scheme (light or dark).
 *
 * @experimental This API may change in future versions
 */
export interface FlowMetaThemeColors {
  /** Background colors for the theme (e.g., body, paper) */
  background?: FlowMetaThemeBackground;

  /** Primary color set for the theme */
  primary?: FlowMetaThemeColorSet;

  /** Secondary color set for the theme */
  secondary?: FlowMetaThemeColorSet;

  /** Text colors for the theme */
  text?: FlowMetaThemeTextColors;
}

/**
 * A single color scheme (light or dark) in the v2 theme.
 *
 * @experimental This API may change in future versions
 */
export interface FlowMetaThemeColorScheme {
  /** All colors defined for this color scheme (light or dark) */
  palette: FlowMetaThemeColors;
}

/**
 * Shape / geometry configuration in the v2 theme.
 *
 * @experimental This API may change in future versions
 */
export interface FlowMetaThemeShape {
  /** Border radius in pixels applied uniformly across components */
  borderRadius?: number;
}

/**
 * Typography configuration in the v2 theme.
 *
 * @experimental This API may change in future versions
 */
export interface FlowMetaThemeTypography {
  /** CSS font-family string */
  fontFamily?: string;
}

/**
 * Resolved theme configuration returned inside the flow metadata response.
 *
 * Maps to the `design.theme` field of {@link FlowMetadataResponse}.
 *
 * @experimental This API may change in future versions
 */
export interface FlowMetaTheme {
  /** Per-scheme color definitions (light and dark) */
  colorSchemes?: {
    /** Dark color scheme */
    dark?: FlowMetaThemeColorScheme;
    /** Light color scheme */
    light?: FlowMetaThemeColorScheme;
  };

  /** The color scheme to apply by default */
  defaultColorScheme?: 'light' | 'dark';

  /** Text direction for the theme */
  direction?: 'ltr' | 'rtl';

  /** Shape/geometry configuration for the theme */
  shape?: FlowMetaThemeShape;

  /** Typography configuration for the theme */
  typography?: FlowMetaThemeTypography;
}

/**
 * Resolved design configuration (theme and layout) for the flow.
 *
 * @experimental This API may change in future versions
 */
export interface DesignMetadata {
  /** Resolved layout configuration (shape is server-defined) */
  layout: Record<string, unknown>;

  /** Resolved theme configuration for the flow */
  theme: FlowMetaTheme;
}

/**
 * Internationalisation metadata for the flow.
 *
 * @experimental This API may change in future versions
 */
export interface I18nMetadata {
  /** The language used for the returned translations (defaults to `en`) */
  language: string;

  /** List of all available language tags (BCP 47) for the entity */
  languages: string[];

  /** Total number of translation keys returned */
  totalResults?: number;

  /**
   * Translations organised by namespace.
   *
   * @example
   * ```json
   * {
   *   "auth": {
   *     "login.button": "Login",
   *     "login.title": "Welcome"
   *   }
   * }
   * ```
   */
  translations: Record<string, Record<string, string>>;
}

/**
 * Aggregated flow metadata response returned by `GET /flow/meta`.
 *
 * @experimental This API may change in future versions
 */
export interface FlowMetadataResponse {
  /**
   * Application metadata.
   * Only present when `type=APP`.
   */
  application?: ApplicationMetadata;

  /** Resolved design configuration */
  design: DesignMetadata;

  /** Internationalisation metadata and translations */
  i18n: I18nMetadata;

  /** Indicates whether the registration flow is enabled for the entity */
  isRegistrationFlowEnabled: boolean;

  /**
   * Organization unit metadata.
   * Always present when `type=OU`.
   * For `type=APP`, only present when the deployment has exactly one OU.
   */
  ou?: OUMetadata;
}

/**
 * Request configuration for `getFlowMetaV2`.
 *
 * @example
 * ```typescript
 * const config: GetFlowMetaRequestConfig = {
 *   baseUrl: 'https://localhost:8090',
 *   type: FlowMetaType.App,
 *   id: '60a9b38b-6eba-9f9e-55f9-267067de4680',
 *   language: 'en',
 *   namespace: 'auth',
 * };
 * ```
 *
 * @experimental This API may change in future versions
 */
export interface GetFlowMetaRequestConfig extends Omit<Partial<RequestInit>, 'method' | 'body'> {
  /**
   * Base URL of the Flow API server (e.g. `https://localhost:8090`).
   * Either `baseUrl` or `url` must be provided.
   */
  baseUrl?: string;

  /**
   * UUID of the entity (application ID or organization unit ID).
   * Optional — when omitted the server returns i18n-only metadata (e.g. for flows
   * like AcceptInvite that are not tied to a specific application or OU).
   */
  id?: string;

  /**
   * Language tag in BCP 47 format for i18n translations.
   * Defaults to `en` on the server side when omitted.
   *
   * @example "en", "es", "fr-CA"
   */
  language?: string;

  /**
   * Filter translations by a specific namespace.
   *
   * @example "auth", "errors", "common"
   */
  namespace?: string;

  /**
   * The type of entity to retrieve metadata for.
   * Optional — must be omitted together with `id` for flows that are not
   * associated with a specific application or OU (e.g. AcceptInvite).
   */
  type?: FlowMetaType;

  /**
   * Fully qualified URL of the `/flow/meta` endpoint.
   * When provided, `baseUrl` is ignored.
   */
  url?: string;
}
