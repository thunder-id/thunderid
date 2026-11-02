// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it, vi} from 'vitest';
import {
  AdministrationFlowConfigKey,
  FLOW_PAGE_SIZE,
  FlowExecutionFailure,
  findAdministrationFlowId,
  findAdministrationFlows,
  resolveAdministrationFlowHandle,
  resolveAdministrationFlowHandles,
  resolveAdministrationFlowId,
  resolveAdministrationFlows,
  runAdministrationFlow,
  type HttpLike,
} from '../administrationFlow';

const SERVER = 'https://localhost:8090';

/**
 * A client that answers each url from a table, so a test states only what it cares about.
 */
function httpFor(routes: Record<string, unknown>, onRequest?: (config: {url: string}) => void) {
  return {
    request: vi.fn((config: unknown) => {
      const {url} = config as {url: string};

      onRequest?.({url});
      const match = Object.keys(routes).find((key) => url.startsWith(key));

      return Promise.resolve({data: match ? routes[match] : undefined});
    }),
  };
}

function configuring(handle: string) {
  return {[`${SERVER}/server-config/flow`]: {merged: {roleDeletionFlow: {defaultHandle: handle}}}};
}

describe('resolveAdministrationFlowHandle', () => {
  it('reads the handle from the merged layer, which is the effective configuration', async () => {
    const http = httpFor({
      [`${SERVER}/server-config/flow`]: {
        readOnly: {roleDeletionFlow: {defaultHandle: 'declared'}},
        writable: {roleDeletionFlow: {defaultHandle: 'written'}},
        merged: {roleDeletionFlow: {defaultHandle: 'effective'}},
      },
    });

    await expect(
      resolveAdministrationFlowHandle(http, SERVER, AdministrationFlowConfigKey.ROLE_DELETION),
    ).resolves.toBe('effective');
  });

  it('returns an empty handle when the action has no flow configured', async () => {
    const http = httpFor({[`${SERVER}/server-config/flow`]: {merged: {}}});

    await expect(
      resolveAdministrationFlowHandle(http, SERVER, AdministrationFlowConfigKey.ROLE_DELETION),
    ).resolves.toBe('');
  });

  it('propagates a read failure instead of reporting no flow', async () => {
    const http = {request: vi.fn(() => Promise.reject(new Error('server-config unreachable')))};

    await expect(
      resolveAdministrationFlowHandle(http, SERVER, AdministrationFlowConfigKey.ROLE_DELETION),
    ).rejects.toThrow('server-config unreachable');
  });

  it('propagates a permission denial instead of falling back to the native endpoint', async () => {
    const denied = Object.assign(new Error('Forbidden'), {response: {status: 403}});
    const http = {request: vi.fn(() => Promise.reject(denied))};

    await expect(resolveAdministrationFlowHandle(http, SERVER, AdministrationFlowConfigKey.ROLE_DELETION)).rejects.toBe(
      denied,
    );
  });

  it('does not run the native fallback when the flow configuration read is denied', async () => {
    const denied = Object.assign(new Error('Forbidden'), {status: 403});
    const http = {request: vi.fn(() => Promise.reject(denied))};

    await expect(
      runAdministrationFlow(http, SERVER, AdministrationFlowConfigKey.ROLE_DELETION, {}, 'role deletion'),
    ).rejects.toBe(denied);
  });
});

describe('findAdministrationFlowId', () => {
  it('walks past a full page, since the bootstrap flows sort last', async () => {
    const filler = Array.from({length: FLOW_PAGE_SIZE}, (_unused, index) => ({
      id: `other-${index}`,
      handle: `other-${index}`,
      flowType: 'ADMINISTRATION',
    }));
    const pages = [filler, [{id: 'wanted-id', handle: 'wanted', flowType: 'ADMINISTRATION'}]];
    const http = {
      request: vi.fn((config: unknown) => {
        const {url} = config as {url: string};

        return Promise.resolve({data: {flows: url.includes('offset=0') ? pages[0] : pages[1]}});
      }),
    };

    await expect(findAdministrationFlowId(http, SERVER, 'wanted')).resolves.toBe('wanted-id');
    expect(http.request).toHaveBeenCalledTimes(2);
  });

  it('stops on a short page rather than paging forever', async () => {
    const http = httpFor({[`${SERVER}/flows`]: {flows: [{id: 'a', handle: 'a', flowType: 'ADMINISTRATION'}]}});

    await expect(findAdministrationFlowId(http, SERVER, 'absent')).resolves.toBeNull();
    expect(http.request).toHaveBeenCalledTimes(1);
  });
});

