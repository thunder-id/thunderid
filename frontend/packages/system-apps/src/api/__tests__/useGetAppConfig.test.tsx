// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {waitFor} from '@testing-library/react';
import {renderHook} from '@thunderid/test-utils';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {DEFAULT_ADMIN_APP_CONFIG} from '../../constants/app-catalog';
import useGetAppConfig from '../useGetAppConfig';

const mockRequest = vi.fn();

vi.mock('@thunderid/react', async (importOriginal) => ({
  ...(await importOriginal<Record<string, unknown>>()),
  useThunderID: () => ({http: {request: mockRequest}}),
}));

describe('useGetAppConfig', () => {
  beforeEach(() => {
    mockRequest.mockReset();
    mockRequest.mockResolvedValue({data: {merged: DEFAULT_ADMIN_APP_CONFIG, readOnly: null, writable: null}});
  });

  it("reads the app's server-config section and returns the configuration in effect", async () => {
    const {result} = renderHook(() => useGetAppConfig('ADMIN'));

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    const [request] = mockRequest.mock.calls[0] as [{method: string; url: string}];
    expect(request.method).toBe('GET');
    expect(request.url).toMatch(/\/server-config\/system-apps\/admin$/);
    expect(result.current.data).toEqual(DEFAULT_ADMIN_APP_CONFIG);
  });

  it("reads the Account App's own section", async () => {
    const {result} = renderHook(() => useGetAppConfig('ACCOUNT'));

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    const [request] = mockRequest.mock.calls[0] as [{url: string}];
    expect(request.url).toMatch(/\/server-config\/system-apps\/account$/);
  });

  it('waits while it is not enabled', () => {
    renderHook(() => useGetAppConfig('ADMIN', {enabled: false}));

    expect(mockRequest).not.toHaveBeenCalled();
  });
});
