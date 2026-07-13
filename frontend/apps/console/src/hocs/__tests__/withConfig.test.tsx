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

import {render, screen} from '@testing-library/react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import withConfig from '../withConfig';

let capturedProviderProps: Record<string, unknown> = {};

function MockChild() {
  return <div data-testid="mock-child">Child</div>;
}
const WithConfigComponent = withConfig(MockChild);

const mockGetClientId = vi.fn();
const mockGetServerUrl = vi.fn();
const mockGetClientUrl = vi.fn();
const mockGetScopes = vi.fn();
const mockGetTrustedIssuerUrl = vi.fn();
const mockGetTrustedIssuerClientId = vi.fn();
const mockGetTrustedIssuerScopes = vi.fn();
const mockGetResourceIdentifier = vi.fn();
const mockIsTrustedIssuerGenericOidc = vi.fn();
const mockConfig: Record<string, unknown> = {};

vi.mock('@thunderid/contexts', () => ({
  useConfig: () => ({
    getClientId: mockGetClientId,
    getServerUrl: mockGetServerUrl,
    getClientUrl: mockGetClientUrl,
    getScopes: mockGetScopes,
    getTrustedIssuerUrl: mockGetTrustedIssuerUrl,
    getTrustedIssuerClientId: mockGetTrustedIssuerClientId,
    getTrustedIssuerScopes: mockGetTrustedIssuerScopes,
    getResourceIdentifier: mockGetResourceIdentifier,
    isTrustedIssuerGenericOidc: mockIsTrustedIssuerGenericOidc,
    config: mockConfig,
  }),
}));

vi.mock('@thunderid/react', () => ({
  ThunderIDProvider: ({
    children,
    /* eslint-disable react/require-default-props */
    baseUrl,
    clientId,
    afterSignInUrl,
    scopes,
    signInOptions,
    preferences,
    sendCookiesInRequests,
    discovery,
    /* eslint-enable react/require-default-props */
  }: {
    children: React.ReactNode;
    baseUrl?: string;
    clientId?: string;
    afterSignInUrl?: string;
    scopes?: string[];
    signInOptions?: Record<string, string>;
    preferences?: Record<string, unknown>;
    sendCookiesInRequests?: boolean;
    discovery?: Record<string, unknown>;
  }) => {
    capturedProviderProps = {
      baseUrl,
      clientId,
      afterSignInUrl,
      scopes,
      signInOptions,
      preferences,
      sendCookiesInRequests,
      discovery,
    };
    return (
      <div
        data-testid="thunderid-provider"
        data-base-url={baseUrl}
        data-client-id={clientId}
        data-after-sign-in-url={afterSignInUrl}
        data-scopes={scopes ? JSON.stringify(scopes) : undefined}
      >
        {children}
      </div>
    );
  },
}));

