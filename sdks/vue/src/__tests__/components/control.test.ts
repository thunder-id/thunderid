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
import {h, ref} from 'vue';
import Loading from '../../components/control/Loading';
import SignedIn from '../../components/control/SignedIn';
import SignedOut from '../../components/control/SignedOut';
import {THUNDERID_KEY} from '../../keys';
import type {ThunderIDContext} from '../../models/contexts';

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

describe('SignedIn', () => {
  it('should render default slot content when user is signed in', () => {
    const mockContext = createMockThunderIDContext({isSignedIn: ref(true)});

    const wrapper = mount(SignedIn, {
      global: {
        provide: {
          [THUNDERID_KEY as symbol]: mockContext,
        },
      },
      slots: {
        default: () => h('div', {class: 'dashboard'}, 'Dashboard'),
      },
    });

    expect(wrapper.find('.dashboard').exists()).toBe(true);
    expect(wrapper.text()).toBe('Dashboard');
  });

  it('should not render default slot content when user is not signed in', () => {
    const mockContext = createMockThunderIDContext({isSignedIn: ref(false)});

    const wrapper = mount(SignedIn, {
      global: {
        provide: {
          [THUNDERID_KEY as symbol]: mockContext,
        },
      },
      slots: {
        default: () => h('div', {class: 'dashboard'}, 'Dashboard'),
      },
    });

    expect(wrapper.find('.dashboard').exists()).toBe(false);
  });

  it('should render fallback slot when user is not signed in', () => {
    const mockContext = createMockThunderIDContext({isSignedIn: ref(false)});

    const wrapper = mount(SignedIn, {
      global: {
        provide: {
          [THUNDERID_KEY as symbol]: mockContext,
        },
      },
      slots: {
        default: () => h('div', {class: 'dashboard'}, 'Dashboard'),
        fallback: () => h('div', {class: 'fallback'}, 'Please sign in'),
      },
    });

    expect(wrapper.find('.dashboard').exists()).toBe(false);
    expect(wrapper.find('.fallback').exists()).toBe(true);
    expect(wrapper.text()).toBe('Please sign in');
  });

  it('should react to auth state changes', async () => {
    const isSignedIn = ref(false);
    const mockContext = createMockThunderIDContext({isSignedIn});

    const wrapper = mount(SignedIn, {
      global: {
        provide: {
          [THUNDERID_KEY as symbol]: mockContext,
        },
      },
      slots: {
        default: () => h('div', {class: 'dashboard'}, 'Dashboard'),
      },
    });

    expect(wrapper.find('.dashboard').exists()).toBe(false);

    isSignedIn.value = true;
    await wrapper.vm.$nextTick();

    expect(wrapper.find('.dashboard').exists()).toBe(true);
  });
});

describe('SignedOut', () => {
  it('should render default slot content when user is not signed in', () => {
    const mockContext = createMockThunderIDContext({isSignedIn: ref(false)});

    const wrapper = mount(SignedOut, {
      global: {
        provide: {
          [THUNDERID_KEY as symbol]: mockContext,
        },
      },
      slots: {
        default: () => h('div', {class: 'landing'}, 'Welcome'),
      },
    });

    expect(wrapper.find('.landing').exists()).toBe(true);
    expect(wrapper.text()).toBe('Welcome');
  });

  it('should not render default slot content when user is signed in', () => {
    const mockContext = createMockThunderIDContext({isSignedIn: ref(true)});

    const wrapper = mount(SignedOut, {
      global: {
        provide: {
          [THUNDERID_KEY as symbol]: mockContext,
        },
      },
      slots: {
        default: () => h('div', {class: 'landing'}, 'Welcome'),
      },
    });

    expect(wrapper.find('.landing').exists()).toBe(false);
  });

  it('should render fallback slot when user is signed in', () => {
    const mockContext = createMockThunderIDContext({isSignedIn: ref(true)});

    const wrapper = mount(SignedOut, {
      global: {
        provide: {
          [THUNDERID_KEY as symbol]: mockContext,
        },
      },
      slots: {
        default: () => h('div', {class: 'landing'}, 'Welcome'),
        fallback: () => h('div', {class: 'fallback'}, 'Already signed in'),
      },
    });

    expect(wrapper.find('.landing').exists()).toBe(false);
    expect(wrapper.find('.fallback').exists()).toBe(true);
  });

  it('should react to auth state changes', async () => {
    const isSignedIn = ref(false);
    const mockContext = createMockThunderIDContext({isSignedIn});

    const wrapper = mount(SignedOut, {
      global: {
        provide: {
          [THUNDERID_KEY as symbol]: mockContext,
        },
      },
      slots: {
        default: () => h('div', {class: 'landing'}, 'Welcome'),
      },
    });

    expect(wrapper.find('.landing').exists()).toBe(true);

    isSignedIn.value = true;
    await wrapper.vm.$nextTick();

    expect(wrapper.find('.landing').exists()).toBe(false);
  });
});

describe('Loading', () => {
  it('should render default slot content when loading', () => {
    const mockContext = createMockThunderIDContext({isLoading: ref(true)});

    const wrapper = mount(Loading, {
      global: {
        provide: {
          [THUNDERID_KEY as symbol]: mockContext,
        },
      },
      slots: {
        default: () => h('div', {class: 'spinner'}, 'Loading...'),
      },
    });

    expect(wrapper.find('.spinner').exists()).toBe(true);
    expect(wrapper.text()).toBe('Loading...');
  });

  it('should not render default slot content when not loading', () => {
    const mockContext = createMockThunderIDContext({isLoading: ref(false)});

    const wrapper = mount(Loading, {
      global: {
        provide: {
          [THUNDERID_KEY as symbol]: mockContext,
        },
      },
      slots: {
        default: () => h('div', {class: 'spinner'}, 'Loading...'),
      },
    });

    expect(wrapper.find('.spinner').exists()).toBe(false);
  });

  it('should render fallback slot when not loading', () => {
    const mockContext = createMockThunderIDContext({isLoading: ref(false)});

    const wrapper = mount(Loading, {
      global: {
        provide: {
          [THUNDERID_KEY as symbol]: mockContext,
        },
      },
      slots: {
        default: () => h('div', {class: 'spinner'}, 'Loading...'),
        fallback: () => h('div', {class: 'content'}, 'Content loaded'),
      },
    });

    expect(wrapper.find('.spinner').exists()).toBe(false);
    expect(wrapper.find('.content').exists()).toBe(true);
  });

  it('should react to loading state changes', async () => {
    const isLoading = ref(true);
    const mockContext = createMockThunderIDContext({isLoading});

    const wrapper = mount(Loading, {
      global: {
        provide: {
          [THUNDERID_KEY as symbol]: mockContext,
        },
      },
      slots: {
        default: () => h('div', {class: 'spinner'}, 'Loading...'),
        fallback: () => h('div', {class: 'content'}, 'Content loaded'),
      },
    });

    expect(wrapper.find('.spinner').exists()).toBe(true);
    expect(wrapper.find('.content').exists()).toBe(false);

    isLoading.value = false;
    await wrapper.vm.$nextTick();

    expect(wrapper.find('.spinner').exists()).toBe(false);
    expect(wrapper.find('.content').exists()).toBe(true);
  });
});
