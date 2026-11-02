// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderHook, waitFor} from '@thunderid/test-utils';
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest';
import useCreateTrustedIssuer from '../useCreateTrustedIssuer';

const mockHttpRequest = vi.fn();
const showToast = vi.fn();

vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({http: {request: mockHttpRequest}}),
}));
vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: () => ({getServerUrl: () => 'https://localhost:8090'}),
    useToast: () => ({showToast}),
  };
});

const request = {
  ouId: 'ou-1',
  name: 'Acme Okta',
  issuer: 'https://acme.okta.com',
  jwksEndpoint: 'https://acme.okta.com/jwks',
  idJagEnabled: true,
};

describe('useCreateTrustedIssuer', () => {
  beforeEach(() => {
    mockHttpRequest.mockReset().mockResolvedValue({
      data: {
        id: 'ti-1',
        type: 'oidc',
        name: 'Acme Okta',
        ouId: 'ou-1',
        issuer: 'https://acme.okta.com',
        jwksEndpoint: 'https://acme.okta.com/jwks',
        idJagEnabled: true,
      },
    });
    showToast.mockReset();
  });

  afterEach(() => vi.clearAllMocks());

  it('POSTs the owning organization unit to /connections/oidc, maps the response and toasts success', async () => {
    const {result} = renderHook(() => useCreateTrustedIssuer());
    result.current.mutate(request);

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({url: 'https://localhost:8090/connections/oidc', method: 'POST', data: request}),
    );
    expect(result.current.data).toMatchObject({id: 'ti-1', ouId: 'ou-1', idJagEnabled: true});
    expect(showToast).toHaveBeenCalledWith(expect.any(String), 'success');
  });

  it('does not toast on a failure (surfaced inline by the caller)', async () => {
    mockHttpRequest.mockRejectedValue({response: {status: 409}});
    const {result} = renderHook(() => useCreateTrustedIssuer());
    result.current.mutate(request);

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(showToast).not.toHaveBeenCalled();
  });
});
