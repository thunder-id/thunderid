// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it, vi} from 'vitest';
import {FLOW_PAGE_SIZE} from '../../models/user-deletion';
import deleteUser, {
  executeDeletionFlow,
  findAdministrationFlowId,
  resolveDeletionFlowHandle,
  type HttpLike,
} from '../deleteUserViaFlow';

const SERVER = 'https://localhost:8090';
const USER_ID = '019fd0e4-de6c-7ea5-9541-7982f40beeb9';
const FLOW_ID = '01900000-0000-7000-8000-000000000077';
const HANDLE = 'default-user-deletion-flow';

/**
 * The request shape the deletion module issues.
 */
interface RecordedRequest {
  url: string;
  method: string;
  data?: unknown;
}

/**
 * Builds an http double that answers by URL, so each test states only the responses it cares about.
 */
function makeHttp(routes: Record<string, unknown>, onRequest?: (config: RecordedRequest) => void): HttpLike {
  return {
    request: vi.fn((config: unknown): Promise<{data?: unknown}> => {
      const request = config as RecordedRequest;
      onRequest?.(request);
      const match = Object.keys(routes).find((key) => request.url.includes(key));
      if (!match) {
        return Promise.reject(new Error(`unexpected request: ${request.method} ${request.url}`));
      }
      const value = routes[match];
      if (value instanceof Error) {
        return Promise.reject(value);
      }
      return Promise.resolve({data: value});
    }),
  };
}

describe('resolveDeletionFlowHandle', () => {
  it('should read the configured handle', async () => {
    const http = makeHttp({'/server-config/flow': {merged: {userDeletionFlow: {defaultHandle: HANDLE}}}});

    await expect(resolveDeletionFlowHandle(http, SERVER)).resolves.toBe(HANDLE);
  });

  it('should return an empty handle when the section is absent', async () => {
    const http = makeHttp({'/server-config/flow': {merged: {}}});

    await expect(resolveDeletionFlowHandle(http, SERVER)).resolves.toBe('');
  });

  // Treating a failed read as "no flow configured" would perform a deletion that skips revocation and
  // report it as a success, hiding the failure entirely.
  it('should propagate a failed config read rather than assuming no flow', async () => {
    const http = makeHttp({'/server-config/flow': new Error('boom')});

    await expect(resolveDeletionFlowHandle(http, SERVER)).rejects.toThrow('boom');
  });

  // The endpoint wraps the section in readOnly/writable/merged layers. Reading the envelope directly
  // would silently yield undefined and fall back to native, which is what this pins against.
  it('should read the merged layer, not the envelope', async () => {
    const http = makeHttp({
      '/server-config/flow': {
        merged: {userDeletionFlow: {defaultHandle: HANDLE}},
        readOnly: {userDeletionFlow: {}},
        writable: {userDeletionFlow: {defaultHandle: HANDLE}},
      },
    });

    await expect(resolveDeletionFlowHandle(http, SERVER)).resolves.toBe(HANDLE);
  });
});

describe('findAdministrationFlowId', () => {
  it('should resolve a handle to its flow id', async () => {
    const http = makeHttp({
      '/flows': {
        flows: [
          {flowType: 'ADMINISTRATION', handle: 'other', id: 'wrong'},
          {flowType: 'ADMINISTRATION', handle: HANDLE, id: FLOW_ID},
        ],
      },
    });

    await expect(findAdministrationFlowId(http, SERVER, HANDLE)).resolves.toBe(FLOW_ID);
  });

  it('should return null when no administration flow carries the handle', async () => {
    const http = makeHttp({'/flows': {flows: []}});

    await expect(findAdministrationFlowId(http, SERVER, 'missing')).resolves.toBeNull();
  });

  // The listing is ordered newest first, so the bootstrap deletion flow sits on the last page once a
  // deployment accumulates administration flows. Stopping at the first page would stop finding it.
  it('should walk pages until the handle is found', async () => {
    const firstPage = Array.from({length: FLOW_PAGE_SIZE}, (_unused, index) => ({
      flowType: 'ADMINISTRATION',
      handle: `filler-${index}`,
      id: `filler-${index}`,
    }));
    const requested: string[] = [];
    const http: HttpLike = {
      request: vi.fn((config: unknown): Promise<{data?: unknown}> => {
        const {url} = config as RecordedRequest;
        requested.push(url);
        const flows = url.includes('offset=0')
          ? firstPage
          : [{flowType: 'ADMINISTRATION', handle: HANDLE, id: FLOW_ID}];
        return Promise.resolve({data: {flows}});
      }),
    };

    await expect(findAdministrationFlowId(http, SERVER, HANDLE)).resolves.toBe(FLOW_ID);
    expect(requested).toHaveLength(2);
    expect(requested[1]).toContain(`offset=${FLOW_PAGE_SIZE}`);
  });
});

