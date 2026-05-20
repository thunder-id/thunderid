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
import {defineComponent, h, shallowRef} from 'vue';
import useUser from '../../composables/useUser';
import {USER_KEY} from '../../keys';
import type {UserContextValue} from '../../models/contexts';

function createMockUserContext(overrides: Partial<UserContextValue> = {}): UserContextValue {
  return {
    profile: shallowRef(null),
    flattenedProfile: shallowRef(null),
    schemas: shallowRef([]),
    updateProfile: vi.fn(),
    revalidateProfile: vi.fn(),
    ...overrides,
  } as unknown as UserContextValue;
}

describe('useUser', () => {
  it('should return the UserContextValue when called inside a provider', () => {
    const mockContext = createMockUserContext();
    let result: UserContextValue | undefined;

    const TestChild = defineComponent({
      setup() {
        result = useUser();
        return () => h('div', 'test');
      },
    });

    mount(TestChild, {
      global: {
        provide: {
          [USER_KEY as symbol]: mockContext,
        },
      },
    });

    expect(result).toBeDefined();
    expect(result.profile.value).toBeNull();
  });

  it('should throw an error when called outside of ThunderIDProvider', () => {
    const TestChild = defineComponent({
      setup() {
        useUser();
        return () => h('div', 'test');
      },
    });

    expect(() => {
      mount(TestChild);
    }).toThrow('[ThunderID] useUser() was called outside of <ThunderIDProvider>');
  });

  it('should expose updateProfile and revalidateProfile methods', () => {
    const mockContext = createMockUserContext();
    let result: UserContextValue | undefined;

    const TestChild = defineComponent({
      setup() {
        result = useUser();
        return () => h('div', 'test');
      },
    });

    mount(TestChild, {
      global: {
        provide: {
          [USER_KEY as symbol]: mockContext,
        },
      },
    });

    expect(typeof result.updateProfile).toBe('function');
    expect(typeof result.revalidateProfile).toBe('function');
  });
});
