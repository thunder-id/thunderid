// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderHook} from '@thunderid/test-utils';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import useFlowVariableConnections from '../useFlowVariableConnections';

const mockUseConnections = vi.hoisted(() => vi.fn());

vi.mock('@thunderid/configure-connections', () => ({
  ConnectionInstanceCategories: {IDENTITY_PROVIDER: 'identity-provider'},
  ConnectionTypes: {GOOGLE: 'google', GITHUB: 'github', OIDC: 'oidc', OAUTH: 'oauth'},
  useConnections: mockUseConnections,
}));

const google = {id: 'idp-1', name: 'Google login', type: 'google', categories: ['identity-provider']};

describe('useFlowVariableConnections', () => {
  beforeEach(() => {
    mockUseConnections.mockReset().mockReturnValue({data: {connections: [google]}, error: null, refetch: vi.fn()});
  });

  it('lists the identity providers, as the connections the variable components read', () => {
    const {result} = renderHook(() => useFlowVariableConnections());

    expect(result.current.connections).toEqual([google]);
    expect(result.current.error).toBeNull();
    expect(mockUseConnections).toHaveBeenCalledWith({category: 'identity-provider', filter: undefined, limit: 100});
  });

  it('leaves out an authentication endpoint connection, which a variable cannot name', () => {
    const endpoint = {
      id: 'idp-9',
      name: 'Org login',
      type: 'authentication-endpoint',
      categories: ['identity-provider'],
    };
    mockUseConnections.mockReturnValue({data: {connections: [google, endpoint]}, error: null, refetch: vi.fn()});

    const {result} = renderHook(() => useFlowVariableConnections());

    expect(result.current.connections).toEqual([google]);
  });

  it('limits the list to what the owning organization unit owns or was shared', () => {
    renderHook(() => useFlowVariableConnections('ou-1'));

    expect(mockUseConnections).toHaveBeenCalledWith({
      category: 'identity-provider',
      filter: 'ouId eq "ou-1"',
      limit: 100,
    });
  });

  it('holds no connections while they load or when they failed, and offers a retry', () => {
    const refetch = vi.fn();
    const failure = new Error('down');

    mockUseConnections.mockReturnValue({data: undefined, error: failure, refetch});
    const {result} = renderHook(() => useFlowVariableConnections());

    expect(result.current.connections).toEqual([]);
    expect(result.current.error).toBe(failure);

    result.current.refetch();

    expect(refetch).toHaveBeenCalledTimes(1);
  });
});
