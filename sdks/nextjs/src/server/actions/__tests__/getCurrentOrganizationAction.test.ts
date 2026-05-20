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

// src/server/actions/__tests__/getCurrentOrganizationAction.test.ts
import {describe, it, expect, vi, beforeEach, afterEach, type Mock} from 'vitest';

// --- Import SUT and mocked deps ---
import getClient from '../../getClient';
import getCurrentOrganizationAction from '../getCurrentOrganizationAction';

// --- Mock client factory BEFORE importing SUT ---
vi.mock('../../getClient', () => ({
  default: vi.fn(),
}));

// A light org shape for testing (only fields we assert on)
interface Org {
  id: string;
  name: string;
  orgHandle?: string;
}

describe('getCurrentOrganizationAction', () => {
  type ActionResult = Awaited<ReturnType<typeof getCurrentOrganizationAction>>;

  const mockClient: {getCurrentOrganization: ReturnType<typeof vi.fn>} = {
    getCurrentOrganization: vi.fn(),
  };

  const sessionId = 'sess-123';
  const org: Org = {id: 'org-001', name: 'Alpha', orgHandle: 'alpha'};

  beforeEach(() => {
    vi.resetAllMocks();
    (getClient as unknown as Mock).mockReturnValue(mockClient);
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('returns success with organization when upstream succeeds', async () => {
    mockClient.getCurrentOrganization.mockResolvedValueOnce(org);

    const result: ActionResult = await getCurrentOrganizationAction(sessionId);

    expect(getClient).toHaveBeenCalledTimes(1);
    expect(mockClient.getCurrentOrganization).toHaveBeenCalledWith(sessionId);

    expect(result.success).toBe(true);
    expect(result.error).toBeNull();
    expect(result.data.organization).toEqual(org);
  });

  it('should pass through the provided sessionId even if it is an empty string', async () => {
    mockClient.getCurrentOrganization.mockResolvedValueOnce(org);

    const result: ActionResult = await getCurrentOrganizationAction('');

    expect(mockClient.getCurrentOrganization).toHaveBeenCalledWith('');
    expect(result.success).toBe(true);
    expect(result.data.organization).toEqual(org);
  });

  it('should return failure shape when client.getCurrentOrganization rejects', async () => {
    mockClient.getCurrentOrganization.mockRejectedValueOnce(new Error('upstream down'));

    const result: ActionResult = await getCurrentOrganizationAction(sessionId);

    expect(result.success).toBe(false);
    expect(result.error).toBe('Failed to get the current organization');
    // Matches the function’s failure payload shape
    expect(result.data).toEqual({user: {}});
  });

  it('should return failure shape when ThunderIDNextClient.getInstance throws', async () => {
    (getClient as unknown as Mock).mockImplementationOnce(() => {
      throw new Error('factory failed');
    });

    const result: ActionResult = await getCurrentOrganizationAction(sessionId);

    expect(result.success).toBe(false);
    expect(result.error).toBe('Failed to get the current organization');
    expect(result.data).toEqual({user: {}});
  });

  it('should not mutate the organization object returned by upstream', async () => {
    const upstreamOrg: Org & {extra: {nested: boolean}} = {...org, extra: {nested: true}};
    mockClient.getCurrentOrganization.mockResolvedValueOnce(upstreamOrg);

    const result: ActionResult = await getCurrentOrganizationAction(sessionId);

    // exact deep equality: whatever upstream returns is passed through
    expect(result.data.organization).toEqual(upstreamOrg);
  });
});
