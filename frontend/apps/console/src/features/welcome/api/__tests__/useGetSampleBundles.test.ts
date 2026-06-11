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

import {renderHook} from '@testing-library/react';
import {describe, expect, it, vi} from 'vitest';

const mockBundles = {
  wayfinder: {configs: {declarative: 'yaml content', env: 'KEY=value'}},
  other: {configs: {declarative: 'other yaml'}},
};

vi.mock('../../data/sampleBundles', () => ({
  SAMPLE_BUNDLES: {
    wayfinder: {configs: {declarative: 'yaml content', env: 'KEY=value'}},
    other: {configs: {declarative: 'other yaml'}},
  },
}));

import {useGetSampleBundles, useGetSampleBundle} from '../useGetSampleBundles';

describe('useGetSampleBundles', () => {
  it('returns all sample bundles', () => {
    const {result} = renderHook(() => useGetSampleBundles());
    expect(result.current).toEqual(mockBundles);
  });

  it('returns a stable reference across renders', () => {
    const {result, rerender} = renderHook(() => useGetSampleBundles());
    const first = result.current;
    rerender();
    expect(result.current).toBe(first);
  });
});

describe('useGetSampleBundle', () => {
  it('returns the bundle for a known key', () => {
    const {result} = renderHook(() => useGetSampleBundle('wayfinder'));
    expect(result.current).toEqual({configs: {declarative: 'yaml content', env: 'KEY=value'}});
  });

  it('returns undefined for an unknown key', () => {
    const {result} = renderHook(() => useGetSampleBundle('nonexistent'));
    expect(result.current).toBeUndefined();
  });

  it('returns a different bundle when the key changes', () => {
    const {result, rerender} = renderHook(({key}: {key: string}) => useGetSampleBundle(key), {
      initialProps: {key: 'wayfinder'},
    });
    expect(result.current?.configs.declarative).toBe('yaml content');

    rerender({key: 'other'});
    expect(result.current?.configs.declarative).toBe('other yaml');
  });

  it('returns a stable reference when the key does not change', () => {
    const {result, rerender} = renderHook(() => useGetSampleBundle('wayfinder'));
    const first = result.current;
    rerender();
    expect(result.current).toBe(first);
  });
});
