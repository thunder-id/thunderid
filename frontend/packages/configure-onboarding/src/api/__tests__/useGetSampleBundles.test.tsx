// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderHook} from '@testing-library/react';
import type {ReactNode} from 'react';
import {describe, expect, it} from 'vitest';
import SampleBundlesProvider from '../../contexts/SampleBundlesProvider';
import type {SampleBundle} from '../../utils/sampleBundles';
import {useGetSampleBundles, useGetSampleBundle} from '../useGetSampleBundles';

const bundles: Record<string, SampleBundle> = {
  wayfinder: {configs: {declarative: 'yaml content', env: 'KEY=value'}},
  other: {configs: {declarative: 'other yaml'}},
};

const wrapper = ({children}: {children: ReactNode}) => (
  <SampleBundlesProvider bundles={bundles}>{children}</SampleBundlesProvider>
);

describe('useGetSampleBundles', () => {
  it('returns the bundles the host supplied', () => {
    const {result} = renderHook(() => useGetSampleBundles(), {wrapper});
    expect(result.current).toBe(bundles);
  });

  it('returns no bundles without a provider', () => {
    const {result} = renderHook(() => useGetSampleBundles());
    expect(result.current).toEqual({});
  });
});

describe('useGetSampleBundle', () => {
  it('returns the bundle for a known key', () => {
    const {result} = renderHook(() => useGetSampleBundle('wayfinder'), {wrapper});
    expect(result.current).toEqual({configs: {declarative: 'yaml content', env: 'KEY=value'}});
  });

  it('returns undefined for an unknown key', () => {
    const {result} = renderHook(() => useGetSampleBundle('nonexistent'), {wrapper});
    expect(result.current).toBeUndefined();
  });

  it('returns a different bundle when the key changes', () => {
    const {result, rerender} = renderHook(({key}: {key: string}) => useGetSampleBundle(key), {
      initialProps: {key: 'wayfinder'},
      wrapper,
    });
    expect(result.current?.configs.declarative).toBe('yaml content');

    rerender({key: 'other'});
    expect(result.current?.configs.declarative).toBe('other yaml');
  });
});
