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

import {render, cleanup} from '@testing-library/react';
import {describe, it, expect, vi, beforeEach, afterEach} from 'vitest';
import I18nProvider from '../I18nProvider';
import {invalidateI18nCache} from '../invalidate-i18n-cache';

// Mock react-i18next
const mockAddResourceBundle = vi.fn();
const mockGetResourceBundle = vi.fn();
const mockEmit = vi.fn();
const mockI18n = {
  language: 'en-US',
  addResourceBundle: mockAddResourceBundle,
  getResourceBundle: mockGetResourceBundle,
  emit: mockEmit,
};

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    i18n: mockI18n,
  }),
}));

// Mock @tanstack/react-query
const mockInvalidateQueries = vi.fn().mockResolvedValue(undefined);
const mockQueryClient = {
  invalidateQueries: mockInvalidateQueries,
};

let mockQueryData:
  | {
      language: string;
      translations: Record<string, Record<string, string>>;
    }
  | undefined;

// Capture queryFn for testing
let capturedQueryFn: (() => Promise<unknown>) | null = null;

vi.mock('@tanstack/react-query', () => ({
  useQuery: (options: {queryFn: () => Promise<unknown>}) => {
    capturedQueryFn = options.queryFn;
    return {
      data: mockQueryData,
    };
  },
  useQueryClient: () => mockQueryClient,
}));

// Mock contexts
vi.mock('@thunderid/contexts', () => ({
  useConfig: () => ({
    getServerUrl: () => 'https://api.example.com',
  }),
}));

// Mock @thunderid/react
const mockHttpRequest = vi.fn().mockResolvedValue({
  data: {
    language: 'en-US',
    translations: {},
  },
});

vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({
    http: {
      request: mockHttpRequest,
    },
  }),
}));