describe('resolveAdministrationFlowId', () => {
  it('returns the id of the flow the handle names', async () => {
    const http = httpFor({
      [`${SERVER}/flows`]: {flows: [{id: 'flow-1', handle: 'wanted', flowType: 'ADMINISTRATION'}]},
    });

    await expect(resolveAdministrationFlowId(http, SERVER, 'wanted', 'role deletion')).resolves.toBe('flow-1');
  });

  it('refuses when the handle names no existing flow, without reading the flow configuration', async () => {
    const http = httpFor({[`${SERVER}/flows`]: {flows: []}});

    await expect(resolveAdministrationFlowId(http, SERVER, 'stale-handle', 'role deletion')).rejects.toThrow(
      'no longer exists',
    );
    expect(http.request).toHaveBeenCalledTimes(1);
  });
});

describe('runAdministrationFlow', () => {
  it('returns null when no flow is configured, so the caller falls back', async () => {
    const http = httpFor({[`${SERVER}/server-config/flow`]: {merged: {}}});

    await expect(
      runAdministrationFlow(http, SERVER, AdministrationFlowConfigKey.ROLE_DELETION, {}, 'role deletion'),
    ).resolves.toBeNull();
  });

  it('refuses when a configured handle names no existing flow, rather than falling back', async () => {
    const http = httpFor({...configuring('stale-handle'), [`${SERVER}/flows`]: {flows: []}});

    await expect(
      runAdministrationFlow(http, SERVER, AdministrationFlowConfigKey.ROLE_DELETION, {}, 'role deletion'),
    ).rejects.toThrow('no longer exists');
  });

  it('sends the inputs it was given and returns the values the flow produced', async () => {
    const seen: {url: string}[] = [];
    const http = httpFor(
      {
        ...configuring('default-role-deletion-flow'),
        [`${SERVER}/flows`]: {
          flows: [{id: 'flow-1', handle: 'default-role-deletion-flow', flowType: 'ADMINISTRATION'}],
        },
        [`${SERVER}/flow/execute`]: {flowStatus: 'COMPLETE', data: {additionalData: {ok: 'yes'}}},
      },
      (config) => seen.push(config),
    );

    await expect(
      runAdministrationFlow(
        http,
        SERVER,
        AdministrationFlowConfigKey.ROLE_DELETION,
        {targetRoleId: 'role-1'},
        'role deletion',
      ),
    ).resolves.toEqual({ok: 'yes'});

    const execution = http.request.mock.calls.find(
      ([config]) => (config as {url: string}).url === `${SERVER}/flow/execute`,
    );

    expect((execution?.[0] as {data: unknown}).data).toEqual({
      flowId: 'flow-1',
      inputs: {targetRoleId: 'role-1'},
    });
  });

  it('carries a refusal code out, so the feature can localize it', async () => {
    const http = httpFor({
      ...configuring('default-role-deletion-flow'),
      [`${SERVER}/flows`]: {
        flows: [{id: 'flow-1', handle: 'default-role-deletion-flow', flowType: 'ADMINISTRATION'}],
      },
      [`${SERVER}/flow/execute`]: {
        flowStatus: 'ERROR',
        error: {code: 'ROL-1013', message: {defaultValue: 'Cannot modify declarative role'}},
      },
    });

    await expect(
      runAdministrationFlow(http, SERVER, AdministrationFlowConfigKey.ROLE_DELETION, {}, 'role deletion'),
    ).rejects.toMatchObject({code: 'ROL-1013'});
  });

  it('treats a paused flow as an error, not a success that did nothing', async () => {
    const http = httpFor({
      ...configuring('default-role-deletion-flow'),
      [`${SERVER}/flows`]: {
        flows: [{id: 'flow-1', handle: 'default-role-deletion-flow', flowType: 'ADMINISTRATION'}],
      },
      [`${SERVER}/flow/execute`]: {flowStatus: 'INCOMPLETE'},
    });

    await expect(
      runAdministrationFlow(http, SERVER, AdministrationFlowConfigKey.ROLE_DELETION, {}, 'role deletion'),
    ).rejects.toThrow('requires additional input');
  });
});

describe('FlowExecutionFailure', () => {
  it('prefers the message over the description, and falls back when neither is set', () => {
    expect(
      new FlowExecutionFailure({message: {defaultValue: 'm'}, description: {defaultValue: 'd'}}, 'f').message,
    ).toBe('m');
    expect(new FlowExecutionFailure({description: {defaultValue: 'd'}}, 'f').message).toBe('d');
    expect(new FlowExecutionFailure({}, 'fallback').message).toBe('fallback');
  });
});

/**
 * A client that answers each url containing a key from a table, recording every url requested.
 */
function httpReturning(responses: Record<string, unknown>): {http: HttpLike; calls: string[]} {
  const calls: string[] = [];
  const http: HttpLike = {
    request: vi.fn((config: unknown) => {
      const {url} = config as {url: string};

      calls.push(url);
      const match = Object.keys(responses).find((key) => url.includes(key));

      return Promise.resolve({data: match ? responses[match] : undefined});
    }),
  };

  return {calls, http};
}

