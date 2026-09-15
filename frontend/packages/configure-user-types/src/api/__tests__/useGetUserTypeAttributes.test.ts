// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {waitFor, renderHook} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, afterEach, vi} from 'vitest';
import type {ApiUserType, UserTypeListResponse} from '../../types/user-types';
import useGetUserTypeAttributes from '../useGetUserTypeAttributes';

const mockHttpRequest = vi.fn();
const mockLoggerWarn = vi.fn();

vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({http: {request: mockHttpRequest}}),
}));

vi.mock('@thunderid/logger/react', () => ({
  useLogger: () => ({
    error: vi.fn(),
    info: vi.fn(),
    warn: mockLoggerWarn,
    debug: vi.fn(),
  }),
}));

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: vi.fn(),
  };
});

const {useConfig} = await import('@thunderid/contexts');

/**
 * Build a list response for the given user types.
 *
 * @param types - The user types to include.
 * @param totalResults - Total results reported by the server, defaults to the number of types.
 * @returns The list response.
 */
function buildListResponse(types: {id: string; name: string}[], totalResults?: number): UserTypeListResponse {
  return {
    totalResults: totalResults ?? types.length,
    startIndex: 1,
    count: types.length,
    types: types.map((type) => ({...type, ouId: 'root-ou', allowSelfRegistration: false})),
  };
}

/**
 * Route the mocked http client: the list endpoint returns the given list, and each user type
 * endpoint returns its schema.
 *
 * @param list - The list response.
 * @param schemas - Schema by user type id. A missing entry rejects, simulating a failed fetch.
 */
function mockRequests(list: UserTypeListResponse, schemas: Record<string, ApiUserType>): void {
  mockHttpRequest.mockImplementation(({url}: {url: string}) => {
    const match = /\/user-types\/([^?]+)/.exec(url);

    if (!match) {
      return Promise.resolve({data: list});
    }

    const schema = schemas[match[1]];
    return schema ? Promise.resolve({data: schema}) : Promise.reject(new Error('failed'));
  });
}

