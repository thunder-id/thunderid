// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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
