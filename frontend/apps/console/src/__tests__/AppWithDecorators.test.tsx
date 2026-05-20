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

import {render, screen} from '@testing-library/react';
import type {ReactNode} from 'react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import AppWithDecorators from '../AppWithDecorators';

const mockGetTrustedIssuerUrl = vi.fn();
const mockGetTrustedIssuerClientId = vi.fn();
const mockGetTrustedIssuerScopes = vi.fn();
const mockGetClientUrl = vi.fn();
const mockGetServerUrl = vi.fn();
const mockIsTrustedIssuerGenericOidc = vi.fn().mockReturnValue(false);
const mockConfig: Record<string, unknown> = {};

// Mock the useConfig hook
vi.mock('@thunderid/contexts', () => ({
  useConfig: () => ({
    getTrustedIssuerUrl: mockGetTrustedIssuerUrl,
    getTrustedIssuerClientId: mockGetTrustedIssuerClientId,
    getTrustedIssuerScopes: mockGetTrustedIssuerScopes,
    getClientUrl: mockGetClientUrl,
    getServerUrl: mockGetServerUrl,
    isTrustedIssuerGenericOidc: mockIsTrustedIssuerGenericOidc,
    config: mockConfig,
  }),
}));

// Mock ThunderIDProvider
interface MockThunderIDProviderProps {
  children: ReactNode;
  baseUrl?: string | null;
  clientId?: string | null;
  afterSignInUrl?: string | null;
  scopes?: string[];
}

vi.mock('@thunderid/react', () => ({
  ThunderIDProvider: ({
    children,
    baseUrl = null,
    clientId = null,
    afterSignInUrl = null,
    scopes = undefined,
  }: MockThunderIDProviderProps) => (
    <div
      data-testid="thunderid-provider"
      data-base-url={baseUrl}
      data-client-id={clientId}
      data-after-sign-in-url={afterSignInUrl}
      data-scopes={scopes ? JSON.stringify(scopes) : undefined}
    >
      {children}
    </div>
  ),
}));

// Mock OxygenUI (used by withTheme)
vi.mock('@wso2/oxygen-ui', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@wso2/oxygen-ui')>();
  return {
    ...actual,
    createOxygenTheme: actual.createOxygenTheme ?? ((theme: unknown) => theme),
    HighContrastTheme: actual.HighContrastTheme ?? {},
    OxygenUIThemeProvider: ({children}: {children: ReactNode}) => <div data-testid="theme-provider">{children}</div>,
    useColorScheme: () => ({mode: 'light', systemMode: 'light'}),
  };
});

// Mock i18next top-level await in withI18n
vi.mock('i18next', () => ({
  default: {
    use: vi.fn().mockReturnThis(),
    init: vi.fn().mockResolvedValue(undefined),
  },
}));

vi.mock('react-i18next', () => ({
  initReactI18next: {},
}));

vi.mock('@thunderid/i18n/locales/en-US', () => ({
  default: {common: {}, navigation: {}},
}));

// Mock I18nProvider (used by withI18n)
vi.mock('../i18n/I18nProvider', () => ({
  default: ({children}: {children: ReactNode}) => <div data-testid="i18n-provider">{children}</div>,
}));

// Mock App component
vi.mock('../App', () => ({
  default: () => <div data-testid="app">App Component</div>,
}));

