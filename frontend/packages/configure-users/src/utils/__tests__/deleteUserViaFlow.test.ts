// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it, vi} from 'vitest';
import deleteUser, {type HttpLike} from '../deleteUserViaFlow';

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
    // The executor declares this input, so the key is part of the contract rather than a detail. A
    // wrong key reaches the server as an unrecognised input and the flow pauses asking for the real
    // one, which surfaces as a failure rather than a silent no-op.
    expect(calls.find((c) => c.url.includes('/flow/execute'))?.data).toMatchObject({
      inputs: {subject: USER_ID},
    });
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
