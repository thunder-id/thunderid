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

import {mount} from '@vue/test-utils';

import {describe, expect, it, vi} from 'vitest';
import {defineComponent, h, ref} from 'vue';
import useThunderID from '../../composables/useThunderID';
import {THUNDERID_KEY} from '../../keys';
import type {ThunderIDContext} from '../../models/contexts';

/**
 * Creates a minimal mock ThunderIDContext for testing purposes.
 */
function createMockThunderIDContext(overrides: Partial<ThunderIDContext> = {}): ThunderIDContext {
  return {
    isSignedIn: ref(false),
    isLoading: ref(false),
    isInitialized: ref(true),
    user: ref(null),
    organization: ref(null),
    signIn: vi.fn(),
    signOut: vi.fn(),
    signUp: vi.fn(),
    signInSilently: vi.fn(),
    getAccessToken: vi.fn(),
    getDecodedIdToken: vi.fn(),
    getIdToken: vi.fn(),
    exchangeToken: vi.fn(),
    reInitialize: vi.fn(),
    clearSession: vi.fn(),
    http: {
      request: vi.fn(),
      requestAll: vi.fn(),
    },
    ...overrides,
  } as unknown as ThunderIDContext;
}

describe('useThunderID', () => {
  it('should return the ThunderIDContext when called inside a provider', () => {
    const mockContext = createMockThunderIDContext();
    let result: ThunderIDContext | undefined;

    const TestChild = defineComponent({
      setup() {
        result = useThunderID();
        return () => h('div', 'test');
      },
    });

    mount(TestChild, {
      global: {
        provide: {
          [THUNDERID_KEY as symbol]: mockContext,
        },
      },
    });

    expect(result).toBeDefined();
    expect(result.isSignedIn.value).toBe(false);
    expect(result.isLoading.value).toBe(false);
    expect(result.isInitialized.value).toBe(true);
  });

  it('should throw an error when called outside of ThunderIDProvider', () => {
    const TestChild = defineComponent({
      setup() {
        useThunderID();
        return () => h('div', 'test');
      },
    });

    expect(() => {
      mount(TestChild);
    }).toThrow('[ThunderID] useThunderID() was called outside of <ThunderIDProvider>');
  });

  it('should return reactive auth state', () => {
    const isSignedIn = ref(false);
    const mockContext = createMockThunderIDContext({isSignedIn});
    let result: ThunderIDContext | undefined;

    const TestChild = defineComponent({
      setup() {
        result = useThunderID();
        return () => h('div', 'test');
      },
    });

    mount(TestChild, {
      global: {
        provide: {
          [THUNDERID_KEY as symbol]: mockContext,
        },
      },
    });

    expect(result.isSignedIn.value).toBe(false);
    isSignedIn.value = true;
    expect(result.isSignedIn.value).toBe(true);
  });

  it('should expose signIn and signOut methods', () => {
    const mockContext = createMockThunderIDContext();
    let result: ThunderIDContext | undefined;

    const TestChild = defineComponent({
      setup() {
        result = useThunderID();
        return () => h('div', 'test');
      },
    });

    mount(TestChild, {
      global: {
        provide: {
          [THUNDERID_KEY as symbol]: mockContext,
        },
      },
    });

    expect(typeof result.signIn).toBe('function');
    expect(typeof result.signOut).toBe('function');
    expect(typeof result.signUp).toBe('function');
    expect(typeof result.getAccessToken).toBe('function');
  });
});