describe('executeDeletionFlow', () => {
  it('should send the flow id and subject and accept a completed flow', async () => {
    let sent: RecordedRequest | undefined;
    const http = makeHttp({'/flow/execute': {flowStatus: 'COMPLETE'}}, (config) => {
      sent = config;
    });

    await expect(executeDeletionFlow(http, SERVER, FLOW_ID, USER_ID)).resolves.toBeUndefined();
    expect(sent?.data).toEqual({flowId: FLOW_ID, inputs: {subject: USER_ID}});
  });

  // Every required input is supplied up front, so a pause means the flow asks for something this
  // caller cannot provide. Reporting it beats leaving the user silently undeleted.
  it('should throw when the flow pauses for input', async () => {
    const http = makeHttp({'/flow/execute': {executionId: 'exec-1', flowStatus: 'INCOMPLETE'}});

    await expect(executeDeletionFlow(http, SERVER, FLOW_ID, USER_ID)).rejects.toThrow('additional input');
  });

  // A refused step reports its executor error in `error`, and the code is what lets the console show
  // the specific reason instead of a generic failure, so it must survive on the thrown error.
  it('should carry the code and message of a refused step', async () => {
    const http = makeHttp({
      '/flow/execute': {
        error: {
          code: 'FET-1084',
          message: {defaultValue: 'user has dependencies', key: 'flows.executor.errors.user_deletion_not_allowed'},
        },
        flowStatus: 'ERROR',
      },
    });

    await expect(executeDeletionFlow(http, SERVER, FLOW_ID, USER_ID)).rejects.toThrow('user has dependencies');

    const failure = await executeDeletionFlow(http, SERVER, FLOW_ID, USER_ID).catch(
      (err: unknown) => err as {code?: string},
    );

    expect(failure.code).toBe('FET-1084');
  });

  // A step can fail without an error envelope, and the deletion must still report a failure rather
  // than an empty message.
  it('should fall back to a named message when a failed step carries no error', async () => {
    const http = makeHttp({'/flow/execute': {flowStatus: 'ERROR'}});

    await expect(executeDeletionFlow(http, SERVER, FLOW_ID, USER_ID)).rejects.toThrow(
      'The user deletion flow did not complete',
    );
  });
});

describe('deleteUser', () => {
  it('should run the flow when one is configured and exists', async () => {
    const calls: RecordedRequest[] = [];
    const http = makeHttp(
      {
        '/flow/execute': {flowStatus: 'COMPLETE'},
        '/flows': {flows: [{flowType: 'ADMINISTRATION', handle: HANDLE, id: FLOW_ID}]},
        '/server-config/flow': {merged: {userDeletionFlow: {defaultHandle: HANDLE}}},
      },
      (config) => calls.push(config),
    );

    await deleteUser(http, SERVER, USER_ID);

    expect(calls.some((c) => c.url.includes('/flow/execute'))).toBe(true);
    expect(calls.some((c) => c.method === 'DELETE')).toBe(false);
  });

  it('should call the native endpoint when no flow is configured', async () => {
    const calls: RecordedRequest[] = [];
    const http = makeHttp({'/server-config/flow': {merged: {}}, '/users/': {}}, (config) => calls.push(config));

    await deleteUser(http, SERVER, USER_ID);

    const deleteCall = calls.find((c) => c.method === 'DELETE');
    expect(deleteCall?.url).toBe(`${SERVER}/users/${USER_ID}`);
    expect(calls.some((c) => c.url.includes('/flow/execute'))).toBe(false);
  });

  // The configured flow may have been deleted or renamed. Falling back keeps deletion working, the
  // same way user onboarding falls back to manual creation when its flow is missing.
  it('should fall back to the native endpoint when the configured flow does not exist', async () => {
    const calls: RecordedRequest[] = [];
    const http = makeHttp(
      {
        '/flows': {flows: []},
        '/server-config/flow': {merged: {userDeletionFlow: {defaultHandle: 'was-deleted'}}},
        '/users/': {},
      },
      (config) => calls.push(config),
    );

    await deleteUser(http, SERVER, USER_ID);

    expect(calls.some((c) => c.method === 'DELETE')).toBe(true);
    expect(calls.some((c) => c.url.includes('/flow/execute'))).toBe(false);
  });

  // A flow that exists but refuses must not be downgraded to a deletion that skips revocation.
  it('should not fall back when the existing flow fails', async () => {
    const calls: RecordedRequest[] = [];
    const http = makeHttp(
      {
        '/flow/execute': {
          error: {
            code: 'FET-1084',
            message: {defaultValue: 'user has dependencies', key: 'flows.executor.errors.user_deletion_not_allowed'},
          },
          flowStatus: 'ERROR',
        },
        '/flows': {flows: [{flowType: 'ADMINISTRATION', handle: HANDLE, id: FLOW_ID}]},
        '/server-config/flow': {merged: {userDeletionFlow: {defaultHandle: HANDLE}}},
        '/users/': {},
      },
      (config) => calls.push(config),
    );

    await expect(deleteUser(http, SERVER, USER_ID)).rejects.toThrow('user has dependencies');
    expect(calls.some((c) => c.method === 'DELETE')).toBe(false);
  });
});
