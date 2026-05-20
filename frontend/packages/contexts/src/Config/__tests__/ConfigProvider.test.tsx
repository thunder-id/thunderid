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

import {act} from 'react';
import {createRoot, type Root} from 'react-dom/client';
import {describe, it, expect, beforeEach, afterEach} from 'vitest';
import ConfigProvider from '../ConfigProvider';
import type {ProductConfig} from '../types';
import useConfig from '../useConfig';

function buildConfig(overrides?: Partial<ProductConfig>): ProductConfig {
  return {
    brand: {
      product_name: 'ThunderID',
      favicon: {light: 'assets/images/favicon.ico', dark: 'assets/images/favicon-inverted.ico'},
    },
    client: {base: '/console', client_id: 'CONSOLE'},
    server: {hostname: 'localhost', port: 8090, http_only: false},
    ...overrides,
  };
}

let container: HTMLDivElement;
let root: Root;

function renderWithConfig(config: ProductConfig | undefined, Consumer: React.ComponentType) {
  window.__THUNDERID_RUNTIME_CONFIG__ = config;
  act(() => {
    root.render(
      <ConfigProvider>
        <Consumer />
      </ConfigProvider>,
    );
  });
}

function ConfigConsumer() {
  const ctx = useConfig();
  return (
    <div>
      <span data-testid="server-url">{ctx.getServerUrl()}</span>
      <span data-testid="trusted-issuer-url">{ctx.getTrustedIssuerUrl()}</span>
      <span data-testid="trusted-issuer-client-id">{ctx.getTrustedIssuerClientId()}</span>
      <span data-testid="trusted-issuer-scopes">{JSON.stringify(ctx.getTrustedIssuerScopes())}</span>
      <span data-testid="client-id">{ctx.getClientId()}</span>
      <span data-testid="scopes">{JSON.stringify(ctx.getScopes())}</span>
      <span data-testid="server-hostname">{ctx.getServerHostname()}</span>
      <span data-testid="server-port">{ctx.getServerPort()}</span>
      <span data-testid="is-http-only">{String(ctx.isHttpOnly())}</span>
    </div>
  );
}

function getTestId(id: string): string {
  return container.querySelector(`[data-testid="${id}"]`)?.textContent ?? '';
}