describe('withConfig (console)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    capturedProviderProps = {};
    Object.keys(mockConfig).forEach((key) => delete mockConfig[key]);
    import.meta.env.VITE_THUNDER_BASE_URL = 'https://env-base.example.com';
    import.meta.env.VITE_THUNDER_CLIENT_ID = 'env-client-id';
    import.meta.env.VITE_THUNDER_AFTER_SIGN_IN_URL = 'https://env-signin.example.com';
    mockGetScopes.mockReturnValue([]);
    mockGetTrustedIssuerUrl.mockReturnValue('https://server.example.com');
    mockGetTrustedIssuerClientId.mockReturnValue('client-id');
    mockGetTrustedIssuerScopes.mockReturnValue([]);
    mockGetResourceIdentifier.mockReturnValue(undefined);
    mockIsTrustedIssuerGenericOidc.mockReturnValue(false);
  });

  it('renders without crashing', () => {
    mockGetClientId.mockReturnValue('client-id');
    mockGetServerUrl.mockReturnValue('https://server.example.com');
    mockGetClientUrl.mockReturnValue('https://client.example.com');

    const {container} = render(<WithConfigComponent />);
    expect(container).toBeInTheDocument();
  });

  it('renders the wrapped component', () => {
    mockGetClientId.mockReturnValue('client-id');
    mockGetServerUrl.mockReturnValue('https://server.example.com');
    mockGetClientUrl.mockReturnValue('https://client.example.com');

    render(<WithConfigComponent />);
    expect(screen.getByTestId('mock-child')).toBeInTheDocument();
  });

  it('wraps with ThunderIDProvider', () => {
    mockGetClientId.mockReturnValue('client-id');
    mockGetServerUrl.mockReturnValue('https://server.example.com');
    mockGetClientUrl.mockReturnValue('https://client.example.com');

    render(<WithConfigComponent />);
    expect(screen.getByTestId('thunderid-provider')).toBeInTheDocument();
  });

  it('passes baseUrl from getTrustedIssuerUrl to ThunderIDProvider', () => {
    mockGetTrustedIssuerUrl.mockReturnValue('https://custom-server.example.com');
    mockGetTrustedIssuerClientId.mockReturnValue('client-id');
    mockGetClientUrl.mockReturnValue('https://client.example.com');

    render(<WithConfigComponent />);
    expect(capturedProviderProps.baseUrl).toBe('https://custom-server.example.com');
  });

  it('passes clientId from getTrustedIssuerClientId to ThunderIDProvider', () => {
    mockGetTrustedIssuerClientId.mockReturnValue('custom-client-id');
    mockGetTrustedIssuerUrl.mockReturnValue('https://server.example.com');
    mockGetClientUrl.mockReturnValue('https://client.example.com');

    render(<WithConfigComponent />);
    expect(capturedProviderProps.clientId).toBe('custom-client-id');
  });

  it('passes afterSignInUrl from useConfig to ThunderIDProvider', () => {
    mockGetClientUrl.mockReturnValue('https://custom-client.example.com');
    mockGetServerUrl.mockReturnValue('https://server.example.com');
    mockGetClientId.mockReturnValue('client-id');

    render(<WithConfigComponent />);
    expect(capturedProviderProps.afterSignInUrl).toBe('https://custom-client.example.com');
  });

  it('falls back to env VITE_THUNDER_BASE_URL when getTrustedIssuerUrl returns null', () => {
    mockGetTrustedIssuerUrl.mockReturnValue(null);
    mockGetTrustedIssuerClientId.mockReturnValue('client-id');
    mockGetClientUrl.mockReturnValue('https://client.example.com');

    render(<WithConfigComponent />);
    expect(capturedProviderProps.baseUrl).toBe('https://env-base.example.com');
  });

  it('falls back to env VITE_THUNDER_CLIENT_ID when getTrustedIssuerClientId returns null', () => {
    mockGetTrustedIssuerClientId.mockReturnValue(null);
    mockGetTrustedIssuerUrl.mockReturnValue('https://server.example.com');
    mockGetClientUrl.mockReturnValue('https://client.example.com');

    render(<WithConfigComponent />);
    expect(capturedProviderProps.clientId).toBe('env-client-id');
  });

  it('falls back to env VITE_THUNDER_AFTER_SIGN_IN_URL when getClientUrl returns null', () => {
    mockGetClientUrl.mockReturnValue(null);
    mockGetServerUrl.mockReturnValue('https://server.example.com');
    mockGetClientId.mockReturnValue('client-id');

    render(<WithConfigComponent />);
    expect(capturedProviderProps.afterSignInUrl).toBe('https://env-signin.example.com');
  });

  it('passes scopes when getTrustedIssuerScopes returns a non-empty array', () => {
    mockGetTrustedIssuerScopes.mockReturnValue(['openid', 'profile', 'email']);
    mockGetTrustedIssuerUrl.mockReturnValue('https://server.example.com');
    mockGetTrustedIssuerClientId.mockReturnValue('client-id');
    mockGetClientUrl.mockReturnValue('https://client.example.com');

    render(<WithConfigComponent />);
    expect(capturedProviderProps.scopes).toEqual(['openid', 'profile', 'email']);
  });

  it('does not pass scopes when getTrustedIssuerScopes returns empty array', () => {
    mockGetTrustedIssuerScopes.mockReturnValue([]);
    mockGetTrustedIssuerUrl.mockReturnValue('https://server.example.com');
    mockGetTrustedIssuerClientId.mockReturnValue('client-id');
    mockGetClientUrl.mockReturnValue('https://client.example.com');

    render(<WithConfigComponent />);
    expect(capturedProviderProps.scopes).toBeUndefined();
  });

  it('wraps different components correctly', () => {
    function AnotherChild() {
      return <div data-testid="another-child">Another</div>;
    }
    const AnotherWrapped = withConfig(AnotherChild);
    mockGetServerUrl.mockReturnValue('https://server.example.com');
    mockGetClientId.mockReturnValue('client-id');
    mockGetClientUrl.mockReturnValue('https://client.example.com');

    render(<AnotherWrapped />);
    expect(screen.getByTestId('another-child')).toBeInTheDocument();
  });

  // --- Trusted Issuer integration ---

  describe('trusted issuer', () => {
    it('passes baseUrl from getTrustedIssuerUrl to ThunderIDProvider', () => {
      mockGetTrustedIssuerUrl.mockReturnValue('http://localhost:8090');
      mockGetTrustedIssuerClientId.mockReturnValue('FEDERATED_CONSOLE');
      mockGetClientUrl.mockReturnValue('http://localhost:9443/console');
      mockGetServerUrl.mockReturnValue('http://localhost:9443');

      render(<WithConfigComponent />);
      expect(capturedProviderProps.baseUrl).toBe('http://localhost:8090');
    });

    it('passes clientId from getTrustedIssuerClientId to ThunderIDProvider', () => {
      mockGetTrustedIssuerUrl.mockReturnValue('http://localhost:8090');
      mockGetTrustedIssuerClientId.mockReturnValue('FEDERATED_CONSOLE');
      mockGetClientUrl.mockReturnValue('http://localhost:9443/console');
      mockGetServerUrl.mockReturnValue('http://localhost:9443');

      render(<WithConfigComponent />);
      expect(capturedProviderProps.clientId).toBe('FEDERATED_CONSOLE');
    });

    it('passes scopes from getTrustedIssuerScopes to ThunderIDProvider', () => {
      mockGetTrustedIssuerUrl.mockReturnValue('http://localhost:8090');
      mockGetTrustedIssuerClientId.mockReturnValue('FEDERATED_CONSOLE');
      mockGetTrustedIssuerScopes.mockReturnValue(['openid', 'profile', 'system']);
      mockGetClientUrl.mockReturnValue('http://localhost:9443/console');
      mockGetServerUrl.mockReturnValue('http://localhost:9443');

      render(<WithConfigComponent />);
      expect(capturedProviderProps.scopes).toEqual(['openid', 'profile', 'system']);
    });

    it('sets signInOptions with resource when trusted_issuer is configured', () => {
      mockConfig.trusted_issuer = {hostname: 'localhost', port: 8090, http_only: true};
      mockGetServerUrl.mockReturnValue('http://localhost:9443');
      mockGetTrustedIssuerUrl.mockReturnValue('http://localhost:8090');
      mockGetTrustedIssuerClientId.mockReturnValue('FEDERATED_CONSOLE');
      mockGetClientUrl.mockReturnValue('http://localhost:9443/console');

      render(<WithConfigComponent />);
      expect(capturedProviderProps.signInOptions).toEqual({resource: 'http://localhost:9443'});
    });

    it('sets signInOptions resource to the resource server URL, not the issuer URL', () => {
      mockConfig.trusted_issuer = {hostname: 'auth.cloud.example.com', port: 443, http_only: false};
      mockGetServerUrl.mockReturnValue('https://tenant.example.com:9443');
      mockGetTrustedIssuerUrl.mockReturnValue('https://auth.cloud.example.com');
      mockGetTrustedIssuerClientId.mockReturnValue('FEDERATED_CONSOLE');
      mockGetClientUrl.mockReturnValue('https://tenant.example.com:9443/console');

      render(<WithConfigComponent />);
      expect(capturedProviderProps.signInOptions).toEqual({resource: 'https://tenant.example.com:9443'});
    });

    it('does not set signInOptions when trusted_issuer is not configured', () => {
      mockGetServerUrl.mockReturnValue('https://server.example.com');
      mockGetClientId.mockReturnValue('client-id');
      mockGetClientUrl.mockReturnValue('https://client.example.com');

      render(<WithConfigComponent />);
      expect(capturedProviderProps.signInOptions).toBeUndefined();
    });

    it('does not pass preferences when the trusted issuer is a same instance', () => {
      mockConfig.trusted_issuer = {hostname: 'localhost', port: 8090, http_only: true, type: 'thunderid'};
      mockIsTrustedIssuerGenericOidc.mockReturnValue(false);
      mockGetServerUrl.mockReturnValue('http://localhost:9443');
      mockGetTrustedIssuerUrl.mockReturnValue('http://localhost:8090');
      mockGetTrustedIssuerClientId.mockReturnValue('FEDERATED_CONSOLE');
      mockGetClientUrl.mockReturnValue('http://localhost:9443/console');

      render(<WithConfigComponent />);
      expect(capturedProviderProps.preferences).toBeUndefined();
    });

    it('disables ThunderID vendor-specific bootstrap calls for generic OIDC trusted issuers', () => {
      mockConfig.trusted_issuer = {hostname: 'tenant.auth0.com', port: 443, http_only: false, type: 'generic'};
      mockIsTrustedIssuerGenericOidc.mockReturnValue(true);
      mockGetServerUrl.mockReturnValue('https://tenant.example.com:9443');
      mockGetTrustedIssuerUrl.mockReturnValue('https://tenant.auth0.com');
      mockGetTrustedIssuerClientId.mockReturnValue('AUTH0_CLIENT_ID');
      mockGetClientUrl.mockReturnValue('https://tenant.example.com:9443/console');

      render(<WithConfigComponent />);
      expect(capturedProviderProps.preferences).toEqual({
        resolveFromMeta: false,
      });
    });

    it('leaves sendCookiesInRequests at SDK default when the trusted issuer is a same instance', () => {
      mockConfig.trusted_issuer = {hostname: 'localhost', port: 8090, http_only: true, type: 'thunderid'};
      mockIsTrustedIssuerGenericOidc.mockReturnValue(false);
      mockGetServerUrl.mockReturnValue('http://localhost:9443');
      mockGetTrustedIssuerUrl.mockReturnValue('http://localhost:8090');
      mockGetTrustedIssuerClientId.mockReturnValue('FEDERATED_CONSOLE');
      mockGetClientUrl.mockReturnValue('http://localhost:9443/console');

      render(<WithConfigComponent />);
      expect(capturedProviderProps.sendCookiesInRequests).toBeUndefined();
    });

    it('sets sendCookiesInRequests=false for generic OIDC trusted issuers (CORS credentialed fetch fix)', () => {
      mockConfig.trusted_issuer = {hostname: 'tenant.auth0.com', port: 443, http_only: false, type: 'generic'};
      mockIsTrustedIssuerGenericOidc.mockReturnValue(true);
      mockGetServerUrl.mockReturnValue('https://tenant.example.com:9443');
      mockGetTrustedIssuerUrl.mockReturnValue('https://tenant.auth0.com');
      mockGetTrustedIssuerClientId.mockReturnValue('AUTH0_CLIENT_ID');
      mockGetClientUrl.mockReturnValue('https://tenant.example.com:9443/console');

      render(<WithConfigComponent />);
      expect(capturedProviderProps.sendCookiesInRequests).toBe(false);
    });
  });

  // --- resource indicator ---

  describe('resource indicator', () => {
    it('sets signInOptions.resource from the resource identifier without trusted_issuer', () => {
      mockGetResourceIdentifier.mockReturnValue('https://localhost:8090/mcp');
      mockGetClientUrl.mockReturnValue('https://client.example.com');

      render(<WithConfigComponent />);
      expect(capturedProviderProps.signInOptions).toEqual({resource: 'https://localhost:8090/mcp'});
    });

    it('uses the server URL over the resource identifier in the trusted_issuer model', () => {
      mockConfig.trusted_issuer = {hostname: 'localhost', port: 8090, http_only: true};
      mockGetResourceIdentifier.mockReturnValue('https://localhost:8090/mcp');
      mockGetServerUrl.mockReturnValue('http://localhost:9443');
      mockGetTrustedIssuerUrl.mockReturnValue('http://localhost:8090');
      mockGetTrustedIssuerClientId.mockReturnValue('FEDERATED_CONSOLE');
      mockGetClientUrl.mockReturnValue('http://localhost:9443/console');

      render(<WithConfigComponent />);
      expect(capturedProviderProps.signInOptions).toEqual({resource: 'http://localhost:9443'});
    });
  });

  // --- config.sdk overrides ---

  describe('sdk overrides', () => {
    it('passes wellKnown.enabled=true by default when no sdk.discovery override', () => {
      mockGetClientUrl.mockReturnValue('https://client.example.com');

      render(<WithConfigComponent />);
      expect(capturedProviderProps.discovery).toEqual({wellKnown: {enabled: true}});
    });

    it('overrides discovery with config.sdk.discovery', () => {
      mockConfig.sdk = {discovery: {wellKnown: {enabled: false}}};
      mockGetClientUrl.mockReturnValue('https://client.example.com');

      render(<WithConfigComponent />);
      expect(capturedProviderProps.discovery).toEqual({wellKnown: {enabled: false}});
    });

    it('overrides sendCookiesInRequests with config.sdk.sendCookiesInRequests', () => {
      mockConfig.sdk = {sendCookiesInRequests: false};
      mockGetClientUrl.mockReturnValue('https://client.example.com');

      render(<WithConfigComponent />);
      expect(capturedProviderProps.sendCookiesInRequests).toBe(false);
    });

    it('config.sdk.sendCookiesInRequests=true overrides generic OIDC default of false', () => {
      mockConfig.trusted_issuer = {hostname: 'tenant.auth0.com', port: 443, http_only: false, type: 'generic'};
      mockConfig.sdk = {sendCookiesInRequests: true};
      mockIsTrustedIssuerGenericOidc.mockReturnValue(true);
      mockGetServerUrl.mockReturnValue('https://tenant.example.com:9443');
      mockGetTrustedIssuerUrl.mockReturnValue('https://tenant.auth0.com');
      mockGetTrustedIssuerClientId.mockReturnValue('AUTH0_CLIENT_ID');
      mockGetClientUrl.mockReturnValue('https://tenant.example.com:9443/console');

      render(<WithConfigComponent />);
      expect(capturedProviderProps.sendCookiesInRequests).toBe(true);
    });

    it('merges config.sdk.signInOptions on top of trusted_issuer resource param', () => {
      mockConfig.trusted_issuer = {hostname: 'localhost', port: 8090, http_only: true};
      mockConfig.sdk = {signInOptions: {prompt: 'login', acr_values: 'urn:example:silver'}};
      mockGetServerUrl.mockReturnValue('http://localhost:9443');
      mockGetTrustedIssuerUrl.mockReturnValue('http://localhost:8090');
      mockGetTrustedIssuerClientId.mockReturnValue('FEDERATED_CONSOLE');
      mockGetClientUrl.mockReturnValue('http://localhost:9443/console');

      render(<WithConfigComponent />);
      expect(capturedProviderProps.signInOptions).toEqual({
        resource: 'http://localhost:9443',
        prompt: 'login',
        acr_values: 'urn:example:silver',
      });
    });

    it('sets signInOptions from config.sdk.signInOptions when no trusted_issuer', () => {
      mockConfig.sdk = {signInOptions: {prompt: 'consent'}};
      mockGetClientUrl.mockReturnValue('https://client.example.com');

      render(<WithConfigComponent />);
      expect(capturedProviderProps.signInOptions).toEqual({prompt: 'consent'});
    });

    it('merges config.sdk.preferences on top of generic OIDC computed preferences', () => {
      mockConfig.trusted_issuer = {hostname: 'tenant.auth0.com', port: 443, http_only: false, type: 'generic'};
      mockConfig.sdk = {preferences: {resolveFromMeta: true}};
      mockIsTrustedIssuerGenericOidc.mockReturnValue(true);
      mockGetServerUrl.mockReturnValue('https://tenant.example.com:9443');
      mockGetTrustedIssuerUrl.mockReturnValue('https://tenant.auth0.com');
      mockGetTrustedIssuerClientId.mockReturnValue('AUTH0_CLIENT_ID');
      mockGetClientUrl.mockReturnValue('https://tenant.example.com:9443/console');

      render(<WithConfigComponent />);
      expect(capturedProviderProps.preferences).toEqual({
        resolveFromMeta: true,
      });
    });

    it('sets preferences from config.sdk.preferences when no generic OIDC', () => {
      mockConfig.sdk = {preferences: {resolveFromMeta: false}};
      mockGetClientUrl.mockReturnValue('https://client.example.com');

      render(<WithConfigComponent />);
      expect(capturedProviderProps.preferences).toEqual({
        resolveFromMeta: false,
      });
    });

    it('overrides baseUrl with config.sdk.baseUrl', () => {
      mockConfig.sdk = {baseUrl: 'https://override.example.com'};
      mockGetTrustedIssuerUrl.mockReturnValue('https://trusted-issuer.example.com');
      mockGetTrustedIssuerClientId.mockReturnValue('client-id');
      mockGetClientUrl.mockReturnValue('https://client.example.com');

      render(<WithConfigComponent />);
      expect(capturedProviderProps.baseUrl).toBe('https://override.example.com');
    });

    it('overrides clientId with config.sdk.clientId', () => {
      mockConfig.sdk = {clientId: 'SDK_OVERRIDE_CLIENT'};
      mockGetTrustedIssuerUrl.mockReturnValue('https://trusted-issuer.example.com');
      mockGetTrustedIssuerClientId.mockReturnValue('TRUSTED_CLIENT');
      mockGetClientUrl.mockReturnValue('https://client.example.com');

      render(<WithConfigComponent />);
      expect(capturedProviderProps.clientId).toBe('SDK_OVERRIDE_CLIENT');
    });

    it('overrides afterSignInUrl with config.sdk.afterSignInUrl', () => {
      mockConfig.sdk = {afterSignInUrl: 'https://override-redirect.example.com'};
      mockGetClientUrl.mockReturnValue('https://client.example.com');

      render(<WithConfigComponent />);
      expect(capturedProviderProps.afterSignInUrl).toBe('https://override-redirect.example.com');
    });

    it('overrides scopes with config.sdk.scopes', () => {
      mockConfig.sdk = {scopes: ['openid', 'email', 'custom']};
      mockGetTrustedIssuerScopes.mockReturnValue(['openid', 'profile']);
      mockGetTrustedIssuerUrl.mockReturnValue('https://trusted-issuer.example.com');
      mockGetTrustedIssuerClientId.mockReturnValue('client-id');
      mockGetClientUrl.mockReturnValue('https://client.example.com');

      render(<WithConfigComponent />);
      expect(capturedProviderProps.scopes).toEqual(['openid', 'email', 'custom']);
    });
  });
});
