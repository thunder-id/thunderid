/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

// src/server/actions/__tests__/getMyOrganizations.test.ts
import {ThunderIDAPIError, Organization} from '@thunderid/node';
import {describe, it, expect, vi, beforeEach, afterEach, type Mock} from 'vitest';

// --- Import SUT and mocked deps ---
import getClient from '../../getClient';
import getMyOrganizations from '../getMyOrganizations';
import getSessionId from '../getSessionId';

// --- Mocks (declare BEFORE importing the SUT) ---
vi.mock('../../getClient', () => ({
  default: vi.fn(),
}));

// Mock the dynamically-imported module that SUT calls as: import('./getSessionId')
vi.mock('../getSessionId', () => ({
  default: vi.fn(),
}));

describe('getMyOrganizations (Next.js server action)', () => {
  const mockClient: {getAccessToken: ReturnType<typeof vi.fn>; getMyOrganizations: ReturnType<typeof vi.fn>} = {
    getAccessToken: vi.fn(),
    getMyOrganizations: vi.fn(),
  };

  const options: {filter: string; limit: number} = {filter: 'type eq "TENANT"', limit: 25};
  const orgs: Organization[] = [
    {id: 'org-1', name: 'Alpha', orgHandle: 'alpha'},
    {id: 'org-2', name: 'Beta', orgHandle: 'beta'},
  ];

  beforeEach(() => {
    vi.resetAllMocks();
    (getClient as unknown as Mock).mockReturnValue(mockClient);
    (getSessionId as unknown as Mock).mockResolvedValue('sess-abc');
    mockClient.getAccessToken.mockResolvedValue('atk-123');
    mockClient.getMyOrganizations.mockResolvedValue(orgs);
    vi.spyOn(console, 'error').mockImplementation(() => {});
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('should return organizations when sessionId is provided (no getSessionId fallback)', async () => {
    const result: Organization[] = await getMyOrganizations(options, 'sess-123');

    expect(getClient).toHaveBeenCalledTimes(1);
    expect(getSessionId).not.toHaveBeenCalled();
    expect(mockClient.getAccessToken).toHaveBeenCalledWith('sess-123');
    expect(mockClient.getMyOrganizations).toHaveBeenCalledWith(options, 'sess-123');
    expect(result).toEqual(orgs);
  });

  it('should fall back to getSessionId when sessionId is undefined', async () => {
    const result: Organization[] = await getMyOrganizations(options, undefined);

    expect(getSessionId).toHaveBeenCalledTimes(1);
    expect(mockClient.getAccessToken).toHaveBeenCalledWith('sess-abc');
    expect(mockClient.getMyOrganizations).toHaveBeenCalledWith(options, 'sess-abc');
    expect(result).toEqual(orgs);
  });

  it('should fall back to getSessionId when sessionId is null', async () => {
    const result: Organization[] = await getMyOrganizations(options, null as unknown as string);

    expect(getSessionId).toHaveBeenCalledTimes(1);
    expect(mockClient.getAccessToken).toHaveBeenCalledWith('sess-abc');
    expect(mockClient.getMyOrganizations).toHaveBeenCalledWith(options, 'sess-abc');
    expect(result).toEqual(orgs);
  });

  it('should treat empty string sessionId as falsy and calls getSessionId', async () => {
    const result: Organization[] = await getMyOrganizations(options, '');

    expect(getSessionId).toHaveBeenCalledTimes(1);
    expect(mockClient.getAccessToken).toHaveBeenCalledWith('sess-abc');
    expect(mockClient.getMyOrganizations).toHaveBeenCalledWith(options, 'sess-abc');
    expect(result).toEqual(orgs);
  });

  it('should pass through undefined options', async () => {
    const result: Organization[] = await getMyOrganizations(undefined, 'sess-123');

    expect(mockClient.getMyOrganizations).toHaveBeenCalledWith(undefined, 'sess-123');
    expect(result).toEqual(orgs);
  });

  it('should throw ThunderIDAPIError(401) when no session can be resolved', async () => {
    (getSessionId as unknown as Mock).mockResolvedValueOnce('');

    await expect(getMyOrganizations(options, undefined)).rejects.toMatchObject({
      constructor: ThunderIDAPIError,
      message: expect.stringContaining('Failed to get the organizations for the user: No session ID available'),
      statusCode: 401,
    });

    // Should fail before calling client methods
    expect(mockClient.getAccessToken).not.toHaveBeenCalled();
    expect(mockClient.getMyOrganizations).not.toHaveBeenCalled();
  });

  it('should throw ThunderIDAPIError(401) when access token resolves to undefined (not signed in)', async () => {
    mockClient.getAccessToken.mockResolvedValueOnce(undefined);

    await expect(getMyOrganizations(options, 'sess-123')).rejects.toMatchObject({
      constructor: ThunderIDAPIError,
      message: expect.stringContaining(
        'Failed to get the organizations for the user: User is not signed in - access token retrieval failed',
      ),
      statusCode: 401,
    });

    expect(mockClient.getAccessToken).toHaveBeenCalledWith('sess-123');
    // eslint-disable-next-line no-console
    expect(console.error).toHaveBeenCalled(); // inner catch logs
    expect(mockClient.getMyOrganizations).not.toHaveBeenCalled();
  });

  it('should treat empty-string access token as not signed in (401)', async () => {
    mockClient.getAccessToken.mockResolvedValueOnce('');

    await expect(getMyOrganizations(options, 'sess-123')).rejects.toMatchObject({
      constructor: ThunderIDAPIError,
      message: expect.stringContaining(
        'Failed to get the organizations for the user: User is not signed in - access token retrieval failed',
      ),
      statusCode: 401,
    });

    // eslint-disable-next-line no-console
    expect(console.error).toHaveBeenCalled();
    expect(mockClient.getMyOrganizations).not.toHaveBeenCalled();
  });

  it('should throw ThunderIDAPIError(401) when getAccessToken throws (e.g., upstream failure)', async () => {
    mockClient.getAccessToken.mockRejectedValueOnce(new Error('token endpoint down'));

    await expect(getMyOrganizations(options, 'sess-123')).rejects.toMatchObject({
      constructor: ThunderIDAPIError,
      message: expect.stringContaining(
        'Failed to get the organizations for the user: User is not signed in - access token retrieval failed',
      ),
      statusCode: 401,
    });

    // eslint-disable-next-line no-console
    expect(console.error).toHaveBeenCalled();
    expect(mockClient.getMyOrganizations).not.toHaveBeenCalled();
  });

  it('should wrap an ThunderIDAPIError from client.getMyOrganizations, preserving statusCode', async () => {
    const upstream: ThunderIDAPIError = new ThunderIDAPIError('Upstream failed', 'ORG_LIST_503', 'server', 503);
    mockClient.getMyOrganizations.mockRejectedValueOnce(upstream);

    await expect(getMyOrganizations(options, 'sess-123')).rejects.toMatchObject({
      constructor: ThunderIDAPIError,
      message: expect.stringContaining('Failed to get the organizations for the user: Upstream failed'),
      statusCode: 503,
    });
  });

  it('should wrap a generic Error from client.getMyOrganizations with undefined statusCode', async () => {
    mockClient.getMyOrganizations.mockRejectedValueOnce(new Error('network down'));

    await expect(getMyOrganizations(options, 'sess-123')).rejects.toMatchObject({
      constructor: ThunderIDAPIError,
      message: expect.stringContaining('Failed to get the organizations for the user: network down'),
      statusCode: undefined,
    });
  });

  it('should wrap an error thrown by ThunderIDNextClient.getInstance()', async () => {
    (getClient as unknown as Mock).mockImplementationOnce(() => {
      throw new Error('factory failed');
    });

    await expect(getMyOrganizations(options, 'sess-123')).rejects.toMatchObject({
      constructor: ThunderIDAPIError,
      message: expect.stringContaining('Failed to get the organizations for the user: factory failed'),
      statusCode: undefined,
    });
  });

  it('should handle minimal call: no options, undefined sessionId -> resolves via getSessionId and succeeds', async () => {
    const result: Organization[] = await getMyOrganizations();

    expect(getSessionId).toHaveBeenCalledTimes(1);
    expect(mockClient.getAccessToken).toHaveBeenCalledWith('sess-abc');
    expect(mockClient.getMyOrganizations).toHaveBeenCalledWith(undefined, 'sess-abc');
    expect(result).toEqual(orgs);
  });
});