describe('resolveAdministrationFlowHandles', () => {
  it('reads the merged layer, not the envelope', async () => {
    // Only the merged layer is the effective configuration.
    const {http} = httpReturning({
      '/server-config/flow': {
        merged: {userSuspendFlow: {defaultHandle: 'merged-handle'}},
        readOnly: {userSuspendFlow: {defaultHandle: 'declarative-handle'}},
      },
    });

    await expect(resolveAdministrationFlowHandles(http, SERVER, ['userSuspendFlow'])).resolves.toEqual({
      userSuspendFlow: 'merged-handle',
    });
  });

  it('omits a purpose that is configured to nothing', async () => {
    // Absent, so the caller withholds the action.
    const {http} = httpReturning({'/server-config/flow': {merged: {userUnlockFlow: {}}}});

    await expect(
      resolveAdministrationFlowHandles(http, SERVER, ['userSuspendFlow', 'userUnlockFlow']),
    ).resolves.toEqual({});
  });

  it('propagates a read failure rather than reporting nothing configured', async () => {
    // Swallowing it would silently withdraw every lifecycle action.
    const http: HttpLike = {request: vi.fn().mockRejectedValue(new Error('boom'))};

    await expect(resolveAdministrationFlowHandles(http, SERVER, ['userSuspendFlow'])).rejects.toThrow('boom');
  });
});

describe('findAdministrationFlows', () => {
  it('resolves every handle in one page walk', async () => {
    const a = {flowType: 'ADMINISTRATION', handle: 'a', id: 'id-a', name: 'A', updatedAt: '2026-10-01T00:00:00Z'};
    const b = {flowType: 'ADMINISTRATION', handle: 'b', id: 'id-b', name: 'B', updatedAt: '2026-10-02T00:00:00Z'};
    const {calls, http} = httpReturning({'/flows': {flows: [a, b]}});

    await expect(findAdministrationFlows(http, SERVER, ['a', 'b'])).resolves.toEqual({a, b});
    // One listing request, not one per handle.
    expect(calls).toHaveLength(1);
  });

  it('walks past a full page to find a handle further along', async () => {
    // The listing is newest first, so bootstrap flows sink as a deployment adds its own.
    const firstPage = Array.from({length: 100}, (_unused, index) => ({
      flowType: 'ADMINISTRATION',
      handle: `other-${index}`,
      id: `id-${index}`,
    }));
    let page = 0;
    const http: HttpLike = {
      request: vi.fn(() => {
        page += 1;

        return Promise.resolve({
          data: {flows: page === 1 ? firstPage : [{flowType: 'ADMINISTRATION', handle: 'wanted', id: 'id-w'}]},
        });
      }),
    };

    await expect(findAdministrationFlows(http, SERVER, ['wanted'])).resolves.toEqual({
      wanted: {flowType: 'ADMINISTRATION', handle: 'wanted', id: 'id-w'},
    });
    expect(page).toBe(2);
  });

  it('ignores a handle carried by a flow of another type', async () => {
    const {http} = httpReturning({
      '/flows': {flows: [{flowType: 'AUTHENTICATION', handle: 'a', id: 'id-a'}]},
    });

    await expect(findAdministrationFlows(http, SERVER, ['a'])).resolves.toEqual({});
  });

  it('makes no request when there is nothing to look for', async () => {
    const {calls, http} = httpReturning({});

    await expect(findAdministrationFlows(http, SERVER, [])).resolves.toEqual({});
    expect(calls).toHaveLength(0);
  });
});

describe('resolveAdministrationFlows', () => {
  it('returns the listed flow for each purpose whose configured handle names one', async () => {
    // Either half missing means the operation is not offered.
    const suspend = {
      flowType: 'ADMINISTRATION',
      handle: 'suspend-handle',
      id: 'flow-1',
      isReadOnly: true,
      name: 'Suspend',
      updatedAt: '2026-10-01T00:00:00Z',
    };
    const {http} = httpReturning({
      '/flows': {flows: [suspend]},
      '/server-config/flow': {
        merged: {
          userSuspendFlow: {defaultHandle: 'suspend-handle'},
          agentSuspendFlow: {defaultHandle: 'suspend-handle'},
          userUnlockFlow: {defaultHandle: 'handle-of-a-deleted-flow'},
        },
      },
    });

    await expect(
      resolveAdministrationFlows(http, SERVER, ['userSuspendFlow', 'agentSuspendFlow', 'userUnlockFlow']),
    ).resolves.toStrictEqual({userSuspendFlow: suspend, agentSuspendFlow: suspend});
  });
});
