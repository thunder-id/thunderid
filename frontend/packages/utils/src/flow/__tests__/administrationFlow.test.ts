// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it, vi} from 'vitest';
import {
  AdministrationFlowConfigKey,
  FLOW_PAGE_SIZE,
  FlowExecutionFailure,
  findAdministrationFlowId,
  resolveAdministrationFlowHandle,
  runAdministrationFlow,
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
