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

import {useConfig} from '@thunderid/contexts';
import {render} from '@thunderid/test-utils';
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest';
import WelcomeRedirect from '../WelcomeRedirect';

const mockNavigate = vi.fn();
const mockIsSignedIn = vi.fn();
const mockLocation = {
  pathname: '/dashboard',
};

vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({
    isSignedIn: mockIsSignedIn() as boolean,
  }),
}));

vi.mock('@thunderid/contexts', async () => {
  const actual = await vi.importActual<typeof import('@thunderid/contexts')>('@thunderid/contexts');
  return {
    ...actual,
    useConfig: vi.fn(),
  };
});

vi.mock('react-router', async () => {
  const actual = await vi.importActual<typeof import('react-router')>('react-router');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
    useLocation: () => mockLocation,
  };
});

const mockSessionStorage = new Map<string, string>();

const mockStorageGetItem = vi.fn((key: string) => mockSessionStorage.get(key) ?? null);
const mockStorageSetItem = vi.fn((key: string, value: string) => {
  mockSessionStorage.set(key, value);
});
const mockStorageRemoveItem = vi.fn((key: string) => {
  mockSessionStorage.delete(key);
});
const mockStorageClear = vi.fn(() => {
  mockSessionStorage.clear();
});

const mockUseConfig = vi.mocked(useConfig);

beforeEach(() => {
  mockSessionStorage.clear();
  mockStorageGetItem.mockClear();
  mockStorageSetItem.mockClear();
  mockStorageRemoveItem.mockClear();
  mockStorageClear.mockClear();
  Object.defineProperty(globalThis, 'sessionStorage', {
    value: {
      getItem: mockStorageGetItem,
      setItem: mockStorageSetItem,
      removeItem: mockStorageRemoveItem,
      clear: mockStorageClear,
      key: vi.fn(),
      length: 0,
    },
    configurable: true,
  });
});

afterEach(() => {
  vi.clearAllMocks();
});

describe('WelcomeRedirect', () => {
  beforeEach(() => {
    mockUseConfig.mockReturnValue({
      config: {
        brand: {
          product_name: 'ThunderID',
          favicon: {light: '', dark: ''},
        },
        client: {base: '', client_id: ''},
        server: {hostname: '', port: 0, http_only: false},
      },
      getServerUrl: () => '',
      getGateCallbackUrl: () => '',
      getServerHostname: () => '',
      getServerPort: () => 0,
      isHttpOnly: () => false,
      getClientId: () => '',
      getScopes: () => [],
      getResourceIdentifier: () => undefined,
      getClientUrl: () => '',
      getClientUuid: () => undefined,
      getTrustedIssuerUrl: () => '',
      getTrustedIssuerClientId: () => '',
      getTrustedIssuerScopes: () => [],
      isTrustedIssuerGenericOidc: () => false,
    });
    mockIsSignedIn.mockReturnValue(true);
    mockLocation.pathname = '/dashboard';
  });

  describe('navigation behavior', () => {
    it('redirects to welcome page when user is signed in and has not dismissed welcome', () => {
      render(<WelcomeRedirect />);

      expect(mockNavigate).toHaveBeenCalledWith('/welcome', {replace: true});
    });

    it('sets dismissed flag in sessionStorage after redirect', () => {
      render(<WelcomeRedirect />);

      expect(mockStorageSetItem).toHaveBeenCalledWith('thunderid:welcome:dismissed', 'true');
    });

    it('does not redirect when user has already dismissed welcome', () => {
      mockSessionStorage.set('thunderid:welcome:dismissed', 'true');

      render(<WelcomeRedirect />);

      expect(mockNavigate).not.toHaveBeenCalled();
    });

    it('does not redirect when user is not signed in', () => {
      mockIsSignedIn.mockReturnValue(false);

      render(<WelcomeRedirect />);

      expect(mockNavigate).not.toHaveBeenCalled();
    });

    it('does not redirect when already on welcome page', () => {
      mockLocation.pathname = '/welcome';

      render(<WelcomeRedirect />);

      expect(mockNavigate).not.toHaveBeenCalled();
    });

    it('does not redirect when on welcome sub pages', () => {
      mockLocation.pathname = '/welcome/create-project';

      render(<WelcomeRedirect />);

      expect(mockNavigate).not.toHaveBeenCalled();
    });
  });

  describe('product name handling', () => {
    it('uses product name in sessionStorage key', () => {
      mockUseConfig.mockReturnValue({
        config: {
          brand: {
            product_name: 'CustomProduct',
            favicon: {light: '', dark: ''},
          },
          client: {base: '', client_id: ''},
          server: {hostname: '', port: 0, http_only: false},
        },
        getServerUrl: () => '',
        getGateCallbackUrl: () => '',
        getServerHostname: () => '',
        getServerPort: () => 0,
        isHttpOnly: () => false,
        getClientId: () => '',
        getScopes: () => [],
        getResourceIdentifier: () => undefined,
        getClientUrl: () => '',
        getClientUuid: () => undefined,
        getTrustedIssuerUrl: () => '',
        getTrustedIssuerClientId: () => '',
        getTrustedIssuerScopes: () => [],
        isTrustedIssuerGenericOidc: () => false,
      });

      render(<WelcomeRedirect />);

      expect(mockStorageSetItem).toHaveBeenCalledWith('customproduct:welcome:dismissed', 'true');
    });

    it('handles different product names separately', () => {
      mockUseConfig.mockReturnValue({
        config: {
          brand: {
            product_name: 'ProductA',
            favicon: {light: '', dark: ''},
          },
          client: {base: '', client_id: ''},
          server: {hostname: '', port: 0, http_only: false},
        },
        getServerUrl: () => '',
        getGateCallbackUrl: () => '',
        getServerHostname: () => '',
        getServerPort: () => 0,
        isHttpOnly: () => false,
        getClientId: () => '',
        getScopes: () => [],
        getResourceIdentifier: () => undefined,
        getClientUrl: () => '',
        getClientUuid: () => undefined,
        getTrustedIssuerUrl: () => '',
        getTrustedIssuerClientId: () => '',
        getTrustedIssuerScopes: () => [],
        isTrustedIssuerGenericOidc: () => false,
      });
      mockSessionStorage.set('welcomeDismissed-ProductB', 'true');

      render(<WelcomeRedirect />);

      expect(mockNavigate).toHaveBeenCalledWith('/welcome', {replace: true});
      expect(mockStorageSetItem).toHaveBeenCalledWith('producta:welcome:dismissed', 'true');
    });
  });

  describe('rendering', () => {
    it('renders nothing (null)', () => {
      const {container} = render(<WelcomeRedirect />);

      expect(container.firstChild).toBeNull();
    });

    it('returns null component', () => {
      const {container} = render(<WelcomeRedirect />);

      expect(container.innerHTML).toBe('');
    });
  });
});
