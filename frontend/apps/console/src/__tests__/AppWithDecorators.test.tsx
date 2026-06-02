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

const mockGetClientUrl = vi.fn();
const mockConfig: Record<string, unknown> = {};

vi.mock('@thunderid/contexts', () => ({
  useConfig: () => ({
    getClientUrl: mockGetClientUrl,
    config: mockConfig,
  }),
}));

interface MockThunderIDProviderProps {
  children: ReactNode;
  baseUrl?: string | null;
  clientId?: string | null;
  afterSignInUrl?: string | null;
  scopes?: string[];
  discovery?: unknown;
}

vi.mock('@thunderid/react', () => ({
  ThunderIDProvider: ({
    children,
    baseUrl = null,
    clientId = null,
    afterSignInUrl = null,
    scopes = undefined,
    discovery = undefined,
  }: MockThunderIDProviderProps) => (
    <div
      data-testid="thunderid-provider"
      data-base-url={baseUrl}
      data-client-id={clientId}
      data-after-sign-in-url={afterSignInUrl}
      data-scopes={scopes ? JSON.stringify(scopes) : undefined}
      data-discovery={discovery ? JSON.stringify(discovery) : undefined}
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
    Object.keys(mockConfig).forEach((key) => delete mockConfig[key]);
    mockConfig.brand = {favicon: {light: 'assets/images/favicon.ico', dark: 'assets/images/favicon-inverted.ico'}};
    import.meta.env.VITE_THUNDER_BASE_URL = 'https://default-base.example.com';
    import.meta.env.VITE_THUNDER_CLIENT_ID = 'default-client-id';
    import.meta.env.VITE_THUNDER_AFTER_SIGN_IN_URL = 'https://default-signin.example.com';
    mockGetClientUrl.mockReturnValue('https://default-client.example.com');
  });

  it('renders ThunderIDProvider with env var and getClientUrl defaults', () => {
    render(<AppWithDecorators />);

    const provider = screen.getByTestId('thunderid-provider');
    expect(provider).toHaveAttribute('data-base-url', 'https://default-base.example.com');
    expect(provider).toHaveAttribute('data-client-id', 'default-client-id');
    expect(provider).toHaveAttribute('data-after-sign-in-url', 'https://default-client.example.com');
  });

  it('falls back to afterSignInUrl env var when getClientUrl returns null', () => {
    mockGetClientUrl.mockReturnValue(null);

    render(<AppWithDecorators />);

    const provider = screen.getByTestId('thunderid-provider');
    expect(provider).toHaveAttribute('data-after-sign-in-url', 'https://default-signin.example.com');
  });

  it('renders App component', () => {
    render(<AppWithDecorators />);

    expect(screen.getByTestId('app')).toBeInTheDocument();
  });

  it('sdk config overrides baseUrl', () => {
    mockConfig.sdk = {baseUrl: 'https://sdk-base.example.com'};

    render(<AppWithDecorators />);

    const provider = screen.getByTestId('thunderid-provider');
    expect(provider).toHaveAttribute('data-base-url', 'https://sdk-base.example.com');
  });

  it('sdk config overrides clientId', () => {
    mockConfig.sdk = {clientId: 'sdk-client-id'};

    render(<AppWithDecorators />);

    const provider = screen.getByTestId('thunderid-provider');
    expect(provider).toHaveAttribute('data-client-id', 'sdk-client-id');
  });

  it('sdk config overrides afterSignInUrl', () => {
    mockConfig.sdk = {afterSignInUrl: 'https://sdk-redirect.example.com'};

    render(<AppWithDecorators />);

    const provider = screen.getByTestId('thunderid-provider');
    expect(provider).toHaveAttribute('data-after-sign-in-url', 'https://sdk-redirect.example.com');
  });

  it('sdk config sets scopes', () => {
    mockConfig.sdk = {scopes: ['openid', 'profile', 'email']};

    render(<AppWithDecorators />);

    const provider = screen.getByTestId('thunderid-provider');
    expect(provider).toHaveAttribute('data-scopes', '["openid","profile","email"]');
  });

  it('does not pass scopes when sdk config has none', () => {
    render(<AppWithDecorators />);

    const provider = screen.getByTestId('thunderid-provider');
    expect(provider).not.toHaveAttribute('data-scopes');
  });

  it('applies default discovery when no sdk config', () => {
    render(<AppWithDecorators />);

    const provider = screen.getByTestId('thunderid-provider');
    expect(provider).toHaveAttribute('data-discovery', JSON.stringify({wellKnown: {enabled: true}}));
  });

  it('sdk config overrides discovery', () => {
    mockConfig.sdk = {discovery: {wellKnown: {enabled: false}}};

    render(<AppWithDecorators />);

    const provider = screen.getByTestId('thunderid-provider');
    expect(provider).toHaveAttribute('data-discovery', JSON.stringify({wellKnown: {enabled: false}}));
  });
});