describe('ConfigProvider', () => {
  let originalConfig: ProductConfig | undefined;

  beforeEach(() => {
    originalConfig = window.__THUNDERID_RUNTIME_CONFIG__;
    container = document.createElement('div');
    document.body.appendChild(container);
    root = createRoot(container);
  });

  afterEach(() => {
    act(() => {
      root.unmount();
    });
    window.__THUNDERID_RUNTIME_CONFIG__ = originalConfig;
    document.body.removeChild(container);
  });

  it('throws when window config is not set', () => {
    window.__THUNDERID_RUNTIME_CONFIG__ = undefined;
    expect(() => {
      act(() => {
        root.render(
          <ConfigProvider>
            <ConfigConsumer />
          </ConfigProvider>,
        );
      });
    }).toThrow('ThunderID runtime configuration is not available on window.__THUNDERID_RUNTIME_CONFIG__');
  });

  it('provides server URL with HTTPS when http_only is false', () => {
    renderWithConfig(buildConfig(), ConfigConsumer);
    expect(getTestId('server-url')).toBe('https://localhost:8090');
  });

  it('provides server URL with HTTP when http_only is true', () => {
    renderWithConfig(buildConfig({server: {hostname: 'localhost', port: 9443, http_only: true}}), ConfigConsumer);
    expect(getTestId('server-url')).toBe('http://localhost:9443');
  });

  it('uses public_url for server URL when provided', () => {
    renderWithConfig(
      buildConfig({
        server: {hostname: 'localhost', port: 8090, http_only: false, public_url: 'https://example.com'},
      }),
      ConfigConsumer,
    );
    expect(getTestId('server-url')).toBe('https://example.com');
  });

  // --- getTrustedIssuerUrl ---

  describe('getTrustedIssuerUrl', () => {
    it('returns trusted issuer URL when configured with http_only false', () => {
      renderWithConfig(
        buildConfig({trusted_issuer: {hostname: 'auth.example.com', port: 443, http_only: false}}),
        ConfigConsumer,
      );
      expect(getTestId('trusted-issuer-url')).toBe('https://auth.example.com:443');
    });

    it('returns trusted issuer URL with HTTP when http_only is true', () => {
      renderWithConfig(
        buildConfig({trusted_issuer: {hostname: 'localhost', port: 8090, http_only: true}}),
        ConfigConsumer,
      );
      expect(getTestId('trusted-issuer-url')).toBe('http://localhost:8090');
    });

    it('uses trusted issuer public_url when provided', () => {
      renderWithConfig(
        buildConfig({
          trusted_issuer: {
            hostname: 'localhost',
            port: 8090,
            http_only: true,
            public_url: 'https://auth.cloud.example.com',
          },
        }),
        ConfigConsumer,
      );
      expect(getTestId('trusted-issuer-url')).toBe('https://auth.cloud.example.com');
    });

    it('falls back to server URL when trusted_issuer is not configured', () => {
      renderWithConfig(buildConfig(), ConfigConsumer);
      expect(getTestId('trusted-issuer-url')).toBe('https://localhost:8090');
    });

    it('falls back to server public_url when trusted_issuer is not configured and public_url is set', () => {
      renderWithConfig(
        buildConfig({
          server: {hostname: 'localhost', port: 8090, http_only: false, public_url: 'https://api.example.com'},
        }),
        ConfigConsumer,
      );
      expect(getTestId('trusted-issuer-url')).toBe('https://api.example.com');
    });
  });

  // --- getTrustedIssuerClientId ---

  describe('getTrustedIssuerClientId', () => {
    it('returns trusted issuer client_id when configured', () => {
      renderWithConfig(
        buildConfig({
          trusted_issuer: {hostname: 'localhost', port: 8090, http_only: true, client_id: 'FEDERATED_CONSOLE'},
        }),
        ConfigConsumer,
      );
      expect(getTestId('trusted-issuer-client-id')).toBe('FEDERATED_CONSOLE');
    });

    it('falls back to client.client_id when trusted_issuer has no client_id', () => {
      renderWithConfig(
        buildConfig({trusted_issuer: {hostname: 'localhost', port: 8090, http_only: true}}),
        ConfigConsumer,
      );
      expect(getTestId('trusted-issuer-client-id')).toBe('CONSOLE');
    });

    it('falls back to client.client_id when trusted_issuer is not configured', () => {
      renderWithConfig(buildConfig(), ConfigConsumer);
      expect(getTestId('trusted-issuer-client-id')).toBe('CONSOLE');
    });
  });

  // --- getTrustedIssuerScopes ---

  describe('getTrustedIssuerScopes', () => {
    it('returns trusted issuer scopes when configured', () => {
      renderWithConfig(
        buildConfig({
          trusted_issuer: {
            hostname: 'localhost',
            port: 8090,
            http_only: true,
            scopes: ['openid', 'profile', 'system'],
          },
        }),
        ConfigConsumer,
      );
      expect(getTestId('trusted-issuer-scopes')).toBe(JSON.stringify(['openid', 'profile', 'system']));
    });

    it('falls back to client.scopes when trusted_issuer has no scopes', () => {
      renderWithConfig(
        buildConfig({
          client: {base: '/console', client_id: 'CONSOLE', scopes: ['openid', 'email']},
          trusted_issuer: {hostname: 'localhost', port: 8090, http_only: true},
        }),
        ConfigConsumer,
      );
      expect(getTestId('trusted-issuer-scopes')).toBe(JSON.stringify(['openid', 'email']));
    });

    it('falls back to client.scopes when trusted_issuer is not configured', () => {
      renderWithConfig(
        buildConfig({client: {base: '/console', client_id: 'CONSOLE', scopes: ['openid', 'profile']}}),
        ConfigConsumer,
      );
      expect(getTestId('trusted-issuer-scopes')).toBe(JSON.stringify(['openid', 'profile']));
    });

    it('returns empty array when neither trusted_issuer nor client scopes are set', () => {
      renderWithConfig(buildConfig(), ConfigConsumer);
      expect(getTestId('trusted-issuer-scopes')).toBe(JSON.stringify([]));
    });
  });
});
