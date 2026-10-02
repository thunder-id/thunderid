// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {waitFor} from '@testing-library/react';
import {renderHook} from '@thunderid/test-utils';
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest';
import {createHttpRouter, SERVER_URL, type HttpCall} from '../../__tests__/http';
import GatewayQueryKeys from '../../constants/gateway-query-keys';
import useApplyDryRun from '../useApplyDryRun';
import useApplyVersion from '../useApplyVersion';
import useCaptureConfigurationVersion from '../useCaptureConfigurationVersion';
import useCreateGatewaySecret from '../useCreateGatewaySecret';
import useCreateGatewayVariable from '../useCreateGatewayVariable';
import useDeleteGateway from '../useDeleteGateway';
import useDeleteGatewaySecret from '../useDeleteGatewaySecret';
import useDeleteGatewayVariable from '../useDeleteGatewayVariable';
import useGetAppliedVersion from '../useGetAppliedVersion';
import useGetConfigurationVersions from '../useGetConfigurationVersions';
import useGetGateway from '../useGetGateway';
import useGetGatewayDiff from '../useGetGatewayDiff';
import useGetGateways from '../useGetGateways';
import useGetGatewaySecrets from '../useGetGatewaySecrets';
import useGetGatewayVariables from '../useGetGatewayVariables';
import useRegisterGateway from '../useRegisterGateway';
import useRevertDryRun from '../useRevertDryRun';
import useRevertGateway from '../useRevertGateway';
import useUpdateGateway from '../useUpdateGateway';
import useUpdateGatewaySecret from '../useUpdateGatewaySecret';
import useUpdateGatewayVariable from '../useUpdateGatewayVariable';

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

  it('lists configuration versions', async () => {
    mockHttpRequest.mockImplementation(createHttpRouter({'GET /configuration-versions': [{version: 2}, {version: 1}]}));

    const {result} = renderHook(() => useGetConfigurationVersions());

    await waitFor(() => expect(result.current.data).toEqual([{version: 2}, {version: 1}]));
  });

  it('treats an empty versions body as no versions', async () => {
    mockHttpRequest.mockImplementation(createHttpRouter({'GET /configuration-versions': null}));

    const {result} = renderHook(() => useGetConfigurationVersions());

    await waitFor(() => expect(result.current.data).toEqual([]));
  });

  it('captures a version and refreshes versions and diffs', async () => {
    mockHttpRequest.mockImplementation(createHttpRouter({'POST /configuration-versions': {version: 3, note: 'n'}}));

    const {result, queryClient} = renderHook(() => useCaptureConfigurationVersion());
    const invalidate = vi.spyOn(queryClient, 'invalidateQueries');

    result.current.mutate({note: 'n'});

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(invalidate).toHaveBeenCalledWith({queryKey: [GatewayQueryKeys.CONFIGURATION_VERSIONS]});
    expect(invalidate).toHaveBeenCalledWith({queryKey: [GatewayQueryKeys.GATEWAY_DIFF]});
    expect(mockShowToast).toHaveBeenCalledWith('Captured version 3.', 'success');
  });

  it('reads the applied version of a gateway', async () => {
    const applied = {gatewayId: 'gw-1', appliedVersion: 2, previousVersion: 1};
    mockHttpRequest.mockImplementation(createHttpRouter({'GET /gateways/gw-1/applied-version': applied}));

    const {result} = renderHook(() => useGetAppliedVersion('gw-1'));

    await waitFor(() => expect(result.current.data).toEqual(applied));
  });

  it('asks for the diff of the chosen version', async () => {
    const diff = {toVersion: 2, summary: {added: 1, updated: 0, deleted: 0, unchanged: 0}, changes: []};
    mockHttpRequest.mockImplementation(createHttpRouter({'GET /gateways/gw-1/diff': diff}));

    const {result} = renderHook(() => useGetGatewayDiff('gw-1', '2'));

    await waitFor(() => expect(result.current.data).toEqual(diff));
    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({url: `${SERVER_URL}/gateways/gw-1/diff?version=2`}),
    );
  });

  it('holds the diff request until enabled', () => {
    const {result} = renderHook(() => useGetGatewayDiff('gw-1', 'latest', false));

    expect(result.current.fetchStatus).toBe('idle');
    expect(mockHttpRequest).not.toHaveBeenCalled();
  });

  it('applies a version and refreshes what the gateway holds when recorded', async () => {
    mockHttpRequest.mockImplementation(
      createHttpRouter({'POST /gateways/gw-1/apply': {gatewayId: 'gw-1', dryRun: false, recorded: true}}),
    );

    const {result, queryClient} = renderHook(() => useApplyVersion());
    const invalidate = vi.spyOn(queryClient, 'invalidateQueries');

    result.current.mutate({gatewayId: 'gw-1', data: {version: 'latest', dryRun: false}});

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({method: 'POST', data: {version: 'latest', dryRun: false}}),
    );
    expect(invalidate).toHaveBeenCalledWith({queryKey: [GatewayQueryKeys.APPLIED_VERSION, 'gw-1']});
  });

  it('refreshes nothing after a dry run', async () => {
    mockHttpRequest.mockImplementation(
      createHttpRouter({'POST /gateways/gw-1/apply': {gatewayId: 'gw-1', dryRun: true, recorded: false}}),
    );

    const {result, queryClient} = renderHook(() => useApplyVersion());
    const invalidate = vi.spyOn(queryClient, 'invalidateQueries');

    result.current.mutate({gatewayId: 'gw-1', data: {version: '1', dryRun: true}});

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(invalidate).not.toHaveBeenCalled();
  });

  it('reverts a gateway', async () => {
    mockHttpRequest.mockImplementation(
      createHttpRouter({'POST /gateways/gw-1/revert': {gatewayId: 'gw-1', dryRun: false, recorded: true}}),
    );

    const {result, queryClient} = renderHook(() => useRevertGateway());
    const invalidate = vi.spyOn(queryClient, 'invalidateQueries');

    result.current.mutate({gatewayId: 'gw-1', data: {dryRun: false}});

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(mockHttpRequest).toHaveBeenCalledWith(expect.objectContaining({data: {dryRun: false}}));
    expect(invalidate).toHaveBeenCalledWith({queryKey: [GatewayQueryKeys.GATEWAY_DIFF, 'gw-1']});
  });

  it('records nothing after a dry-run revert', async () => {
    mockHttpRequest.mockImplementation(
      createHttpRouter({'POST /gateways/gw-1/revert': {gatewayId: 'gw-1', dryRun: true, recorded: false}}),
    );

    const {result, queryClient} = renderHook(() => useRevertGateway());
    const invalidate = vi.spyOn(queryClient, 'invalidateQueries');

    result.current.mutate({gatewayId: 'gw-1', data: {dryRun: true}});

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(invalidate).not.toHaveBeenCalled();
  });

  it('runs a dry run of an apply to find missing values, and holds it until enabled', async () => {
    const result = {gatewayId: 'gw-1', dryRun: true, recorded: false, missing: {variables: ['API_URL']}};
    mockHttpRequest.mockImplementation(createHttpRouter({'POST /gateways/gw-1/apply': result}));

    const {result: idle} = renderHook(() => useApplyDryRun('gw-1', 'latest', false));
    expect(idle.current.fetchStatus).toBe('idle');

    const {result: check} = renderHook(() => useApplyDryRun('gw-1', '2'));
    await waitFor(() => expect(check.current.data).toEqual(result));
    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({method: 'POST', data: {version: '2', dryRun: true}}),
    );
  });

  it('runs a dry run of a revert to find missing values', async () => {
    const result = {gatewayId: 'gw-1', dryRun: true, recorded: false};
    mockHttpRequest.mockImplementation(createHttpRouter({'POST /gateways/gw-1/revert': result}));

    const {result: check} = renderHook(() => useRevertDryRun('gw-1'));

    await waitFor(() => expect(check.current.data).toEqual(result));
    expect(mockHttpRequest).toHaveBeenCalledWith(expect.objectContaining({data: {dryRun: true}}));
  });

  it('lists the variables and the secrets of a gateway a page at a time', async () => {
    const variables = {totalResults: 1, startIndex: 1, count: 1, variables: [{name: 'A', value: '1'}]};
    const secrets = {totalResults: 1, startIndex: 1, count: 1, secrets: [{name: 'S', exists: true}]};
    mockHttpRequest.mockImplementation(
      createHttpRouter({'GET /gateways/gw-1/variables': variables, 'GET /gateways/gw-1/secrets': secrets}),
    );

    const {result: listedVariables} = renderHook(() => useGetGatewayVariables('gw-1', {limit: 25, offset: 50}));
    const {result: listedSecrets} = renderHook(() => useGetGatewaySecrets('gw-1'));

    await waitFor(() => expect(listedVariables.current.data).toEqual(variables));
    await waitFor(() => expect(listedSecrets.current.data).toEqual(secrets));
    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({url: `${SERVER_URL}/gateways/gw-1/variables?limit=25&offset=50`}),
    );
    expect(mockHttpRequest).toHaveBeenCalledWith(expect.objectContaining({url: `${SERVER_URL}/gateways/gw-1/secrets`}));
  });

  it.each([
    {
      label: 'creates a variable',
      hook: useCreateGatewayVariable,
      vars: {gatewayId: 'gw-1', data: {name: 'A', value: '1'}},
      call: {method: 'POST', url: `${SERVER_URL}/gateways/gw-1/variables`},
      key: GatewayQueryKeys.GATEWAY_VARIABLES,
      toast: 'Variable A added.',
    },
    {
      label: 'updates a variable',
      hook: useUpdateGatewayVariable,
      vars: {gatewayId: 'gw-1', name: 'A', data: {value: '2'}},
      call: {method: 'PUT', url: `${SERVER_URL}/gateways/gw-1/variables/A`},
      key: GatewayQueryKeys.GATEWAY_VARIABLES,
      toast: 'Variable A updated.',
    },
    {
      label: 'deletes a variable',
      hook: useDeleteGatewayVariable,
      vars: {gatewayId: 'gw-1', name: 'A'},
      call: {method: 'DELETE', url: `${SERVER_URL}/gateways/gw-1/variables/A`},
      key: GatewayQueryKeys.GATEWAY_VARIABLES,
      toast: 'Variable A deleted.',
    },
    {
      label: 'creates a secret',
      hook: useCreateGatewaySecret,
      vars: {gatewayId: 'gw-1', data: {name: 'S', value: 'x'}},
      call: {method: 'POST', url: `${SERVER_URL}/gateways/gw-1/secrets`},
      key: GatewayQueryKeys.GATEWAY_SECRETS,
      toast: 'Secret S added.',
    },
    {
      label: 'replaces a secret',
      hook: useUpdateGatewaySecret,
      vars: {gatewayId: 'gw-1', name: 'S', data: {value: 'y'}},
      call: {method: 'PUT', url: `${SERVER_URL}/gateways/gw-1/secrets/S`},
      key: GatewayQueryKeys.GATEWAY_SECRETS,
      toast: 'Secret S value replaced.',
    },
    {
      label: 'deletes a secret',
      hook: useDeleteGatewaySecret,
      vars: {gatewayId: 'gw-1', name: 'S'},
      call: {method: 'DELETE', url: `${SERVER_URL}/gateways/gw-1/secrets/S`},
      key: GatewayQueryKeys.GATEWAY_SECRETS,
      toast: 'Secret S deleted.',
    },
  ])('$label and refreshes the listing and the missing-value checks', async ({hook, vars, call, key, toast}) => {
    mockHttpRequest.mockResolvedValue({data: null});

    const {result, queryClient} = renderHook(() =>
      (hook as () => {mutate: (v: unknown) => void; isSuccess: boolean})(),
    );
    const invalidate = vi.spyOn(queryClient, 'invalidateQueries');

    result.current.mutate(vars);

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(mockHttpRequest).toHaveBeenCalledWith(expect.objectContaining(call));
    expect(invalidate).toHaveBeenCalledWith({queryKey: [key, 'gw-1']});
    expect(invalidate).toHaveBeenCalledWith({queryKey: [GatewayQueryKeys.GATEWAY_DRY_RUN, 'gw-1']});
    expect(mockShowToast).toHaveBeenCalledWith(toast, 'success');
  });
});