describe('useGetUserTypeAttributes', () => {
  beforeEach(() => {
    mockHttpRequest.mockReset();
    mockLoggerWarn.mockReset();

    vi.mocked(useConfig).mockReturnValue({
      getServerUrl: vi.fn().mockReturnValue('https://api.test.com'),
    } as unknown as ReturnType<typeof useConfig>);
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('should request user types with the maximum page size', async () => {
    mockRequests(buildListResponse([{id: '1', name: 'Person'}]), {
      '1': {id: '1', name: 'Person', ouId: 'root-ou', allowSelfRegistration: false, schema: {email: {type: 'string'}}},
    });

    renderHook(() => useGetUserTypeAttributes());

    await waitFor(() => {
      expect(mockHttpRequest).toHaveBeenCalled();
    });

    const firstCall = mockHttpRequest.mock.calls[0][0] as {url: string};
    expect(firstCall.url).toContain('limit=100');
  });

  it('should aggregate and de-duplicate attributes across user types', async () => {
    mockRequests(
      buildListResponse([
        {id: '1', name: 'Person'},
        {id: '2', name: 'Customers'},
      ]),
      {
        '1': {
          id: '1',
          name: 'Person',
          ouId: 'root-ou',
          allowSelfRegistration: false,
          schema: {email: {type: 'string'}, username: {type: 'string'}},
        },
        '2': {
          id: '2',
          name: 'Customers',
          ouId: 'root-ou',
          allowSelfRegistration: false,
          schema: {email: {type: 'string'}, gender: {type: 'string'}},
        },
      },
    );

    const {result} = renderHook(() => useGetUserTypeAttributes());

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.attributes.map((attribute) => attribute.attribute)).toEqual(['email', 'gender', 'username']);
    expect(result.current.attributes.find((attribute) => attribute.attribute === 'email')?.userTypes).toEqual([
      'Person',
      'Customers',
    ]);
  });

  it('should keep credential and standard variants of the same attribute separate', async () => {
    mockRequests(
      buildListResponse([
        {id: '1', name: 'Person'},
        {id: '2', name: 'Customers'},
      ]),
      {
        '1': {
          id: '1',
          name: 'Person',
          ouId: 'root-ou',
          allowSelfRegistration: false,
          schema: {secret: {type: 'string', credential: true}},
        },
        '2': {
          id: '2',
          name: 'Customers',
          ouId: 'root-ou',
          allowSelfRegistration: false,
          schema: {secret: {type: 'string'}},
        },
      },
    );

    const {result} = renderHook(() => useGetUserTypeAttributes());

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    const secrets = result.current.attributes.filter((attribute) => attribute.attribute === 'secret');
    expect(secrets).toHaveLength(2);
    expect(secrets.find((attribute) => attribute.credential)?.userTypes).toEqual(['Person']);
    expect(secrets.find((attribute) => !attribute.credential)?.userTypes).toEqual(['Customers']);
  });

  it('should list a user type once when its schema flattens to the same attribute twice', async () => {
    mockRequests(buildListResponse([{id: '1', name: 'Person'}]), {
      '1': {
        id: '1',
        name: 'Person',
        ouId: 'root-ou',
        allowSelfRegistration: false,
        // A literal dotted key collides with the path flattened out of the nested object.
        schema: {
          address: {type: 'object', properties: {city: {type: 'string'}}},
          'address.city': {type: 'string'},
        },
      },
    });

    const {result} = renderHook(() => useGetUserTypeAttributes());

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.attributes).toHaveLength(1);
    expect(result.current.attributes[0].attribute).toBe('address.city');
    expect(result.current.attributes[0].userTypes).toEqual(['Person']);
  });

  it('should report loading until every schema resolves', () => {
    mockHttpRequest.mockImplementation(({url}: {url: string}) =>
      /\/user-types\/[^?]+/.test(url)
        ? new Promise(() => null)
        : Promise.resolve({data: buildListResponse([{id: '1', name: 'Person'}])}),
    );

    const {result} = renderHook(() => useGetUserTypeAttributes());

    expect(result.current.isLoading).toBe(true);
    expect(result.current.attributes).toEqual([]);
  });

  it('should keep attributes from the user types that resolved when one schema fails', async () => {
    mockRequests(
      buildListResponse([
        {id: '1', name: 'Person'},
        {id: '2', name: 'Broken'},
      ]),
      {
        '1': {
          id: '1',
          name: 'Person',
          ouId: 'root-ou',
          allowSelfRegistration: false,
          schema: {email: {type: 'string'}},
        },
      },
    );

    const {result} = renderHook(() => useGetUserTypeAttributes());

    await waitFor(() => {
      expect(result.current.attributes).toHaveLength(1);
    });

    expect(result.current.attributes[0].attribute).toBe('email');
    expect(result.current.attributes[0].userTypes).toEqual(['Person']);
  });

  it('should warn when the server reports more user types than are fetched', async () => {
    mockRequests(buildListResponse([{id: '1', name: 'Person'}], 150), {
      '1': {id: '1', name: 'Person', ouId: 'root-ou', allowSelfRegistration: false, schema: {email: {type: 'string'}}},
    });

    renderHook(() => useGetUserTypeAttributes());

    await waitFor(() => {
      expect(mockLoggerWarn).toHaveBeenCalledWith(expect.any(String), expect.objectContaining({totalResults: 150}));
    });
  });

  it('should not warn when every user type is fetched', async () => {
    mockRequests(buildListResponse([{id: '1', name: 'Person'}]), {
      '1': {id: '1', name: 'Person', ouId: 'root-ou', allowSelfRegistration: false, schema: {email: {type: 'string'}}},
    });

    const {result} = renderHook(() => useGetUserTypeAttributes());

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(mockLoggerWarn).not.toHaveBeenCalled();
  });
});
