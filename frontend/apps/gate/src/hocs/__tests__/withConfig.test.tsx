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

/* eslint-disable react/require-default-props */
import {render} from '@testing-library/react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import withConfig from '../withConfig';

let capturedProviderProps: Record<string, unknown> = {};

function MockChild() {
  return <div data-testid="app-with-theme">App With Theme</div>;
}
const AppWithConfig = withConfig(MockChild);

vi.mock('@thunderid/react', () => ({
  ThunderIDProvider: ({
    children,
    baseUrl,
    discovery,
    preferences,
    sendCookiesInRequests,
    signInOptions,
  }: {
    children: React.ReactNode;
    baseUrl?: string;
    discovery?: Record<string, unknown>;
    preferences?: Record<string, unknown>;
    sendCookiesInRequests?: boolean;
    signInOptions?: Record<string, string>;
  }) => {
    capturedProviderProps = {baseUrl, discovery, preferences, sendCookiesInRequests, signInOptions};
    return <div data-testid="thunderid-provider">{children}</div>;
  },
}));

const mockGetServerUrl = vi.fn();
const mockConfig: Record<string, unknown> = {};

vi.mock('@thunderid/contexts', () => ({
  useConfig: () => ({
    getServerUrl: mockGetServerUrl,
    config: mockConfig,
  }),
}));

describe('AppWithConfig', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    capturedProviderProps = {};
    Object.keys(mockConfig).forEach((key) => delete mockConfig[key]);
    import.meta.env.VITE_THUNDER_BASE_URL = 'https://env-fallback-url.example.com';
  });

  it('renders without crashing', () => {
    mockGetServerUrl.mockReturnValue('https://server-url.com');
    const {container} = render(<AppWithConfig />);
    expect(container).toBeInTheDocument();
  });

  it('renders AppWithTheme component', () => {
    mockGetServerUrl.mockReturnValue('https://server-url.com');
    const {getByTestId} = render(<AppWithConfig />);
    expect(getByTestId('app-with-theme')).toBeInTheDocument();
  });

  it('uses getServerUrl when available', () => {
    mockGetServerUrl.mockReturnValue('https://custom-server.com');
    render(<AppWithConfig />);
    expect(capturedProviderProps.baseUrl).toBe('https://custom-server.com');
  });

  it('falls back to VITE_THUNDER_BASE_URL when getServerUrl returns undefined', () => {
    mockGetServerUrl.mockReturnValue(undefined);
    render(<AppWithConfig />);
    expect(capturedProviderProps.baseUrl).toBe('https://env-fallback-url.example.com');
  });

  it('falls back to VITE_THUNDER_BASE_URL when getServerUrl returns null', () => {
    mockGetServerUrl.mockReturnValue(null);
    render(<AppWithConfig />);
    expect(capturedProviderProps.baseUrl).toBe('https://env-fallback-url.example.com');
  });

  // --- config.sdk overrides ---

  describe('sdk overrides', () => {
    it('passes no extra sdk props when config.sdk is absent', () => {
      mockGetServerUrl.mockReturnValue('https://server-url.com');

      render(<AppWithConfig />);
      expect(capturedProviderProps.discovery).toBeUndefined();
      expect(capturedProviderProps.preferences).toBeUndefined();
      expect(capturedProviderProps.sendCookiesInRequests).toBeUndefined();
      expect(capturedProviderProps.signInOptions).toBeUndefined();
    });

    it('passes config.sdk.discovery to ThunderIDProvider', () => {
      mockConfig.sdk = {discovery: {wellKnown: {enabled: false}}};
      mockGetServerUrl.mockReturnValue('https://server-url.com');

      render(<AppWithConfig />);
      expect(capturedProviderProps.discovery).toEqual({wellKnown: {enabled: false}});
    });

    it('passes config.sdk.sendCookiesInRequests to ThunderIDProvider', () => {
      mockConfig.sdk = {sendCookiesInRequests: false};
      mockGetServerUrl.mockReturnValue('https://server-url.com');

      render(<AppWithConfig />);
      expect(capturedProviderProps.sendCookiesInRequests).toBe(false);
    });

    it('passes config.sdk.signInOptions to ThunderIDProvider', () => {
      mockConfig.sdk = {signInOptions: {prompt: 'login'}};
      mockGetServerUrl.mockReturnValue('https://server-url.com');

      render(<AppWithConfig />);
      expect(capturedProviderProps.signInOptions).toEqual({prompt: 'login'});
    });

    it('passes config.sdk.preferences to ThunderIDProvider', () => {
      mockConfig.sdk = {preferences: {resolveFromMeta: false}};
      mockGetServerUrl.mockReturnValue('https://server-url.com');

      render(<AppWithConfig />);
      expect(capturedProviderProps.preferences).toEqual({
        resolveFromMeta: false,
      });
    });

    it('overrides baseUrl with config.sdk.baseUrl', () => {
      mockConfig.sdk = {baseUrl: 'https://sdk-override.example.com'};
      mockGetServerUrl.mockReturnValue('https://server-url.com');

      render(<AppWithConfig />);
      expect(capturedProviderProps.baseUrl).toBe('https://sdk-override.example.com');
    });
  });
});
