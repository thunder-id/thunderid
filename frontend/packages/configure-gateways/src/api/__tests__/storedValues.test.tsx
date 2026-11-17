// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {QueryClient} from '@tanstack/react-query';
import {renderHook, waitFor} from '@thunderid/test-utils';
import {act} from 'react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import {fakeHttp, queryOf, SERVER, storeListing, type Call} from '../../__tests__/fakeHttp';
import {
  MAX_NAMES_QUERY_BYTES,
  nameBatches,
  useDeleteStoredValue,
  useSetStoredValue,
  useStoredValueCount,
  useStoredValuesNamed,
  useStoredValuesPage,
} from '../useStoredValues';

let http = fakeHttp({});
vi.mock('@thunderid/react', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/react')>()),
  useThunderID: () => ({http: {request: (call: Call) => http.request(call)}}),
}));
vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {...actual, useConfig: () => ({...actual.useConfig(), getServerUrl: () => SERVER})};
});

const name = (index: number): string => `VALUE_${String(index).padStart(3, '0')}`;
const variables = Array.from({length: 250}, (_, index) => ({name: name(index), value: `v${index}`}));

/** The reads of the gateway's variables, each by its query. */
const reads = (): Record<string, string>[] =>
  http.calls
    .filter((call: Call) => call.method === 'GET' && call.url.split('?')[0].endsWith('/gateways/gw-dev/variables'))
    .map(queryOf);

beforeEach(() => {
  http = fakeHttp({
    'GET /gateways/gw-dev/variables': storeListing('variables', variables),
    'GET /gateways/gw-dev/secrets': storeListing('secrets', [{name: 'S', exists: true}]),
    'PUT /gateways/gw-dev/variables/VALUE_000': {name: 'VALUE_000', value: 'new'},
    'DELETE /gateways/gw-dev/variables/VALUE_000': {},
  });
});

describe('useStoredValuesPage', () => {
  it('asks for one page by limit and offset, and answers with it and the total', async () => {
    const {result} = renderHook(() => useStoredValuesPage('gw-dev', 'variable', {limit: 25, offset: 50}));

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(reads()).toEqual([{limit: '25', offset: '50'}]);
    expect(result.current.data?.totalResults).toBe(250);
    expect(result.current.data?.values.map((value) => value.name)).toEqual(
      Array.from({length: 25}, (_, index) => name(50 + index)),
    );
  });

  it("sends a search as the store's name prefix filter", async () => {
    const {result} = renderHook(() =>
      useStoredValuesPage('gw-dev', 'variable', {limit: 10, offset: 0, prefix: 'VALUE_24'}),
    );

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(reads()).toEqual([{limit: '10', offset: '0', filter: 'name sw "VALUE_24"'}]);
    expect(result.current.data?.totalResults).toBe(10);
  });

  it('keeps the page before shown while the next is read, but not across gateways', async () => {
    http = fakeHttp({
      'GET /gateways/gw-dev/variables': storeListing('variables', variables),
      'GET /gateways/gw-prod/variables': () => new Promise(() => undefined),
    });
    const {result, rerender} = renderHook(
      ({gatewayId, offset}: {gatewayId: string; offset: number}) =>
        useStoredValuesPage(gatewayId, 'variable', {limit: 10, offset}),
      {initialProps: {gatewayId: 'gw-dev', offset: 0}},
    );
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    rerender({gatewayId: 'gw-dev', offset: 10});
    expect(result.current.isPlaceholderData).toBe(true);
    expect(result.current.data?.values[0].name).toBe(name(0));
    await waitFor(() => expect(result.current.data?.values[0].name).toBe(name(10)));

    rerender({gatewayId: 'gw-prod', offset: 10});
    expect(result.current.data).toBeUndefined();
  });

  it('reads the secrets from their own collection', async () => {
    const {result} = renderHook(() => useStoredValuesPage('gw-dev', 'secret', {limit: 10, offset: 0}));

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toEqual({values: [{name: 'S', exists: true}], totalResults: 1});
  });
});

