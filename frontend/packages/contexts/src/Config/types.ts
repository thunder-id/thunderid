/**
 * Copyright (c) 2025-2026, WSO2 LLC. (https://www.wso2.com).
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

import {OxygenThemeType} from '@wso2/oxygen-ui/styles/index';

/**
 * Server configuration interface that defines connection parameters
 * for the backend server.
 *
 * @public
 */
export interface ServerConfig {
  /**
   * Server hostname or IP address
   * @example "localhost", "api.example.com", "192.168.1.100"
   */
  hostname: string;

  /**
   * Server port number
   * @example 8090, 3000, 8080
   */
  port: number;

  /**
   * Whether to use HTTP only (no HTTPS). When true, connections will use HTTP protocol.
   * When false, HTTPS will be used for secure connections.
   */
  http_only: boolean;

  /**
   * Optional public URL for the server. If provided, this will be used instead of
   * constructing the URL from hostname, port, and http_only.
   * @example "https://example.com", "https://api.local:8080"
   */
  public_url?: string;
}

/**
 * Client configuration interface that defines authentication and client-specific settings.
 *
 * @public
 */
export interface ClientConfig {
  /**
   * Base path for the client application.
   * @example "/console", "/admin", "/my-app"
   */
  base: string;

  /**
   * Unique identifier for the client application, used for authentication
   * and authorization with identity providers like ThunderID.
   * @example "CONSOLE", "my-app-client-id"
   */
  client_id: string;

  /**
   * UUID of the client application. If not provided in configuration,
   * it will be extracted from the applicationId URL parameter.
   * @example "e7db0b52-06a7-45e4-977b-6914b81a2069"
   */
  uuid?: string;

  /**
   * OAuth2/OIDC scopes requested during authentication.
   * @example ["openid", "profile", "email", "system"]
   */
  scopes?: string[];

  /**
   * Server hostname or IP address
   * @example "localhost", "api.example.com", "192.168.1.100"
   */
  hostname?: string;

  /**
   * Server port number
   * @example 8090, 3000, 8080
   */
  port?: number;

  /**
   * Whether to use HTTP only (no HTTPS). When true, connections will use HTTP protocol.
   * When false, HTTPS will be used for secure connections.
   */
  http_only?: boolean;
}

/**
 * Theme configuration interface that defines theming options for applications.
 */
export interface ThemeConfig {
  /** Unique key for the theme */
  key: string;

  /** Display name for the theme */
  label: string;

  /** Theme object compatible with Oxygen UI theming system or relative path to a theme config file */
  theme: string | Partial<OxygenThemeType>;
}

/**
 * Design configuration interface that defines theming and UI customization settings.
 *
 * @public
 */
export interface DesignConfig {
  initialTheme?: string;
  themes?: ThemeConfig[];
}

/**
 * Branding configuration interface that defines product name and other branding-related settings.
 *
 * @public
 */
export interface BrandConfig {
  /**
   * Product name for branding purposes.
   * @example "My Product", "Awesome Product"
   */
  product_name: string;

  /**
   * Favicon image paths for light and dark color schemes.
   * @example { light: "assets/images/favicon.ico", dark: "assets/images/favicon-inverted.ico" }
   */
  favicon: {light: string; dark: string};

  /**
   * Documentation site URLs.
   */
  documentation?: {baseUrl: string; releasesUrl: string};

  /**
   * Design configuration for theming and UI customization.
   */
  design?: DesignConfig;
}

/**
 * Trusted issuer configuration interface that defines connection parameters
 * for an external authentication server separate from the resource server.
 *
 * When provided, the application authenticates against this server (OAuth/OIDC)
 * while using the main `server` config for API resource calls.
 *
 * @public
 */
export interface TrustedIssuerConfig {
  /**
   * Trusted issuer hostname or IP address
   * @example "auth.example.com", "localhost"
   */
  hostname: string;

  /**
   * Trusted issuer port number
   * @example 8090, 443
   */
  port: number;

  /**
   * Whether to use HTTP only (no HTTPS)
   */
  http_only: boolean;

  /**
   * Optional public URL for the trusted issuer
   * @example "https://auth.example.com"
   */
  public_url?: string;

  /**
   * OAuth client ID registered on the trusted issuer for this application
   * @example "FEDERATED_CONSOLE"
   */
  client_id?: string;

  /**
   * OAuth2/OIDC scopes to request from the trusted issuer
   * @example ["openid", "profile", "email", "system"]
   */
  scopes?: string[];

  /**
   * Type of external authorization server. Set to `generic` when the trusted
   * issuer is a generic OIDC provider rather than another instance of the same type.
   * When `generic`, the console skips specific bootstrap calls
   * (flow metadata, branding preferences) that would otherwise fail against a generic OIDC provider.
   *
   * Defaults to self for backward compatibility with existing
   * federation deployments.
   */
  type?: 'thunderid' | 'generic';
}

/**
 * Runtime overrides for the ThunderID SDK provider (ThunderIDProvider props).
 *
 * Accepts any valid ThunderIDProvider prop. Values are deep-merged on top of
 * the defaults derived from the application config, so only fields that need
 * to differ from the computed defaults must be specified.
 * `config.sdk` takes the highest precedence — it overrides both the defaults
 * derived from `trusted_issuer` and the identity-related props (baseUrl,
 * clientId, afterSignInUrl, scopes) resolved from the server/client config.
 *
 * @public
 */
export type SdkConfig = Record<string, unknown>;

/**
 * Runtime configuration interface that contains all configuration
 * settings for applications.
 *
 * This interface defines the complete structure of the runtime configuration
 * that can be loaded from window object or provided
 * as default values.
 *
 * @public
 */
export interface ProductConfig {
  /** Branding configuration such as product name and logo */
  brand: BrandConfig;

  /** Client-specific configuration including authentication settings */
  client: ClientConfig;

  /** Server connection configuration for API resource calls */
  server: ServerConfig;

  /** Optional trusted issuer configuration for external token validation */
  trusted_issuer?: TrustedIssuerConfig;

  /** Optional design configuration for theming and UI customization */
  design?: DesignConfig;

  /** Optional SDK provider overrides. Values here take precedence over computed defaults. */
  sdk?: SdkConfig;
}

/**
 * Global window interface extension for runtime configuration.
 *
 * This declaration extends the global Window interface to include the
 * runtime configuration object. The configuration is typically
 * loaded from a config.js file in the public directory and made available
 * globally on the window object.
 *
 * @example
 * ```javascript
 * // In public/config.js
 * window.__THUNDERID_RUNTIME_CONFIG__ = {
 *   client: {
 *     client_id: 'CONSOLE'
 *   },
 *   server: {
 *     hostname: 'localhost',
 *     port: 8090,
 *     http_only: false
 *   }
 * };
 * ```
 *
 * @public
 */
declare global {
  interface Window {
    /** Runtime configuration loaded from config.js */
    __THUNDERID_RUNTIME_CONFIG__?: ProductConfig;
  }
}
