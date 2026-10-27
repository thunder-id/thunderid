// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderHook} from '@testing-library/react';
import {DEFAULT_ADMIN_APP_CONFIG, type AppConfig} from '@thunderid/system-apps';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import useAdminAppConfig from '../hooks/useAdminAppConfig';

const mockState: {config: AppConfig | undefined; granted: string[]; permissionsLoading: boolean} = {
  config: undefined,
  granted: [],
  permissionsLoading: false,
};

vi.mock('@thunderid/system-apps', async (importOriginal) => ({
  ...(await importOriginal<Record<string, unknown>>()),
  useGetAppConfig: () => ({data: mockState.config, isLoading: false}),
}));

vi.mock('@thunderid/react', () => ({
  useGrantedPermissions: () => ({
    can: (required: string) =>
      mockState.granted.some((permission) => permission === required || required.startsWith(`${permission}:`)),
    isLoading: mockState.permissionsLoading,
    permissions: mockState.granted,
  }),
}));

const withPages = (overrides: Record<string, Partial<AppConfig['pages'][number]>>): AppConfig => ({
  ...DEFAULT_ADMIN_APP_CONFIG,
  pages: DEFAULT_ADMIN_APP_CONFIG.pages.map((page) => ({...page, ...overrides[page.key]})),
});

describe('useAdminAppConfig', () => {
  beforeEach(() => {
    mockState.config = undefined;
    mockState.granted = [];
    mockState.permissionsLoading = false;
  });

  it('shows every page of the default configuration until one is saved', () => {
    const {result} = renderHook(() => useAdminAppConfig());

    expect(result.current.visibleResources).toEqual(['users', 'groups', 'roles', 'agents']);
  });

  it('leaves out pages that are off', () => {
    mockState.config = withPages({agents: {enabled: false}, roles: {enabled: false}});

    const {result} = renderHook(() => useAdminAppConfig());

    expect(result.current.visibleResources).toEqual(['users', 'groups']);
  });

  it('shows a by-permission page only when the token covers its view permission', () => {
    mockState.config = withPages({groups: {visibility: 'BY_PERMISSION'}, roles: {visibility: 'BY_PERMISSION'}});
    mockState.granted = ['system:group'];

    const {result} = renderHook(() => useAdminAppConfig());

    expect(result.current.visibleResources).toEqual(['users', 'groups', 'agents']);
  });

  it('reports loading while the permissions are read', () => {
    mockState.permissionsLoading = true;

    const {result} = renderHook(() => useAdminAppConfig());

    expect(result.current.isLoading).toBe(true);
  });
});
