// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it, vi} from 'vitest';
import type {UpdateRoleRequest} from '../../models/requests';
import type {Role} from '../../models/role';
import {permissionsRemoved, updateRoleViaFlow} from '../roleAdministrationFlow';

const SERVER = 'https://localhost:8090';
const ROLE_ID = 'role-1';
const FLOW_HANDLE = 'default-role-permission-removal-flow';

interface RecordedRequest {
  url: string;
  method: string;
  data?: unknown;
}

/**
 * A client that answers each url from a table and records every request in order, so a test can
 * assert what was written and when.
 */
function httpFor(routes: Record<string, unknown>) {
  const requests: RecordedRequest[] = [];

  return {
    requests,
    request: vi.fn((config: unknown) => {
      const {url, method, data} = config as RecordedRequest;

      requests.push({url, method, data});
      const match = Object.keys(routes).find((key) => url.startsWith(key));

      return Promise.resolve({data: match ? routes[match] : undefined});
    }),
  };
}

function configuring(handle?: string) {
  return {
    [`${SERVER}/server-config/flow`]: {
      merged: handle === undefined ? {} : {rolePermissionRemovalFlow: {defaultHandle: handle}},
    },
  };
}

const flowListing = {
  [`${SERVER}/flows`]: {flows: [{id: 'flow-1', handle: FLOW_HANDLE, flowType: 'ADMINISTRATION'}]},
};

const flowCompletes = {[`${SERVER}/flow/execute`]: {flowStatus: 'COMPLETE'}};

const current: Role = {
  id: ROLE_ID,
  name: 'Editor',
  description: 'Edits things',
  ouId: 'ou-1',
  permissions: [
    {resourceServerId: 'rs-a', permissions: ['read', 'write']},
    {resourceServerId: 'rs-b', permissions: ['read']},
  ],
};

function requestFor(overrides: Partial<UpdateRoleRequest>): UpdateRoleRequest {
  return {
    name: current.name,
    description: current.description,
    ouId: current.ouId,
    permissions: current.permissions ?? [],
    ...overrides,
  };
}

const isWrite = (request: RecordedRequest): boolean => request.method !== 'GET';

describe('permissionsRemoved', () => {
  it('reports a removal when a whole resource server is dropped', () => {
    expect(permissionsRemoved(current.permissions, [{resourceServerId: 'rs-a', permissions: ['read', 'write']}])).toBe(
      true,
    );
  });

  it('reports a removal from one resource server even while another gains permissions', () => {
    expect(
      permissionsRemoved(current.permissions, [
        {resourceServerId: 'rs-a', permissions: ['read']},
        {resourceServerId: 'rs-b', permissions: ['read', 'write', 'delete']},
      ]),
    ).toBe(true);
  });

  // Permission names are scoped to their resource server, so the same name kept elsewhere is
  // still a removal.
  it('treats a permission moved to another resource server as removed from the original one', () => {
    expect(
      permissionsRemoved(
        [{resourceServerId: 'rs-a', permissions: ['read']}],
        [{resourceServerId: 'rs-b', permissions: ['read']}],
      ),
    ).toBe(true);
  });

  it('reports no removal when permissions are only added', () => {
    expect(
      permissionsRemoved(current.permissions, [
        {resourceServerId: 'rs-a', permissions: ['read', 'write', 'delete']},
        {resourceServerId: 'rs-b', permissions: ['read']},
        {resourceServerId: 'rs-c', permissions: ['read']},
      ]),
    ).toBe(false);
  });

  it('reports no removal when the role had no permissions', () => {
    expect(permissionsRemoved(undefined, [{resourceServerId: 'rs-a', permissions: ['read']}])).toBe(false);
  });
});

