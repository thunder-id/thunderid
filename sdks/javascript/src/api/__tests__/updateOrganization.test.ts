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

import {describe, it, expect, vi, beforeEach} from 'vitest';
import ThunderIDAPIError from '../../errors/ThunderIDAPIError';
import type {OrganizationDetails} from '../getOrganization';
import updateOrganization, {createPatchOperations} from '../updateOrganization';

interface PatchOp {
  operation: 'REPLACE' | 'ADD' | 'REMOVE';
  path: string;
  value?: unknown;
}

describe('updateOrganization', (): void => {
  const baseUrl = 'https://api.asgardeo.io/t/demo';
  const organizationId = '0d5e071b-d3d3-475d-b3c6-1a20ee2fa9b1';

  beforeEach((): void => {
    vi.resetAllMocks();
  });

  it('should update organization successfully with default fetch', async (): Promise<void> => {
    const mockOrg: OrganizationDetails = {
      description: 'Updated',
      id: organizationId,
      name: 'Updated Name',
      orgHandle: 'demo',
    };

    global.fetch = vi.fn().mockResolvedValue({
      json: () => Promise.resolve(mockOrg),
      ok: true,
    });

    const operations: PatchOp[] = [
      {operation: 'REPLACE' as const, path: '/name', value: 'Updated Name'},
      {operation: 'REPLACE' as const, path: '/description', value: 'Updated'},
    ];

    const result: OrganizationDetails = await updateOrganization({
      baseUrl,
      operations,
      organizationId,
    });

    expect(fetch).toHaveBeenCalledWith(`${baseUrl}/api/server/v1/organizations/${organizationId}`, {
      body: JSON.stringify(operations),
      headers: {
        Accept: 'application/json',
        'Content-Type': 'application/json',
      },
      method: 'PATCH',
    });
    expect(result).toEqual(mockOrg);
  });

  it('should use custom fetcher when provided', async (): Promise<void> => {
    const mockOrg: OrganizationDetails = {
      id: organizationId,
      name: 'Custom',
      orgHandle: 'custom',
    };

    const customFetcher: ReturnType<typeof vi.fn> = vi.fn().mockResolvedValue({
      json: () => Promise.resolve(mockOrg),
      ok: true,
    });

    const operations: PatchOp[] = [{operation: 'REPLACE' as const, path: '/name', value: 'Custom'}];

    const result: OrganizationDetails = await updateOrganization({
      baseUrl,
      fetcher: customFetcher,
      operations,
      organizationId,
    });

    expect(result).toEqual(mockOrg);
    expect(customFetcher).toHaveBeenCalledWith(
      `${baseUrl}/api/server/v1/organizations/${organizationId}`,
      expect.objectContaining({
        body: JSON.stringify(operations),
        headers: expect.objectContaining({
          Accept: 'application/json',
          'Content-Type': 'application/json',
        }),
        method: 'PATCH',
      }),
    );
  });

  it('should handle errors thrown directly by custom fetcher', async (): Promise<void> => {
    const customFetcher: ReturnType<typeof vi.fn> = vi.fn().mockImplementation(() => {
      throw new Error('Custom fetcher failure');
    });

    const operations: PatchOp[] = [{operation: 'REPLACE' as const, path: '/name', value: 'X'}];

    await expect(updateOrganization({baseUrl, fetcher: customFetcher, operations, organizationId})).rejects.toThrow(
      'Network or parsing error: Custom fetcher failure',
    );
  });

  it('should throw ThunderIDAPIError for invalid base URL', async (): Promise<void> => {
    const operations: PatchOp[] = [{operation: 'REPLACE' as const, path: '/name', value: 'X'}];

    await expect(updateOrganization({baseUrl: 'invalid-url' as any, operations, organizationId})).rejects.toThrow(
      ThunderIDAPIError,
    );

    await expect(updateOrganization({baseUrl: 'invalid-url' as any, operations, organizationId})).rejects.toThrow(
      'Invalid base URL provided.',
    );
  });

  it('should throw ThunderIDAPIError for undefined baseUrl', async (): Promise<void> => {
    const operations: PatchOp[] = [{operation: 'REPLACE' as const, path: '/name', value: 'X'}];

    await expect(updateOrganization({baseUrl: undefined as any, operations, organizationId})).rejects.toThrow(
      ThunderIDAPIError,
    );
    await expect(updateOrganization({baseUrl: undefined as any, operations, organizationId})).rejects.toThrow(
      'Invalid base URL provided.',
    );
  });

  it('should throw ThunderIDAPIError for empty string baseUrl', async (): Promise<void> => {
    const operations: PatchOp[] = [{operation: 'REPLACE' as const, path: '/name', value: 'X'}];

    await expect(updateOrganization({baseUrl: '', operations, organizationId})).rejects.toThrow(ThunderIDAPIError);
    await expect(updateOrganization({baseUrl: '', operations, organizationId})).rejects.toThrow(
      'Invalid base URL provided.',
    );
  });

  it('should throw ThunderIDAPIError when organizationId is missing', async (): Promise<void> => {
    const operations: PatchOp[] = [{operation: 'REPLACE' as const, path: '/name', value: 'X'}];

    await expect(updateOrganization({baseUrl, operations, organizationId: '' as any})).rejects.toThrow(
      ThunderIDAPIError,
    );
    await expect(updateOrganization({baseUrl, operations, organizationId: '' as any})).rejects.toThrow(
      'Organization ID is required',
    );
  });

  it('should throw ThunderIDAPIError when operations is missing/empty', async (): Promise<void> => {
    await expect(updateOrganization({baseUrl, operations: undefined as any, organizationId})).rejects.toThrow(
      'Operations array is required and cannot be empty',
    );

    await expect(updateOrganization({baseUrl, operations: [], organizationId})).rejects.toThrow(
      'Operations array is required and cannot be empty',
    );

    await expect(updateOrganization({baseUrl, operations: 'not-array' as any, organizationId})).rejects.toThrow(
      'Operations array is required and cannot be empty',
    );
  });

  it('should handle HTTP error responses', async (): Promise<void> => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 400,
      statusText: 'Bad Request',
      text: () => Promise.resolve('Invalid operations'),
    });

    const operations: PatchOp[] = [{operation: 'REPLACE' as const, path: '/name', value: 'X'}];

    await expect(updateOrganization({baseUrl, operations, organizationId})).rejects.toThrow(ThunderIDAPIError);
    await expect(updateOrganization({baseUrl, operations, organizationId})).rejects.toThrow(
      'Failed to update organization: Invalid operations',
    );
  });

  it('should handle network or parsing errors', async (): Promise<void> => {
    global.fetch = vi.fn().mockRejectedValue(new Error('Network error'));

    const operations: PatchOp[] = [{operation: 'REPLACE' as const, path: '/name', value: 'X'}];

    await expect(updateOrganization({baseUrl, operations, organizationId})).rejects.toThrow(ThunderIDAPIError);
    await expect(updateOrganization({baseUrl, operations, organizationId})).rejects.toThrow(
      'Network or parsing error: Network error',
    );
  });

  it('should handle non-Error rejections', async (): Promise<void> => {
    global.fetch = vi.fn().mockRejectedValue('unexpected failure');

    const operations: PatchOp[] = [{operation: 'REPLACE' as const, path: '/name', value: 'X'}];

    await expect(updateOrganization({baseUrl, operations, organizationId})).rejects.toThrow(
      'Network or parsing error: Unknown error',
    );
  });

  it('should include custom headers when provided', async (): Promise<void> => {
    const mockOrg: OrganizationDetails = {
      id: organizationId,
      name: 'Header Org',
      orgHandle: 'header-org',
    };

    global.fetch = vi.fn().mockResolvedValue({
      json: () => Promise.resolve(mockOrg),
      ok: true,
    });

    const operations: PatchOp[] = [{operation: 'REPLACE' as const, path: '/name', value: 'Header Org'}];

    const customHeaders: Record<string, string> = {
      Authorization: 'Bearer token',
      'X-Custom-Header': 'custom-value',
    };

    await updateOrganization({
      baseUrl,
      headers: customHeaders,
      operations,
      organizationId,
    });

    expect(fetch).toHaveBeenCalledWith(`${baseUrl}/api/server/v1/organizations/${organizationId}`, {
      body: JSON.stringify(operations),
      headers: {
        Accept: 'application/json',
        Authorization: 'Bearer token',
        'Content-Type': 'application/json',
        'X-Custom-Header': 'custom-value',
      },
      method: 'PATCH',
    });
  });

  it('should always use HTTP PATCH even if a different method is passed in requestConfig', async (): Promise<void> => {
    const mockOrg: OrganizationDetails = {
      id: organizationId,
      name: 'A',
      orgHandle: 'a',
    };

    global.fetch = vi.fn().mockResolvedValue({
      json: () => Promise.resolve(mockOrg),
      ok: true,
    });

    const operations: PatchOp[] = [{operation: 'REPLACE' as const, path: '/name', value: 'A'}];

    await updateOrganization({
      baseUrl,
      method: 'PUT',
      operations,
      organizationId,
    });

    expect(fetch).toHaveBeenCalledWith(
      `${baseUrl}/api/server/v1/organizations/${organizationId}`,
      expect.objectContaining({method: 'PATCH'}),
    );
  });

  it('should send the exact operations array as body (no mutation)', async (): Promise<void> => {
    const mockOrg: OrganizationDetails = {
      id: organizationId,
      name: 'B',
      orgHandle: 'b',
    };

    global.fetch = vi.fn().mockResolvedValue({
      json: () => Promise.resolve(mockOrg),
      ok: true,
    });

    const operations: PatchOp[] = [
      {operation: 'REPLACE' as const, path: 'name', value: 'B'},
      {operation: 'REMOVE' as const, path: 'description'},
    ];

    await updateOrganization({baseUrl, operations, organizationId});

    const [, init] = (fetch as any).mock.calls[0];
    expect(JSON.parse(init.body)).toEqual(operations);
  });
});

describe('createPatchOperations', (): void => {
  it('should generate REPLACE for non-empty values and REMOVE for empty', (): void => {
    const payload: Record<string, unknown> = {
      description: '',
      extra: 'value',
      name: 'Updated Organization',
      note: null,
    };

    const ops: PatchOp[] = createPatchOperations(payload);

    expect(ops).toEqual(
      expect.arrayContaining([
        {operation: 'REPLACE', path: '/name', value: 'Updated Organization'},
        {operation: 'REPLACE', path: '/extra', value: 'value'},
        {operation: 'REMOVE', path: '/description'},
        {operation: 'REMOVE', path: '/note'},
      ]),
    );
  });

  it('should prefix all paths with a slash', (): void => {
    const ops: PatchOp[] = createPatchOperations({
      summary: '',
      title: 'A',
    });

    expect(ops.find((o: PatchOp) => o.path === '/title')).toBeDefined();
    expect(ops.find((o: PatchOp) => o.path === '/summary')).toBeDefined();
  });

  it('should handle undefined payload values as REMOVE', (): void => {
    const ops: PatchOp[] = createPatchOperations({
      something: undefined,
    });

    expect(ops).toEqual([{operation: 'REMOVE', path: '/something'}]);
  });
});
