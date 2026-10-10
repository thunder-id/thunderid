// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {waitFor} from '@testing-library/react';
import {renderHook} from '@thunderid/test-utils';
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest';
import {createHttpRouter, SERVER_URL, type HttpCall} from '../../__tests__/http';
import GatewayQueryKeys from '../../constants/gateway-query-keys';
import useDeleteGateway from '../useDeleteGateway';
import useGetGateway from '../useGetGateway';
import useGetGateways from '../useGetGateways';
import useRegisterGateway from '../useRegisterGateway';
import useUpdateGateway from '../useUpdateGateway';

const mockHttpRequest = vi.fn<(call: HttpCall) => Promise<unknown>>();
vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({http: {request: mockHttpRequest}}),
}));

const mockShowToast = vi.fn();
vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: () => ({getServerUrl: () => SERVER_URL}),
    useToast: () => ({showToast: mockShowToast}),
  };
});

const gateway = {id: 'gw-1', name: 'production', baseUrl: 'https://dp.example.com'};

describe('gateway API hooks', () => {
  beforeEach(() => {
    mockHttpRequest.mockReset();
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('lists gateways, treating an empty body as no gateways', async () => {
    mockHttpRequest.mockImplementation(createHttpRouter({'GET /gateways': null}));

    const {result} = renderHook(() => useGetGateways());

    await waitFor(() => expect(result.current.data).toEqual([]));
  });

  it('gets one gateway, and waits for an id', async () => {
    mockHttpRequest.mockImplementation(createHttpRouter({'GET /gateways/gw-1': gateway}));

    const {result: idle} = renderHook(() => useGetGateway(''));
    expect(idle.current.fetchStatus).toBe('idle');

    const {result} = renderHook(() => useGetGateway('gw-1'));
    await waitFor(() => expect(result.current.data).toEqual(gateway));
  });

  it('registers a gateway and refreshes the listing', async () => {
    const registration = {...gateway, key: 'generated-key'};
    mockHttpRequest.mockImplementation(createHttpRouter({'POST /gateways': registration}));

    const {result, queryClient} = renderHook(() => useRegisterGateway());
    const invalidate = vi.spyOn(queryClient, 'invalidateQueries');

    result.current.mutate({name: 'production', baseUrl: gateway.baseUrl});

    await waitFor(() => expect(result.current.data).toEqual(registration));

    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({method: 'POST', data: {name: 'production', baseUrl: gateway.baseUrl}}),
    );
    expect(invalidate).toHaveBeenCalledWith({queryKey: [GatewayQueryKeys.GATEWAYS]});
  });

  it('updates a gateway, caches the result and confirms the edit', async () => {
    mockHttpRequest.mockImplementation(createHttpRouter({'PUT /gateways/gw-1': {...gateway, name: 'prod'}}));

    const {result, queryClient} = renderHook(() => useUpdateGateway());

    result.current.mutate({id: 'gw-1', data: {name: 'prod'}});

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(queryClient.getQueryData([GatewayQueryKeys.GATEWAY, 'gw-1'])).toEqual({...gateway, name: 'prod'});
    expect(mockShowToast).toHaveBeenCalledWith('Gateway updated.', 'success');
  });

  it('confirms a key rotation in its own words', async () => {
    mockHttpRequest.mockImplementation(createHttpRouter({'PUT /gateways/gw-1': gateway}));

    const {result} = renderHook(() => useUpdateGateway());

    result.current.mutate({id: 'gw-1', data: {key: 'new-key'}});

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(mockShowToast).toHaveBeenCalledWith('The key was rotated.', 'success');
  });

  it('removes a gateway', async () => {
    mockHttpRequest.mockImplementation(createHttpRouter({'DELETE /gateways/gw-1': null}));

    const {result, queryClient} = renderHook(() => useDeleteGateway());
    const invalidate = vi.spyOn(queryClient, 'invalidateQueries');

    result.current.mutate('gw-1');

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(invalidate).toHaveBeenCalledWith({queryKey: [GatewayQueryKeys.GATEWAYS]});
    expect(mockShowToast).toHaveBeenCalledWith('Gateway removed.', 'success');
  });
});
