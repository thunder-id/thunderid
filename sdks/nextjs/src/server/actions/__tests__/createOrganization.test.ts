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

import {ThunderIDAPIError, Organization, CreateOrganizationPayload} from '@thunderid/node';
import {describe, it, expect, vi, beforeEach, afterEach, Mock} from 'vitest';

// Adjust these paths if your project structure is different
import getClient from '../../getClient';
import createOrganization from '../createOrganization';

// Use the same class so we can assert instanceof and status code propagation

// Pull the mocked modules so we can access their spies
import getSessionId from '../getSessionId';

// ---- Mocks ----
vi.mock('../../getClient', () => ({
  default: vi.fn(),
}));

vi.mock('../getSessionId', () => ({
  default: vi.fn(),
}));

describe('createOrganization (Next.js server action)', () => {
  const mockClient: {createOrganization: ReturnType<typeof vi.fn>} = {
    createOrganization: vi.fn(),
  };

  const basePayload: CreateOrganizationPayload = {
    description: 'Screen sharing organization',
    name: 'Team Viewer',
    orgHandle: 'team-viewer',
    parentId: 'parent-123',
    type: 'TENANT',
  };

  const mockOrg: Organization = {
    id: 'org-001',
    name: 'Team Viewer',
    orgHandle: 'team-viewer',
  };

  beforeEach(() => {
    vi.resetAllMocks();

    // Default: getInstance returns our mock client
    (getClient as unknown as Mock).mockReturnValue(mockClient);
    // Default: getSessionId resolves to a session id
    (getSessionId as unknown as Mock).mockResolvedValue('sess-abc');
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('should create an organization successfully when a sessionId is provided', async () => {
    mockClient.createOrganization.mockResolvedValueOnce(mockOrg);

    const result: Organization = await createOrganization(basePayload, 'sess-123');

    expect(getClient).toHaveBeenCalledTimes(1);
    expect(getSessionId).not.toHaveBeenCalled();
    expect(mockClient.createOrganization).toHaveBeenCalledWith(basePayload, 'sess-123');
    expect(result).toEqual(mockOrg);
  });

  it('should fall back to getSessionId when sessionId is undefined', async () => {
    mockClient.createOrganization.mockResolvedValueOnce(mockOrg);

    const result: Organization = await createOrganization(basePayload, undefined as unknown as string);

    expect(getSessionId).toHaveBeenCalledTimes(1);
    expect(mockClient.createOrganization).toHaveBeenCalledWith(basePayload, 'sess-abc');
    expect(result).toEqual(mockOrg);
  });

  it('should fall back to getSessionId when sessionId is null', async () => {
    mockClient.createOrganization.mockResolvedValueOnce(mockOrg);

    const result: Organization = await createOrganization(basePayload, null as unknown as string);

    expect(getSessionId).toHaveBeenCalledTimes(1);
    expect(mockClient.createOrganization).toHaveBeenCalledWith(basePayload, 'sess-abc');
    expect(result).toEqual(mockOrg);
  });

  it('should not call getSessionId when an empty string is passed (empty string is not nullish)', async () => {
    mockClient.createOrganization.mockResolvedValueOnce(mockOrg);

    const result: Organization = await createOrganization(basePayload, '');

    expect(getSessionId).not.toHaveBeenCalled();
    expect(mockClient.createOrganization).toHaveBeenCalledWith(basePayload, '');
    expect(result).toEqual(mockOrg);
  });

  it('should wrap an ThunderIDAPIError thrown by client.createOrganization, preserving statusCode', async () => {
    const original: ThunderIDAPIError = new ThunderIDAPIError(
      'Upstream validation failed',
      'ORG_CREATE_400',
      'server',
      400,
    );
    mockClient.createOrganization.mockRejectedValueOnce(original);

    await expect(createOrganization(basePayload, 'sess-1')).rejects.toMatchObject({
      constructor: ThunderIDAPIError,
      message: expect.stringContaining('Failed to create the organization: Upstream validation failed'),
      statusCode: 400,
    });
  });
});
