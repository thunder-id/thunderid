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

const mockGetClientUrl = vi.fn();
const mockConfig: Record<string, unknown> = {};

vi.mock('@thunderid/contexts', () => ({
  useConfig: () => ({
    getClientUrl: mockGetClientUrl,
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
    mockGetClientUrl.mockReturnValue('https://client.example.com');
  });

  it('renders without crashing', () => {
    const {container} = render(<WithConfigComponent />);
    expect(container).toBeInTheDocument();
  });

  it('renders the wrapped component', () => {
    render(<WithConfigComponent />);
    expect(screen.getByTestId('mock-child')).toBeInTheDocument();
  });

  it('wraps with ThunderIDProvider', () => {
    render(<WithConfigComponent />);
    expect(screen.getByTestId('thunderid-provider')).toBeInTheDocument();
  });

  it('passes baseUrl from VITE_THUNDER_BASE_URL env var to ThunderIDProvider', () => {
    render(<WithConfigComponent />);
    expect(capturedProviderProps.baseUrl).toBe('https://env-base.example.com');
  });

  it('passes clientId from VITE_THUNDER_CLIENT_ID env var to ThunderIDProvider', () => {
    render(<WithConfigComponent />);
    expect(capturedProviderProps.clientId).toBe('env-client-id');
  });

  it('passes afterSignInUrl from getClientUrl to ThunderIDProvider', () => {
    mockGetClientUrl.mockReturnValue('https://custom-client.example.com');

    render(<WithConfigComponent />);
    expect(capturedProviderProps.afterSignInUrl).toBe('https://custom-client.example.com');
  });

  it('falls back to env VITE_THUNDER_AFTER_SIGN_IN_URL when getClientUrl returns null', () => {
    mockGetClientUrl.mockReturnValue(null);

    render(<WithConfigComponent />);
    expect(capturedProviderProps.afterSignInUrl).toBe('https://env-signin.example.com');
  });

  it('wraps different components correctly', () => {
    function AnotherChild() {
      return <div data-testid="another-child">Another</div>;
    }
    const AnotherWrapped = withConfig(AnotherChild);

    render(<AnotherWrapped />);
    expect(screen.getByTestId('another-child')).toBeInTheDocument();
  });

  // --- config.sdk overrides ---

  describe('sdk overrides', () => {
    it('passes wellKnown.enabled=true by default when no sdk.discovery override', () => {
      render(<WithConfigComponent />);
      expect(capturedProviderProps.discovery).toEqual({wellKnown: {enabled: true}});
    });

    it('overrides discovery with config.sdk.discovery', () => {
      mockConfig.sdk = {discovery: {wellKnown: {enabled: false}}};

      render(<WithConfigComponent />);
      expect(capturedProviderProps.discovery).toEqual({wellKnown: {enabled: false}});
    });

    it('overrides sendCookiesInRequests with config.sdk.sendCookiesInRequests', () => {
      mockConfig.sdk = {sendCookiesInRequests: false};

      render(<WithConfigComponent />);
      expect(capturedProviderProps.sendCookiesInRequests).toBe(false);
    });

    it('sets signInOptions from config.sdk.signInOptions', () => {
      mockConfig.sdk = {signInOptions: {prompt: 'consent'}};

      render(<WithConfigComponent />);
      expect(capturedProviderProps.signInOptions).toEqual({prompt: 'consent'});
    });

    it('sets preferences from config.sdk.preferences', () => {
      mockConfig.sdk = {preferences: {resolveFromMeta: false, theme: {inheritFromBranding: false}}};

      render(<WithConfigComponent />);
      expect(capturedProviderProps.preferences).toEqual({
        resolveFromMeta: false,
        theme: {inheritFromBranding: false},
      });
    });

    it('merges config.sdk.preferences, preserving unspecified sibling keys', () => {
      mockConfig.sdk = {preferences: {resolveFromMeta: true}};

      render(<WithConfigComponent />);
      expect(capturedProviderProps.preferences).toEqual({resolveFromMeta: true});
    });

    it('overrides baseUrl with config.sdk.baseUrl', () => {
      mockConfig.sdk = {baseUrl: 'https://override.example.com'};

      render(<WithConfigComponent />);
      expect(capturedProviderProps.baseUrl).toBe('https://override.example.com');
    });

    it('overrides clientId with config.sdk.clientId', () => {
      mockConfig.sdk = {clientId: 'SDK_OVERRIDE_CLIENT'};

      render(<WithConfigComponent />);
      expect(capturedProviderProps.clientId).toBe('SDK_OVERRIDE_CLIENT');
    });

    it('overrides afterSignInUrl with config.sdk.afterSignInUrl', () => {
      mockConfig.sdk = {afterSignInUrl: 'https://override-redirect.example.com'};

      render(<WithConfigComponent />);
      expect(capturedProviderProps.afterSignInUrl).toBe('https://override-redirect.example.com');
    });

    it('overrides scopes with config.sdk.scopes', () => {
      mockConfig.sdk = {scopes: ['openid', 'email', 'custom']};

      render(<WithConfigComponent />);
      expect(capturedProviderProps.scopes).toEqual(['openid', 'email', 'custom']);
    });
  });
});