describe('useStoredValueCount', () => {
  it('counts what a gateway holds with a page of one', async () => {
    const {result} = renderHook(() => useStoredValueCount('gw-dev', 'variable'));

    await waitFor(() => expect(result.current.data).toBe(250));
    expect(reads()).toEqual([{limit: '1', offset: '0'}]);
  });
});

describe('useStoredValuesNamed', () => {
  it('looks names up a hundred at a time, and nothing else', async () => {
    const wanted = [...variables.map((value) => value.name), 'NOT_HELD'];
    const {result} = renderHook(() => useStoredValuesNamed('gw-dev', 'variable', wanted));

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    const lookups = reads();
    expect(lookups).toHaveLength(3);
    lookups.forEach((query) => {
      expect(query.limit).toBe('100');
      expect(query.offset).toBe('0');
      expect(query.names.split(',').length).toBeLessThanOrEqual(100);
    });
    expect(lookups.flatMap((query) => query.names.split(',')).sort()).toEqual([...wanted].sort());
    expect(result.current.data).toHaveLength(250);
  });

  it('looks long names up in smaller batches, one after another', async () => {
    const long = Array.from({length: 60}, (_, index) => `L${String(index).padStart(3, '0')}${'X'.repeat(251)}`);
    http = fakeHttp({'GET /gateways/gw-dev/variables': storeListing('variables', [])});
    const {result} = renderHook(() => useStoredValuesNamed('gw-dev', 'variable', long));

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    const lookups = reads();
    expect(lookups.map((query) => query.names.split(',').length)).toEqual([23, 23, 14]);
    lookups.forEach((query) => {
      expect(encodeURIComponent(query.names).length).toBeLessThanOrEqual(MAX_NAMES_QUERY_BYTES);
    });
  });

  it('asks for each name once, and not for a name no store could hold', async () => {
    const {result} = renderHook(() =>
      useStoredValuesNamed('gw-dev', 'variable', ['VALUE_001', 'VALUE_001', 'not-a-name', '1ST']),
    );

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(reads()).toEqual([{limit: '100', offset: '0', names: 'VALUE_001'}]);
    expect(result.current.data).toEqual([{name: 'VALUE_001', value: 'v1'}]);
  });

  it('asks nothing when there are no names to look up', async () => {
    const {result} = renderHook(() => useStoredValuesNamed('gw-dev', 'variable', []));

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toEqual([]);
    expect(http.calls).toHaveLength(0);
  });
});

describe('stored value mutations', () => {
  it('refresh the pages, counts and lookups of that kind when a value is set or removed', async () => {
    const queryClient = new QueryClient({defaultOptions: {queries: {retry: false}}});
    const invalidate = vi.spyOn(queryClient, 'invalidateQueries');
    const {result} = renderHook(
      () => ({
        page: useStoredValuesPage('gw-dev', 'variable', {limit: 10, offset: 0}),
        named: useStoredValuesNamed('gw-dev', 'variable', ['VALUE_000']),
        set: useSetStoredValue('gw-dev', 'variable'),
        remove: useDeleteStoredValue('gw-dev', 'variable'),
      }),
      {queryClient},
    );
    await waitFor(() => expect(result.current.page.isSuccess && result.current.named.isSuccess).toBe(true));
    expect(reads()).toHaveLength(2);

    await act(async () => {
      await result.current.set.mutateAsync({name: 'VALUE_000', value: 'new'});
    });
    expect(invalidate).toHaveBeenCalledWith({queryKey: ['deploy-values', 'gw-dev', 'variables']});
    await waitFor(() => expect(reads()).toHaveLength(4));

    await act(async () => {
      await result.current.remove.mutateAsync('VALUE_000');
    });
    expect(invalidate).toHaveBeenCalledTimes(2);
    await waitFor(() => expect(reads()).toHaveLength(6));
  });
});

describe('nameBatches', () => {
  it('holds at most a hundred names a batch, each name once and in order', () => {
    const names = Array.from({length: 250}, (_, index) => name(index));

    const batches = nameBatches(names);

    expect(batches.map((batch) => batch.length)).toEqual([100, 100, 50]);
    expect(batches.flat()).toEqual(names);
    expect(nameBatches([])).toEqual([]);
  });
});
