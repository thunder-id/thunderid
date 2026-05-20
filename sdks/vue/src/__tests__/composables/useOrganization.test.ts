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
import {defineComponent, h, ref, shallowRef} from 'vue';
import useOrganization from '../../composables/useOrganization';
import {ORGANIZATION_KEY} from '../../keys';
import type {OrganizationContextValue} from '../../models/contexts';

function createMockOrganizationContext(overrides: Partial<OrganizationContextValue> = {}): OrganizationContextValue {
  return {
    myOrganizations: shallowRef([]),
    currentOrganization: shallowRef(null),
    switchOrganization: vi.fn(),
    getAllOrganizations: vi.fn(),
    revalidateMyOrganizations: vi.fn(),
    isLoading: ref(false),
    error: ref(null),
    ...overrides,
  } as unknown as OrganizationContextValue;
}

describe('useOrganization', () => {
  it('should return the OrganizationContextValue when called inside a provider', () => {
    const mockContext = createMockOrganizationContext();
    let result: OrganizationContextValue | undefined;

    const TestChild = defineComponent({
      setup() {
        result = useOrganization();
        return () => h('div', 'test');
      },
    });

    mount(TestChild, {
      global: {
        provide: {
          [ORGANIZATION_KEY as symbol]: mockContext,
        },
      },
    });

    expect(result).toBeDefined();
    expect(result.myOrganizations.value).toEqual([]);
    expect(result.currentOrganization.value).toBeNull();
  });

  it('should throw an error when called outside of ThunderIDProvider', () => {
    const TestChild = defineComponent({
      setup() {
        useOrganization();
        return () => h('div', 'test');
      },
    });

    expect(() => {
      mount(TestChild);
    }).toThrow('[ThunderID] useOrganization() was called outside of <ThunderIDProvider>');
  });

  it('should expose organization management methods', () => {
    const mockContext = createMockOrganizationContext();
    let result: OrganizationContextValue | undefined;

    const TestChild = defineComponent({
      setup() {
        result = useOrganization();
        return () => h('div', 'test');
      },
    });

    mount(TestChild, {
      global: {
        provide: {
          [ORGANIZATION_KEY as symbol]: mockContext,
        },
      },
    });

    expect(typeof result.switchOrganization).toBe('function');
    expect(typeof result.getAllOrganizations).toBe('function');
    expect(typeof result.revalidateMyOrganizations).toBe('function');
  });
});
