// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {QueryClient, QueryClientProvider} from '@tanstack/react-query';
import {renderHook, waitFor} from '@testing-library/react';
import type {JSX, ReactNode} from 'react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import useOrganizationName from '../hooks/useOrganizationName';

const profile: Record<string, unknown> = {};
const getOrganizationUnit = vi.fn();

vi.mock('@thunderid/react', () => ({
  getOrganizationUnit: (config: {ouId: string}): unknown => getOrganizationUnit(config),
  useManagementRequestConfig: () => ({baseUrl: 'https://localhost:8090'}),
  useUser: () => ({flattenedProfile: profile}),
}));

const wrapper = ({children}: {children: ReactNode}): JSX.Element => (
  <QueryClientProvider client={new QueryClient({defaultOptions: {queries: {retry: false}}})}>
    {children}
  </QueryClientProvider>
);

describe('useOrganizationName', () => {
  beforeEach(() => {
    for (const key of Object.keys(profile)) delete profile[key];
    getOrganizationUnit.mockReset();
  });

  it('reads the name from the token, then the handle, without a request', () => {
    profile.ouHandle = 'default';
    expect(renderHook(() => useOrganizationName(), {wrapper}).result.current).toBe('default');

    profile.ouName = 'Default';
    expect(renderHook(() => useOrganizationName(), {wrapper}).result.current).toBe('Default');
    expect(getOrganizationUnit).not.toHaveBeenCalled();
  });

  it('looks the organization unit up by its ID when the token carries no name', async () => {
    profile.ouId = 'ou-1';
    getOrganizationUnit.mockResolvedValue({handle: 'default', id: 'ou-1', name: 'Default'});

    const {result} = renderHook(() => useOrganizationName(), {wrapper});

    await waitFor(() => expect(result.current).toBe('Default'));
    expect(getOrganizationUnit).toHaveBeenCalledWith({baseUrl: 'https://localhost:8090', ouId: 'ou-1'});
  });

  it('knows no name when the token names no organization unit', () => {
    expect(renderHook(() => useOrganizationName(), {wrapper}).result.current).toBeUndefined();
    expect(getOrganizationUnit).not.toHaveBeenCalled();
  });
});