describe('updateRoleViaFlow', () => {
  it('updates natively without reading the flow configuration when nothing is removed', async () => {
    const data = requestFor({
      permissions: [...(current.permissions ?? []), {resourceServerId: 'rs-c', permissions: ['read']}],
    });
    const http = httpFor({[`${SERVER}/roles/${ROLE_ID}`]: {...current, permissions: data.permissions}});

    const updated = await updateRoleViaFlow(http, SERVER, ROLE_ID, data, current);

    expect(http.requests).toEqual([{url: `${SERVER}/roles/${ROLE_ID}`, method: 'PUT', data}]);
    expect(updated.permissions).toEqual(data.permissions);
  });

  it('updates natively when a permission is removed but no flow is configured', async () => {
    const data = requestFor({permissions: [{resourceServerId: 'rs-a', permissions: ['read', 'write']}]});
    const http = httpFor({
      ...configuring(),
      [`${SERVER}/roles/${ROLE_ID}`]: {...current, permissions: data.permissions},
    });

    await updateRoleViaFlow(http, SERVER, ROLE_ID, data, current);

    expect(http.requests.map((request) => request.url)).toEqual([
      `${SERVER}/server-config/flow`,
      `${SERVER}/roles/${ROLE_ID}`,
    ]);
    expect(http.requests[1]).toMatchObject({method: 'PUT', data});
  });

  it('runs the flow with the new permission set when a whole resource server is removed', async () => {
    const next = [{resourceServerId: 'rs-a', permissions: ['read', 'write']}];
    const http = httpFor({...configuring(FLOW_HANDLE), ...flowListing, ...flowCompletes});

    const updated = await updateRoleViaFlow(http, SERVER, ROLE_ID, requestFor({permissions: next}), current);

    const writes = http.requests.filter(isWrite);
    expect(writes).toEqual([
      {
        url: `${SERVER}/flow/execute`,
        method: 'POST',
        data: {flowId: 'flow-1', inputs: {targetPermissions: JSON.stringify(next), targetRoleId: ROLE_ID}},
      },
    ]);
    expect(updated).toEqual({...current, permissions: next});
  });

  it('runs the flow when one resource server loses a permission while another gains one', async () => {
    const next = [
      {resourceServerId: 'rs-a', permissions: ['read']},
      {resourceServerId: 'rs-b', permissions: ['read', 'write']},
    ];
    const http = httpFor({...configuring(FLOW_HANDLE), ...flowListing, ...flowCompletes});

    await updateRoleViaFlow(http, SERVER, ROLE_ID, requestFor({permissions: next}), current);

    expect(http.requests.filter(isWrite).map((request) => request.url)).toEqual([`${SERVER}/flow/execute`]);
  });

  // A stale handle must not fall back to the native endpoint, which would remove the permissions
  // without revoking anything.
  it('refuses without writing anything when the configured flow no longer exists', async () => {
    const http = httpFor({...configuring('stale-handle'), ...flowListing});

    await expect(
      updateRoleViaFlow(
        http,
        SERVER,
        ROLE_ID,
        requestFor({name: 'Renamed', permissions: [{resourceServerId: 'rs-a', permissions: ['read']}]}),
        current,
      ),
    ).rejects.toThrow('no longer exists');
    expect(http.requests.filter(isWrite)).toEqual([]);
  });

  // The rename and the flow are separate writes, so the flow id is resolved first: a missing flow
  // must not leave a half-applied update behind.
  it('resolves the flow id before any write, then applies other fields before the flow', async () => {
    const next = [{resourceServerId: 'rs-a', permissions: ['read']}];
    const http = httpFor({
      ...configuring(FLOW_HANDLE),
      ...flowListing,
      ...flowCompletes,
      [`${SERVER}/roles/${ROLE_ID}`]: {...current, name: 'Renamed'},
    });

    const updated = await updateRoleViaFlow(
      http,
      SERVER,
      ROLE_ID,
      requestFor({name: 'Renamed', permissions: next}),
      current,
    );

    expect(http.requests.map((request) => `${request.method} ${request.url.split('?')[0]}`)).toEqual([
      `GET ${SERVER}/server-config/flow`,
      `GET ${SERVER}/flows`,
      `PUT ${SERVER}/roles/${ROLE_ID}`,
      `POST ${SERVER}/flow/execute`,
    ]);
    // The native write keeps the current permissions, so only the flow removes any.
    expect(http.requests[2].data).toEqual(requestFor({name: 'Renamed'}));
    expect(updated).toEqual({...current, name: 'Renamed', permissions: next});
  });
});
