// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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
 * Resolves a documentation link path against the configured base URL. Absolute URLs
 * (starting with `http`) are returned as-is.
 *
 * @internal
 */
function resolveDocumentationUrl(baseUrl: string, path: string): string {
  if (/^https?:\/\//.test(path)) {
    return path;
  }
  return `${baseUrl.replace(/\/+$/, '')}/${path.replace(/^\/+/, '')}`;
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

  // `config` is exposed on the context below, so consumers can read a plain field (e.g.
  // config.documentation?.baseUrl) directly. Only add a getter method here when it does real
  // work, resolving, merging, or falling back, beyond a single optional-chained property access.
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
      isControlPlane: () => config.mode === 'control_plane',
      getDocumentationLink: (key: string) => {
        const baseUrl = config.documentation?.baseUrl;
        const path = config.documentation?.links?.[key];

        if (!baseUrl || !path) {
          return undefined;
        }

        return resolveDocumentationUrl(baseUrl, path);
      },
    }),
    [config],
  );

  return <ConfigContext.Provider value={contextValue}>{children}</ConfigContext.Provider>;
}
