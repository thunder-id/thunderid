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

import {Context, createContext} from 'react';
import {ProductConfig} from './types';

/**
 * Configuration context interface that provides access to runtime configuration
 * and utility methods for server-related operations.
 *
 * @public
 */
export interface ConfigContextType {
  /**
   * The complete configuration object loaded from window object
   * or default values if not available
   */
  config: ProductConfig;

  /**
   * Gets the complete server URL including protocol, hostname, and port
   * @returns The full server URL (e.g., "https://localhost:8090")
   */
  getServerUrl: () => string;

  /**
   * Gets the server hostname from the configuration
   * @returns The server hostname (e.g., "localhost"), or undefined when not configured
   */
  getServerHostname: () => string | undefined;

  /**
   * Gets the server port from the configuration
   * @returns The server port number (e.g., 8090), or undefined when not configured
   */
  getServerPort: () => number | undefined;

  /**
   * Checks if HTTP-only mode is enabled in the configuration
   * @returns True if HTTP-only mode is enabled, false if HTTPS is used, or undefined when not configured
   */
  isHttpOnly: () => boolean | undefined;

  /**
   * Gets the client ID from the configuration
   * @returns The client ID string (e.g., "CONSOLE")
   */
  getClientId: () => string;

  /**
   * Gets the OAuth2/OIDC scopes from the configuration
   * @returns The scopes array (e.g., ["openid", "profile", "email", "system"])
   */
  getScopes: () => string[];

  /**
   * Gets the resource server identifier from the configuration
   * @returns The identifier string, or undefined if not configured
   */
  getResourceIdentifier: () => string | undefined;

  /**
   * Gets the complete client URL including protocol, hostname, port, and base path
   * @returns The full client URL (e.g., "https://localhost:8090/console")
   */
  getClientUrl: () => string;

  /**
   * Gets the client UUID from configuration or URL parameters
   * @returns The client UUID string or undefined if not available
   */
  getClientUuid: () => string | undefined;

  /**
   * Gets the trusted issuer URL. When trusted_issuer is configured, returns
   * the external issuer URL. Otherwise falls back to getServerUrl().
   * @returns The trusted issuer URL (e.g., "https://auth.example.com:8090")
   */
  getTrustedIssuerUrl: () => string;

  /**
   * Gets the OAuth redirect/callback URL for the login gate app. When gate_client is
   * configured, builds the URL from it. Otherwise falls back to
   * `${getServerUrl()}/gate/callback`.
   * @returns The gate callback URL (e.g., "https://localhost:5190/gate/callback")
   */
  getGateCallbackUrl: () => string;

  /**
   * Gets the OAuth client ID for the trusted issuer. When trusted_issuer.client_id is
   * configured, returns that. Otherwise falls back to getClientId().
   * @returns The trusted issuer client ID string
   */
  getTrustedIssuerClientId: () => string;

  /**
   * Gets the OAuth scopes for the trusted issuer. When trusted_issuer.scopes is
   * configured, returns those. Otherwise falls back to getScopes().
   * @returns The trusted issuer scopes array
   */
  getTrustedIssuerScopes: () => string[];

  /**
   * Indicates whether the configured trusted issuer is a generic OIDC provider
   * rather than the same type of instance. When true, the console must suppress
   * specific bootstrap calls (flow metadata, branding preferences) that
   * would otherwise fail against a generic OIDC provider.
   *
   * Returns false when no trusted issuer is configured, and when the configured
   * trusted issuer type is `default`.
   *
   * @returns True if the trusted issuer is a generic OIDC provider
   */
  isTrustedIssuerGenericOidc: () => boolean;
}

/**
 * React context for accessing runtime configuration throughout the application.
 *
 * This context provides access to the configuration loaded from window object.
 * or falls back to default values. It should be used within a `ConfigProvider` component.
 *
 * @example
 * ```tsx
 * import ConfigContext from './ConfigContext';
 * import { useContext } from 'react';
 *
 * const MyComponent = () => {
 *   const context = useContext(ConfigContext);
 *   if (!context) {
 *     throw new Error('Component must be used within ConfigProvider');
 *   }
 *
 *   const { config, getServerUrl } = context;
 *   return <div>Server: {getServerUrl()}</div>;
 * };
 * ```
 *
 * @public
 */
const ConfigContext: Context<ConfigContextType | undefined> = createContext<ConfigContextType | undefined>(undefined);

export default ConfigContext;