describe('AppWithDecorators', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // Reset the mock config object (preserving the same reference so the vi.mock closure keeps working).
    Object.keys(mockConfig).forEach((key) => delete mockConfig[key]);
    mockConfig.brand = {favicon: {light: 'assets/images/favicon.ico', dark: 'assets/images/favicon-inverted.ico'}};
    // Set up default environment variables
    import.meta.env.VITE_THUNDER_BASE_URL = 'https://default-base.example.com';
    import.meta.env.VITE_THUNDER_CLIENT_ID = 'default-client-id';
    import.meta.env.VITE_THUNDER_AFTER_SIGN_IN_URL = 'https://default-signin.example.com';
    // Default to empty scopes
    mockGetTrustedIssuerScopes.mockReturnValue([]);
  });

  it('renders ThunderIDProvider with config values', () => {
    mockGetTrustedIssuerClientId.mockReturnValue('test-client-id');
    mockGetTrustedIssuerUrl.mockReturnValue('https://test-server.example.com');
    mockGetClientUrl.mockReturnValue('https://test-client.example.com');

    render(<AppWithDecorators />);

    const provider = screen.getByTestId('thunderid-provider');
    expect(provider).toHaveAttribute('data-base-url', 'https://test-server.example.com');
    expect(provider).toHaveAttribute('data-client-id', 'test-client-id');
    expect(provider).toHaveAttribute('data-after-sign-in-url', 'https://test-client.example.com');
  });

  it('falls back to environment variables when config returns null', () => {
    mockGetTrustedIssuerClientId.mockReturnValue(null);
    mockGetTrustedIssuerUrl.mockReturnValue(null);
    mockGetClientUrl.mockReturnValue(null);

    render(<AppWithDecorators />);

    const provider = screen.getByTestId('thunderid-provider');
    expect(provider).toHaveAttribute('data-base-url', 'https://default-base.example.com');
    expect(provider).toHaveAttribute('data-client-id', 'default-client-id');
    expect(provider).toHaveAttribute('data-after-sign-in-url', 'https://default-signin.example.com');
  });

  it('renders App component', () => {
    mockGetTrustedIssuerClientId.mockReturnValue('test-client-id');
    mockGetTrustedIssuerUrl.mockReturnValue('https://test-server.example.com');
    mockGetClientUrl.mockReturnValue('https://test-client.example.com');

    render(<AppWithDecorators />);

    expect(screen.getByTestId('app')).toBeInTheDocument();
  });

  it('uses config value for baseUrl when available', () => {
    mockGetTrustedIssuerUrl.mockReturnValue('https://config-server.example.com');
    mockGetTrustedIssuerClientId.mockReturnValue(null);
    mockGetClientUrl.mockReturnValue(null);

    render(<AppWithDecorators />);

    const provider = screen.getByTestId('thunderid-provider');
    expect(provider).toHaveAttribute('data-base-url', 'https://config-server.example.com');
  });

  it('uses config value for clientId when available', () => {
    mockGetTrustedIssuerClientId.mockReturnValue('config-client-id');
    mockGetTrustedIssuerUrl.mockReturnValue(null);
    mockGetClientUrl.mockReturnValue(null);

    render(<AppWithDecorators />);

    const provider = screen.getByTestId('thunderid-provider');
    expect(provider).toHaveAttribute('data-client-id', 'config-client-id');
  });

  it('uses config value for afterSignInUrl when available', () => {
    mockGetClientUrl.mockReturnValue('https://config-client.example.com');
    mockGetTrustedIssuerUrl.mockReturnValue(null);
    mockGetTrustedIssuerClientId.mockReturnValue(null);

    render(<AppWithDecorators />);

    const provider = screen.getByTestId('thunderid-provider');
    expect(provider).toHaveAttribute('data-after-sign-in-url', 'https://config-client.example.com');
  });

  it('falls back to environment variables when config returns undefined', () => {
    mockGetTrustedIssuerClientId.mockReturnValue(undefined);
    mockGetTrustedIssuerUrl.mockReturnValue(undefined);
    mockGetClientUrl.mockReturnValue(undefined);

    render(<AppWithDecorators />);

    const provider = screen.getByTestId('thunderid-provider');
    expect(provider).toHaveAttribute('data-base-url', 'https://default-base.example.com');
    expect(provider).toHaveAttribute('data-client-id', 'default-client-id');
    expect(provider).toHaveAttribute('data-after-sign-in-url', 'https://default-signin.example.com');
  });

  it('handles mixed config values and fallbacks - scenario 1', () => {
    mockGetTrustedIssuerUrl.mockReturnValue('https://config-server.example.com');
    mockGetTrustedIssuerClientId.mockReturnValue(undefined);
    mockGetClientUrl.mockReturnValue('https://config-client.example.com');

    render(<AppWithDecorators />);

    const provider = screen.getByTestId('thunderid-provider');
    expect(provider).toHaveAttribute('data-base-url', 'https://config-server.example.com');
    expect(provider).toHaveAttribute('data-client-id', 'default-client-id');
    expect(provider).toHaveAttribute('data-after-sign-in-url', 'https://config-client.example.com');
  });

  it('handles mixed config values and fallbacks - scenario 2', () => {
    mockGetTrustedIssuerUrl.mockReturnValue(null);
    mockGetTrustedIssuerClientId.mockReturnValue('config-client-id');
    mockGetClientUrl.mockReturnValue(null);

    render(<AppWithDecorators />);

    const provider = screen.getByTestId('thunderid-provider');
    expect(provider).toHaveAttribute('data-base-url', 'https://default-base.example.com');
    expect(provider).toHaveAttribute('data-client-id', 'config-client-id');
    expect(provider).toHaveAttribute('data-after-sign-in-url', 'https://default-signin.example.com');
  });

  it('uses config value for scopes when available', () => {
    mockGetTrustedIssuerClientId.mockReturnValue('test-client-id');
    mockGetTrustedIssuerUrl.mockReturnValue('https://test-server.example.com');
    mockGetClientUrl.mockReturnValue('https://test-client.example.com');
    mockGetTrustedIssuerScopes.mockReturnValue(['openid', 'profile', 'email', 'system']);

    render(<AppWithDecorators />);

    const provider = screen.getByTestId('thunderid-provider');
    expect(provider).toHaveAttribute('data-scopes', '["openid","profile","email","system"]');
  });

  it('does not pass scopes prop when config returns empty array', () => {
    mockGetTrustedIssuerClientId.mockReturnValue('test-client-id');
    mockGetTrustedIssuerUrl.mockReturnValue('https://test-server.example.com');
    mockGetClientUrl.mockReturnValue('https://test-client.example.com');
    mockGetTrustedIssuerScopes.mockReturnValue([]);

    render(<AppWithDecorators />);

    const provider = screen.getByTestId('thunderid-provider');
    expect(provider).not.toHaveAttribute('data-scopes');
  });

  it('passes scopes when config has scopes', () => {
    mockGetTrustedIssuerClientId.mockReturnValue('test-client-id');
    mockGetTrustedIssuerUrl.mockReturnValue('https://test-server.example.com');
    mockGetClientUrl.mockReturnValue('https://test-client.example.com');
    mockGetTrustedIssuerScopes.mockReturnValue(['openid', 'profile']);

    render(<AppWithDecorators />);

    const provider = screen.getByTestId('thunderid-provider');
    expect(provider).toHaveAttribute('data-scopes', '["openid","profile"]');
  });

  it('handles scopes from config with other fallbacks', () => {
    mockGetTrustedIssuerClientId.mockReturnValue(null);
    mockGetTrustedIssuerUrl.mockReturnValue(null);
    mockGetClientUrl.mockReturnValue(null);
    mockGetTrustedIssuerScopes.mockReturnValue(['openid', 'profile', 'email']);

    render(<AppWithDecorators />);

    const provider = screen.getByTestId('thunderid-provider');
    expect(provider).toHaveAttribute('data-base-url', 'https://default-base.example.com');
    expect(provider).toHaveAttribute('data-client-id', 'default-client-id');
    expect(provider).toHaveAttribute('data-after-sign-in-url', 'https://default-signin.example.com');
    expect(provider).toHaveAttribute('data-scopes', '["openid","profile","email"]');
  });

  it('properly evaluates falsy values for config options', () => {
    // Test that falsy values (null, undefined, empty string, etc.) are properly handled
    // Empty strings are truthy in JavaScript, so they will be used as-is
    mockGetTrustedIssuerClientId.mockReturnValue('');
    mockGetTrustedIssuerUrl.mockReturnValue('');
    mockGetClientUrl.mockReturnValue('');
    mockGetTrustedIssuerScopes.mockReturnValue([]);

    render(<AppWithDecorators />);

    const provider = screen.getByTestId('thunderid-provider');
    // Empty strings are truthy, so they will be passed through (not fallback to env vars)
    expect(provider).toHaveAttribute('data-base-url', '');
    expect(provider).toHaveAttribute('data-client-id', '');
    expect(provider).toHaveAttribute('data-after-sign-in-url', '');
    expect(provider).not.toHaveAttribute('data-scopes');
  });

  it('handles all config values as truthy strings', () => {
    mockGetTrustedIssuerClientId.mockReturnValue('client-123');
    mockGetTrustedIssuerUrl.mockReturnValue('https://server.test');
    mockGetClientUrl.mockReturnValue('https://client.test');
    mockGetTrustedIssuerScopes.mockReturnValue(['scope1', 'scope2', 'scope3']);

    render(<AppWithDecorators />);

    const provider = screen.getByTestId('thunderid-provider');
    expect(provider).toHaveAttribute('data-base-url', 'https://server.test');
    expect(provider).toHaveAttribute('data-client-id', 'client-123');
    expect(provider).toHaveAttribute('data-after-sign-in-url', 'https://client.test');
    expect(provider).toHaveAttribute('data-scopes', '["scope1","scope2","scope3"]');
  });
});
