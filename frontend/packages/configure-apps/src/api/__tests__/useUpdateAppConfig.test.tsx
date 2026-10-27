// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {waitFor} from '@testing-library/react';
import {DEFAULT_ADMIN_APP_CONFIG} from '@thunderid/system-apps';
import {renderHook} from '@thunderid/test-utils';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import useUpdateAppConfig from '../useUpdateAppConfig';

const mockRequest = vi.fn();

vi.mock('@thunderid/react', async (importOriginal) => ({
  ...(await importOriginal<Record<string, unknown>>()),
  useThunderID: () => ({http: {request: mockRequest}}),
}));

describe('useUpdateAppConfig', () => {
  beforeEach(() => {
    mockRequest.mockReset();
    mockRequest.mockResolvedValue({data: {merged: DEFAULT_ADMIN_APP_CONFIG, readOnly: null, writable: null}});
  });

  it("sends only the app's configuration to its server-config section", async () => {
    const {result} = renderHook(() => useUpdateAppConfig());

    result.current.mutate({
      ...DEFAULT_ADMIN_APP_CONFIG,
      appTheme: {borderRadius: 12},
      applicationId: 'app-1',
      gate: {themeId: 'theme-1'},
    });

    await waitFor(() => expect(mockRequest).toHaveBeenCalled());
    const [request] = mockRequest.mock.calls[0] as [{data: Record<string, unknown>; method: string; url: string}];
    expect(request.method).toBe('PUT');
    expect(request.url).toMatch(/\/server-config\/system-apps\/admin$/);
    expect(Object.keys(request.data).sort()).toEqual(['appTheme', 'branding', 'content', 'navigation', 'pages']);
  });

  it('returns the configuration in effect after the save', async () => {
    const {result} = renderHook(() => useUpdateAppConfig());

    result.current.mutate(DEFAULT_ADMIN_APP_CONFIG);

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toEqual(DEFAULT_ADMIN_APP_CONFIG);
  });
});
