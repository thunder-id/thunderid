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

import {describe, it, expect, vi, beforeEach, afterEach, type Mock} from 'vitest';

// Import SUT and mocked deps
import getClient from '../../getClient';
import getOrganizationAction from '../getOrganizationAction';

// Mock client factory BEFORE importing the SUT
vi.mock('../../getClient', () => ({
  default: vi.fn(),
}));

// Minimal shape for testing; add fields only if you assert on them
interface OrganizationDetails {
  id: string;
  name: string;
  orgHandle?: string;
}

type ActionResult = Awaited<ReturnType<typeof getOrganizationAction>>;

describe('getOrganizationAction', () => {
  const mockClient: {getOrganization: ReturnType<typeof vi.fn>} = {
    getOrganization: vi.fn(),
  };

  const orgId = 'org-001';
  const sessionId = 'sess-123';
  const org: OrganizationDetails = {id: orgId, name: 'Alpha', orgHandle: 'alpha'};

  beforeEach(() => {
    vi.resetAllMocks();
    (getClient as unknown as Mock).mockReturnValue(mockClient);
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('should return success with organization when upstream succeeds', async () => {
    mockClient.getOrganization.mockResolvedValueOnce(org);

    const result: ActionResult = await getOrganizationAction(orgId, sessionId);

    expect(getClient).toHaveBeenCalledTimes(1);
    expect(mockClient.getOrganization).toHaveBeenCalledWith(orgId, sessionId);

    expect(result).toEqual({
      data: {organization: org},
      error: null,
      success: true,
    });
  });

  it('should pass through empty-string organizationId and sessionId (documents current behavior)', async () => {
    mockClient.getOrganization.mockResolvedValueOnce(org);

    const result: ActionResult = await getOrganizationAction('', '');

    expect(mockClient.getOrganization).toHaveBeenCalledWith('', '');
    expect(result.success).toBe(true);
    expect(result.data.organization).toEqual(org);
  });

  it('should return failure shape when client.getOrganization rejects', async () => {
    mockClient.getOrganization.mockRejectedValueOnce(new Error('upstream down'));

    const result: ActionResult = await getOrganizationAction(orgId, sessionId);

    expect(result).toEqual({
      data: {user: {}},
      error: 'Failed to get organization',
      success: false,
    });
  });

  it('should return failure shape when ThunderIDNextClient.getInstance throws', async () => {
    (getClient as unknown as Mock).mockImplementationOnce(() => {
      throw new Error('factory failed');
    });

    const result: ActionResult = await getOrganizationAction(orgId, sessionId);

    expect(result).toEqual({
      data: {user: {}},
      error: 'Failed to get organization',
      success: false,
    });
  });

  it('should return failure shape when client rejects with a non-Error value', async () => {
    mockClient.getOrganization.mockRejectedValueOnce('bad');
    const result: ActionResult = await getOrganizationAction(orgId, sessionId);
    expect(result).toEqual({
      data: {user: {}},
      error: 'Failed to get organization',
      success: false,
    });
  });

  it('should not mutate the organization object returned by upstream', async () => {
    const upstreamOrg: OrganizationDetails & {extra: {nested: boolean}} = {...org, extra: {nested: true}};
    mockClient.getOrganization.mockResolvedValueOnce(upstreamOrg);

    const result: ActionResult = await getOrganizationAction(orgId, sessionId);

    expect(result.data.organization).toEqual(upstreamOrg);
  });
});
