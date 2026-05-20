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

import {render, screen, act} from '@testing-library/react';
import {describe, it, expect, vi, beforeEach} from 'vitest';

// Mock i18next top-level await before importing withI18n
vi.mock('i18next', () => ({
  default: {
    use: vi.fn().mockReturnThis(),
    init: vi.fn().mockResolvedValue(undefined),
  },
}));

vi.mock('@thunderid/i18n/locales/en-US', () => ({
  default: {common: {}, navigation: {}},
}));

const mockMeta = {i18n: {language: 'en-US', translations: {}}};
const mockAddResourceBundle = vi.fn();
const mockEmit = vi.fn();
const mockChangeLanguage = vi.fn().mockResolvedValue(undefined);
const mockGetResourceBundle = vi.fn().mockReturnValue({});

vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({meta: mockMeta}),
}));

vi.mock('react-i18next', () => ({
  initReactI18next: {},
  useTranslation: () => ({
    i18n: {
      language: 'en-US',
      getResourceBundle: mockGetResourceBundle,
      addResourceBundle: mockAddResourceBundle,
      emit: mockEmit,
      changeLanguage: mockChangeLanguage,
    },
  }),
}));

describe('withI18n (gate)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders without crashing', async () => {
    const {default: withI18n} = await import('../withI18n');
    function MockChild() {
      return <div data-testid="mock-child">Child</div>;
    }
    const WithI18nComponent = withI18n(MockChild);

    const {container} = render(<WithI18nComponent />);
    expect(container).toBeInTheDocument();
  });

  it('renders the wrapped component', async () => {
    const {default: withI18n} = await import('../withI18n');
    function MockChild() {
      return <div data-testid="mock-child">Child</div>;
    }
    const WithI18nComponent = withI18n(MockChild);

    render(<WithI18nComponent />);
    expect(screen.getByTestId('mock-child')).toBeInTheDocument();
  });

  it('wraps different components correctly', async () => {
    const {default: withI18n} = await import('../withI18n');
    function AnotherChild() {
      return <div data-testid="another-child">Another</div>;
    }
    const AnotherWrapped = withI18n(AnotherChild);

    render(<AnotherWrapped />);
    expect(screen.getByTestId('another-child')).toBeInTheDocument();
  });

  it('merges translations from meta into i18next when available', async () => {
    mockMeta.i18n = {
      language: 'en-US',
      translations: {
        common: {greeting: 'Hello'},
      },
    };

    const {default: withI18n} = await import('../withI18n');
    function MockChild() {
      return <div data-testid="mock-child">Child</div>;
    }
    const WithI18nComponent = withI18n(MockChild);

    act(() => {
      render(<WithI18nComponent />);
    });

    expect(mockAddResourceBundle).toHaveBeenCalledWith(
      'en-US',
      'common',
      expect.objectContaining({greeting: 'Hello'}),
      true,
      true,
    );
    expect(mockEmit).toHaveBeenCalledWith('added', 'en-US', ['common']);
  });

  it('does not crash when meta has no i18n translations', async () => {
    mockMeta.i18n = undefined as unknown as typeof mockMeta.i18n;

    const {default: withI18n} = await import('../withI18n');
    function MockChild() {
      return <div data-testid="mock-child">Child</div>;
    }
    const WithI18nComponent = withI18n(MockChild);

    render(<WithI18nComponent />);
    expect(screen.getByTestId('mock-child')).toBeInTheDocument();
  });

  it('does not crash when meta is null', async () => {
    (mockMeta as unknown as Record<string, unknown>).i18n = undefined;

    const {default: withI18n} = await import('../withI18n');
    function MockChild() {
      return <div data-testid="mock-child">Child</div>;
    }
    const WithI18nComponent = withI18n(MockChild);

    render(<WithI18nComponent />);
    expect(screen.getByTestId('mock-child')).toBeInTheDocument();
  });

  it('skips namespace with empty translations object', async () => {
    mockMeta.i18n = {
      language: 'en-US',
      translations: {
        common: {},
        nav: {home: 'Home'},
      },
    };

    const {default: withI18n} = await import('../withI18n');
    function MockChild() {
      return <div data-testid="mock-child">Child</div>;
    }
    const WithI18nComponent = withI18n(MockChild);

    act(() => {
      render(<WithI18nComponent />);
    });

    // Should only add nav, not common (empty)
    expect(mockAddResourceBundle).toHaveBeenCalledWith(
      'en-US',
      'nav',
      expect.objectContaining({home: 'Home'}),
      true,
      true,
    );
    expect(mockAddResourceBundle).not.toHaveBeenCalledWith(
      'en-US',
      'common',
      expect.anything(),
      expect.anything(),
      expect.anything(),
    );
  });

  it('changes language when meta language differs from current i18n language', async () => {
    mockMeta.i18n = {
      language: 'fr-FR',
      translations: {
        common: {greeting: 'Bonjour'},
      },
    };

    const {default: withI18n} = await import('../withI18n');
    function MockChild() {
      return <div data-testid="mock-child">Child</div>;
    }
    const WithI18nComponent = withI18n(MockChild);

    act(() => {
      render(<WithI18nComponent />);
    });

    expect(mockChangeLanguage).toHaveBeenCalledWith('fr-FR');
  });
});
