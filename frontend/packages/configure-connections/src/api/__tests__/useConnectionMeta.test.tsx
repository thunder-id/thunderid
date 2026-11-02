// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderHook, waitFor} from '@thunderid/test-utils';
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest';
import useConnectionMeta from '../useConnectionMeta';

const mockHttpRequest = vi.fn();

vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({http: {request: mockHttpRequest}}),
}));
vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {...actual, useConfig: () => ({getServerUrl: () => 'https://localhost:8090'})};
});

const SMTP_META = {
  vendor: 'email-smtp',
  authentication: {
    methods: [
      {type: 'none', displayName: 'None', fields: []},
      {
        type: 'basic',
        displayName: 'Username and Password',
        fields: [
          {key: 'username', type: 'string', required: true, displayName: 'Username'},
          {key: 'password', type: 'string', required: true, credential: true, displayName: 'Password'},
        ],
      },
    ],
  },
};

describe('useConnectionMeta', () => {
  beforeEach(() => {
    mockHttpRequest.mockReset().mockResolvedValue({data: {vendors: [SMTP_META]}});
  });

  afterEach(() => vi.clearAllMocks());

  it('requests the metadata filtered to the given vendor', async () => {
    const {result} = renderHook(() => useConnectionMeta('email-smtp'));

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        url: 'https://localhost:8090/connections/meta?vendor=email-smtp',
        method: 'GET',
      }),
    );
  });

  // The server answers with a list in both modes, so the hook unwraps the requested vendor's entry
  // and callers read its methods directly.
  it("unwraps the requested vendor's entry from the list", async () => {
    const {result} = renderHook(() => useConnectionMeta('email-smtp'));

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.data).toEqual(SMTP_META);
    expect(result.current.data?.authentication.methods[1].fields?.map((field) => field.key)).toEqual([
      'username',
      'password',
    ]);
  });

  it('yields no entry when the server does not describe the vendor', async () => {
    mockHttpRequest.mockResolvedValue({data: {vendors: []}});
    const {result} = renderHook(() => useConnectionMeta('email-smtp'));

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.data).toBeUndefined();
  });

  it('does not request anything without a vendor', () => {
    renderHook(() => useConnectionMeta(undefined));

    expect(mockHttpRequest).not.toHaveBeenCalled();
  });
});
