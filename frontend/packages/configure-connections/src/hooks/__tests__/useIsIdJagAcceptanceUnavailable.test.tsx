// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderHook} from '@thunderid/test-utils';
import {afterEach, describe, expect, it, vi} from 'vitest';
import useIsIdJagAcceptanceUnavailable from '../useIsIdJagAcceptanceUnavailable';

const {mockUseAuthorizationServerMetadata} = vi.hoisted(() => ({mockUseAuthorizationServerMetadata: vi.fn()}));

vi.mock('../../api/useAuthorizationServerMetadata', () => ({
  default: mockUseAuthorizationServerMetadata,
}));

const ID_JAG_PROFILE = 'urn:ietf:params:oauth:grant-profile:id-jag';
const JWT_BEARER = 'urn:ietf:params:oauth:grant-type:jwt-bearer';

const mockServerMetadata = (data: unknown, state: {isLoading?: boolean; isError?: boolean} = {}): void => {
  mockUseAuthorizationServerMetadata.mockReturnValue({data, ...state});
};

describe('useIsIdJagAcceptanceUnavailable', () => {
  afterEach(() => {
    vi.clearAllMocks();
  });

  it('should be true when the metadata has loaded without the grant profiles field', () => {
    mockServerMetadata({grant_types_supported: ['authorization_code']});

    const {result} = renderHook(() => useIsIdJagAcceptanceUnavailable());

    expect(result.current).toBe(true);
  });

  it('should be true when the metadata lists other grant profiles only', () => {
    mockServerMetadata({
      grant_types_supported: ['authorization_code', JWT_BEARER],
      authorization_grant_profiles_supported: ['urn:example:profile'],
    });

    const {result} = renderHook(() => useIsIdJagAcceptanceUnavailable());

    expect(result.current).toBe(true);
  });

  it('should be false when the metadata lists the ID-JAG grant profile', () => {
    mockServerMetadata({
      grant_types_supported: ['authorization_code', JWT_BEARER],
      authorization_grant_profiles_supported: [ID_JAG_PROFILE],
    });

    const {result} = renderHook(() => useIsIdJagAcceptanceUnavailable());

    expect(result.current).toBe(false);
  });

  it('should be false while the metadata is loading', () => {
    mockServerMetadata(undefined, {isLoading: true});

    const {result} = renderHook(() => useIsIdJagAcceptanceUnavailable());

    expect(result.current).toBe(false);
  });

  it('should be false when the metadata failed to load', () => {
    mockServerMetadata(undefined, {isError: true});

    const {result} = renderHook(() => useIsIdJagAcceptanceUnavailable());

    expect(result.current).toBe(false);
  });

  it('should be false when the metadata carries no grant types list', () => {
    mockServerMetadata({issuer: 'https://localhost:8090'});

    const {result} = renderHook(() => useIsIdJagAcceptanceUnavailable());

    expect(result.current).toBe(false);
  });
});