describe('I18nProvider', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockQueryData = undefined;
    mockGetResourceBundle.mockReturnValue({});
    capturedQueryFn = null;
  });

  afterEach(() => {
    cleanup();
  });

  it('should render children', () => {
    const {getByText} = render(
      <I18nProvider>
        <div>Test Child</div>
      </I18nProvider>,
    );

    expect(getByText('Test Child')).toBeInTheDocument();
  });

  it('should not add translations when apiTranslations is undefined', () => {
    mockQueryData = undefined;

    render(
      <I18nProvider>
        <div>Test</div>
      </I18nProvider>,
    );

    expect(mockAddResourceBundle).not.toHaveBeenCalled();
    expect(mockEmit).not.toHaveBeenCalled();
  });

  it('should not add translations when translations object is empty', () => {
    mockQueryData = {
      language: 'en-US',
      translations: {},
    };

    render(
      <I18nProvider>
        <div>Test</div>
      </I18nProvider>,
    );

    expect(mockAddResourceBundle).not.toHaveBeenCalled();
    expect(mockEmit).not.toHaveBeenCalled();
  });

  it('should skip empty namespace translations', () => {
    mockQueryData = {
      language: 'en-US',
      translations: {
        emptyNamespace: {},
        validNamespace: {
          key1: 'value1',
        },
      },
    };

    render(
      <I18nProvider>
        <div>Test</div>
      </I18nProvider>,
    );

    // Should only add the valid namespace
    expect(mockAddResourceBundle).toHaveBeenCalledTimes(1);
    expect(mockAddResourceBundle).toHaveBeenCalledWith(
      'en-US',
      'validNamespace',
      expect.objectContaining({key1: 'value1'}),
      true,
      true,
    );
  });

  it('should merge API translations with existing bundle', () => {
    mockGetResourceBundle.mockReturnValue({
      existingKey: 'existingValue',
      overriddenKey: 'oldValue',
    });

    mockQueryData = {
      language: 'en-US',
      translations: {
        testNamespace: {
          newKey: 'newValue',
          overriddenKey: 'newValue',
        },
      },
    };

    render(
      <I18nProvider>
        <div>Test</div>
      </I18nProvider>,
    );

    expect(mockAddResourceBundle).toHaveBeenCalledWith(
      'en-US',
      'testNamespace',
      {
        existingKey: 'existingValue',
        newKey: 'newValue',
        overriddenKey: 'newValue', // API takes precedence
      },
      true,
      true,
    );
  });

  it('should emit added event when translations are added', () => {
    mockQueryData = {
      language: 'en-US',
      translations: {
        namespace1: {key1: 'value1'},
        namespace2: {key2: 'value2'},
      },
    };

    render(
      <I18nProvider>
        <div>Test</div>
      </I18nProvider>,
    );

    expect(mockEmit).toHaveBeenCalledWith('added', 'en-US', ['namespace1', 'namespace2']);
  });

  it('should not emit added event when no translations were added', () => {
    mockQueryData = {
      language: 'en-US',
      translations: {
        emptyNamespace: {},
      },
    };

    render(
      <I18nProvider>
        <div>Test</div>
      </I18nProvider>,
    );

    expect(mockEmit).not.toHaveBeenCalled();
  });

  it('should handle undefined existing bundle gracefully', () => {
    mockGetResourceBundle.mockReturnValue(undefined);

    mockQueryData = {
      language: 'en-US',
      translations: {
        newNamespace: {
          key: 'value',
        },
      },
    };

    render(
      <I18nProvider>
        <div>Test</div>
      </I18nProvider>,
    );

    expect(mockAddResourceBundle).toHaveBeenCalledWith('en-US', 'newNamespace', {key: 'value'}, true, true);
  });

  it('should register cache invalidator on mount', () => {
    render(
      <I18nProvider>
        <div>Test</div>
      </I18nProvider>,
    );

    // Call the registered invalidator
    invalidateI18nCache();

    expect(mockInvalidateQueries).toHaveBeenCalledWith({
      queryKey: ['i18n-translations'],
    });
  });

  it('should unregister cache invalidator on unmount', () => {
    const {unmount} = render(
      <I18nProvider>
        <div>Test</div>
      </I18nProvider>,
    );

    // Verify it's registered first
    invalidateI18nCache();
    expect(mockInvalidateQueries).toHaveBeenCalledTimes(1);

    // Unmount and verify invalidator is unregistered
    unmount();
    mockInvalidateQueries.mockClear();

    invalidateI18nCache();
    expect(mockInvalidateQueries).not.toHaveBeenCalled();
  });

  it('should handle invalidateQueries rejection gracefully', () => {
    mockInvalidateQueries.mockRejectedValueOnce(new Error('Query invalidation failed'));

    render(
      <I18nProvider>
        <div>Test</div>
      </I18nProvider>,
    );

    // Should not throw when invalidation fails
    expect(() => invalidateI18nCache()).not.toThrow();
  });

  it('should add multiple namespaces from API response', () => {
    mockQueryData = {
      language: 'en-US',
      translations: {
        common: {greeting: 'Hello'},
        errors: {notFound: 'Not Found'},
        buttons: {submit: 'Submit'},
      },
    };

    render(
      <I18nProvider>
        <div>Test</div>
      </I18nProvider>,
    );

    expect(mockAddResourceBundle).toHaveBeenCalledTimes(3);
    expect(mockAddResourceBundle).toHaveBeenCalledWith('en-US', 'common', expect.any(Object), true, true);
    expect(mockAddResourceBundle).toHaveBeenCalledWith('en-US', 'errors', expect.any(Object), true, true);
    expect(mockAddResourceBundle).toHaveBeenCalledWith('en-US', 'buttons', expect.any(Object), true, true);
  });

  it('should skip null namespace translations', () => {
    mockQueryData = {
      language: 'en-US',
      translations: {
        validNamespace: {key: 'value'},
        // eslint-disable-next-line @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-assignment
        nullNamespace: null as any,
      },
    };

    render(
      <I18nProvider>
        <div>Test</div>
      </I18nProvider>,
    );

    expect(mockAddResourceBundle).toHaveBeenCalledTimes(1);
    expect(mockAddResourceBundle).toHaveBeenCalledWith(
      'en-US',
      'validNamespace',
      expect.objectContaining({key: 'value'}),
      true,
      true,
    );
  });

  describe('queryFn', () => {
    it('should fetch translations from the API with correct URL and options', async () => {
      const expectedResponse = {
        language: 'en-US',
        translations: {
          testNamespace: {key: 'value'},
        },
      };
      mockHttpRequest.mockResolvedValueOnce({data: expectedResponse});

      render(
        <I18nProvider>
          <div>Test</div>
        </I18nProvider>,
      );

      expect(capturedQueryFn).toBeDefined();

      // Execute the captured queryFn
      const result = await capturedQueryFn!();

      expect(mockHttpRequest).toHaveBeenCalledWith({
        url: 'https://api.example.com/i18n/languages/en-US/translations/resolve',
        method: 'GET',
        attachToken: false,
        withCredentials: false,
      });
      expect(result).toEqual(expectedResponse);
    });

    it('should return response data from the API', async () => {
      const translationsData = {
        language: 'en-US',
        totalResults: 10,
        translations: {
          common: {hello: 'Hello', world: 'World'},
        },
      };
      mockHttpRequest.mockResolvedValueOnce({data: translationsData});

      render(
        <I18nProvider>
          <div>Test</div>
        </I18nProvider>,
      );

      const result = await capturedQueryFn!();

      expect(result).toEqual(translationsData);
    });
  });
});
