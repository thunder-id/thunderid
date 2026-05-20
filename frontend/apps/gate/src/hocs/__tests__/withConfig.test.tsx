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

import {render} from '@testing-library/react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import withConfig from '../withConfig';

// Track the baseUrl passed to ThunderIDProvider
let capturedBaseUrl: string | undefined;

function MockChild() {
  return <div data-testid="app-with-theme">App With Theme</div>;
}
const AppWithConfig = withConfig(MockChild);

// Mock ThunderIDProvider to capture baseUrl
vi.mock('@thunderid/react', () => ({
  ThunderIDProvider: ({children, baseUrl}: {children: React.ReactNode; baseUrl: string}) => {
    capturedBaseUrl = baseUrl;
    return <div data-testid="thunderid-provider">{children}</div>;
  },
}));

// Create mock for useConfig
const mockGetServerUrl = vi.fn();
vi.mock('@thunderid/contexts', () => ({
  useConfig: () => ({
    getServerUrl: mockGetServerUrl,
  }),
}));

describe('AppWithConfig', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    capturedBaseUrl = undefined;
    // Set up default environment variable for fallback tests
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
    expect(capturedBaseUrl).toBe('https://custom-server.com');
  });

  it('falls back to VITE_THUNDER_BASE_URL when getServerUrl returns undefined', () => {
    mockGetServerUrl.mockReturnValue(undefined);
    render(<AppWithConfig />);
    expect(capturedBaseUrl).toBe('https://env-fallback-url.example.com');
  });

  it('falls back to VITE_THUNDER_BASE_URL when getServerUrl returns null', () => {
    mockGetServerUrl.mockReturnValue(null);
    render(<AppWithConfig />);
    expect(capturedBaseUrl).toBe('https://env-fallback-url.example.com');
  });
});
