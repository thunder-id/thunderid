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

import {useMemo, PropsWithChildren} from 'react';
import ConfigContext, {ConfigContextType} from './ConfigContext';
import {ProductConfig} from './types';

/**
 * Props for the ConfigProvider component.
 *
 * @public
 */
export type ConfigProviderProps = PropsWithChildren;

/**
 * Loads configuration from window object or uses default values.
 *
 * This function safely accesses the global window object and merges any runtime
 * configuration with the default configuration values. It performs a deep merge
 * to ensure all configuration properties are properly set.
 *
 * @returns The merged configuration object
 *
 * @internal
 */
function loadConfig(): ProductConfig {
  if (typeof window !== 'undefined' && window.__THUNDERID_RUNTIME_CONFIG__) {
    return window.__THUNDERID_RUNTIME_CONFIG__;
  }

  throw new Error('ThunderID runtime configuration is not available on window.__THUNDERID_RUNTIME_CONFIG__');
}

/**
 * Resolves the resource server URL from config, falling back to the served origin.
 *
 * @internal
 */
function buildServerUrl(config: ProductConfig): string {
  // If public_url is provided, use it directly
  if (config.server?.public_url) {
    return config.server.public_url;
  }
  // Otherwise, construct from hostname, port, and http_only when configured
  const {hostname, port, http_only: httpOnly} = config.server ?? {};
  if (hostname && port !== undefined) {
    const protocol: string = httpOnly ? 'http' : 'https';
    return `${protocol}://${hostname}:${port}`;
  }
  // Fall back to the URL the app is served from
  return typeof window !== 'undefined' ? window.location.origin : '';
}

/**
 * React context provider component that provides runtime configuration
 * to all child components.
 *
 * This component loads configuration from window object at
 * initialization time and provides it through React context. If the global
 * configuration is not available, it falls back to default values.
 *
 * The provider creates utility methods for common configuration operations
 * such as getting the server URL, hostname, port, and checking HTTP-only mode.
 *
 * @param props - The component props
 * @param props.children - React children to be wrapped with the configuration context
 *
 * @returns JSX element that provides configuration context to children
 *
 * @example
 * ```tsx
 * import ConfigProvider from './ConfigProvider';
 * import App from './App';
 *
 * function Root() {
 *   return (
 *     <ConfigProvider>
 *       <App />
 *     </ConfigProvider>
 *   );
 * }
 * ```
 *
 * @public
 */
export default function ConfigProvider({children}: ConfigProviderProps) {
  const config = useMemo(() => loadConfig(), []);

  const contextValue: ConfigContextType = useMemo(
    () => ({
      config,
      getServerUrl: () => buildServerUrl(config),
      getGateCallbackUrl: () => {
        const gate = config.gate_client;

        let base: string | undefined;
        if (gate?.public_url) {
          base = gate.public_url;
        } else if (gate?.hostname) {
          const scheme: string = gate.scheme ?? 'https';
          base = gate.port !== undefined ? `${scheme}://${gate.hostname}:${gate.port}` : `${scheme}://${gate.hostname}`;
        }
        // Fall back to the resource server URL when the gate app is not separately configured.
        base ??= buildServerUrl(config);

        return `${base.replace(/\/+$/, '')}/gate/callback`;
      },
      getServerHostname: () => config.server?.hostname,
      getServerPort: () => config.server?.port,
      isHttpOnly: () => config.server?.http_only,
      getClientId: () => config.client.client_id,
      getScopes: () => config.client.scopes ?? [],
      getResourceIdentifier: () => config.client.resource_identifier,
      getClientUrl: () => {
        const {hostname, port, http_only: httpOnly, base} = config.client;

        // If client has its own hostname/port/protocol config, use that
        if (hostname && port !== undefined && httpOnly !== undefined) {
          const protocol: string = httpOnly ? 'http' : 'https';
          const baseUrl = `${protocol}://${hostname}:${port}`;
          return base ? `${baseUrl}${base}` : baseUrl;
        }

        // Otherwise, use window.location.origin and add base if it exists
        const origin: string = typeof window !== 'undefined' ? window.location.origin : '';
        return base ? `${origin}${base}` : origin;
      },
      getClientUuid: () => {
        // First, check if UUID is available in configuration
        if (config.client.uuid) {
          return config.client.uuid;
        }

        // If not in config, try to get applicationId from URL parameters
        if (typeof window !== 'undefined') {
          const urlParams = new URLSearchParams(window.location.search);
          const applicationId = urlParams.get('applicationId');
          if (applicationId) {
            return applicationId;
          }
        }

        return undefined;
      },
      getTrustedIssuerUrl: () => {
        if (config.trusted_issuer) {
          if (config.trusted_issuer.public_url) {
            return config.trusted_issuer.public_url;
          }
          const {hostname, port, http_only: httpOnly} = config.trusted_issuer;
          const protocol: string = httpOnly ? 'http' : 'https';
          return `${protocol}://${hostname}:${port}`;
        }
        // Fall back to resource server URL
        if (config.server?.public_url) {
          return config.server.public_url;
        }
        const {hostname, port, http_only: httpOnly} = config.server ?? {};
        if (hostname && port !== undefined) {
          const protocol: string = httpOnly ? 'http' : 'https';
          return `${protocol}://${hostname}:${port}`;
        }
        // Fall back to the URL the app is served from
        return typeof window !== 'undefined' ? window.location.origin : '';
      },
      getTrustedIssuerClientId: () => {
        if (config.trusted_issuer?.client_id) {
          return config.trusted_issuer.client_id;
        }
        return config.client.client_id;
      },
      getTrustedIssuerScopes: () => {
        if (config.trusted_issuer?.scopes) {
          return config.trusted_issuer.scopes;
        }
        return config.client.scopes ?? [];
      },
      isTrustedIssuerGenericOidc: () => config.trusted_issuer?.type === 'generic',
    }),
    [config],
  );

  return <ConfigContext.Provider value={contextValue}>{children}</ConfigContext.Provider>;
}
