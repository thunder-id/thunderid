// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it, vi} from 'vitest';
import {FLOW_PAGE_SIZE} from '../../models/application-administration-flow';
import {
  deleteApplicationViaFlow,
  executeApplicationFlow,
  findAdministrationFlowId,
  regenerateClientSecretViaFlow,
  resolveApplicationFlowHandle,
  type HttpLike,
} from '../applicationAdministrationFlow';

const SERVER = 'https://localhost:8090';
const APP_ID = '01a03248-56f6-75a8-a821-eb93cd60fc7b';
const DELETION_FLOW_ID = '01900000-0000-7000-8000-000000000079';
const ROTATION_FLOW_ID = '01900000-0000-7000-8000-00000000007a';
const DELETION_HANDLE = 'default-application-deletion-flow';
const ROTATION_HANDLE = 'default-client-secret-regeneration-flow';
const NEW_SECRET = 'KdJL7PrfDTh-XwT8kGtg1nMydcDQg_L5an25vV52Ej8';

/**
 * The request shape the administration module issues.
 */
interface RecordedRequest {
  url: string;
  method: string;
  data?: unknown;
}

/**
 * Builds an http double that answers by URL, so each test states only the responses it cares about.
 *
 * An action issues up to three requests (read the handle, resolve it to an id, then execute), so a
 * positional double would attach a response to the wrong call.
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

describe('resolveApplicationFlowHandle', () => {
  it('should read the configured handle for the requested action', async () => {
    const http = makeHttp({
      '/server-config/flow': {
        merged: {
          applicationDeletionFlow: {defaultHandle: DELETION_HANDLE},
          clientSecretRegenerationFlow: {defaultHandle: ROTATION_HANDLE},
        },
      },
    });

    await expect(resolveApplicationFlowHandle(http, SERVER, 'applicationDeletionFlow')).resolves.toBe(DELETION_HANDLE);
    await expect(resolveApplicationFlowHandle(http, SERVER, 'clientSecretRegenerationFlow')).resolves.toBe(
      ROTATION_HANDLE,
    );
  });

  it('should return an empty handle when the entry is absent', async () => {
    const http = makeHttp({'/server-config/flow': {merged: {}}});

    await expect(resolveApplicationFlowHandle(http, SERVER, 'applicationDeletionFlow')).resolves.toBe('');
  });

  // Treating a failed read as "no flow configured" would perform the action through the native endpoint,
  // which revokes nothing, and report it as a success.
  it('should propagate a failed config read rather than assuming no flow', async () => {
    const http = makeHttp({'/server-config/flow': new Error('boom')});

    await expect(resolveApplicationFlowHandle(http, SERVER, 'applicationDeletionFlow')).rejects.toThrow('boom');
  });

  // The endpoint wraps the section in readOnly/writable/merged layers. Reading the envelope directly
  // would silently yield undefined and fall back to native, which is what this pins against.
  it('should read the merged layer, not the envelope', async () => {
    const http = makeHttp({
      '/server-config/flow': {
        merged: {applicationDeletionFlow: {defaultHandle: DELETION_HANDLE}},
        readOnly: {applicationDeletionFlow: {}},
        writable: {applicationDeletionFlow: {defaultHandle: DELETION_HANDLE}},
      },
    });

    await expect(resolveApplicationFlowHandle(http, SERVER, 'applicationDeletionFlow')).resolves.toBe(DELETION_HANDLE);
  });
});

describe('findAdministrationFlowId', () => {
  it('should resolve a handle to its flow id', async () => {
    const http = makeHttp({
      '/flows': {
        flows: [
          {flowType: 'ADMINISTRATION', handle: 'other', id: 'wrong'},
          {flowType: 'ADMINISTRATION', handle: DELETION_HANDLE, id: DELETION_FLOW_ID},
        ],
      },
    });

    await expect(findAdministrationFlowId(http, SERVER, DELETION_HANDLE)).resolves.toBe(DELETION_FLOW_ID);
  });

  it('should return null when no administration flow carries the handle', async () => {
    const http = makeHttp({'/flows': {flows: []}});

    await expect(findAdministrationFlowId(http, SERVER, 'missing')).resolves.toBeNull();
  });

  // The listing is ordered newest first, so the bootstrap flows sit on the last page once a deployment
  // accumulates administration flows. Stopping at the first page would stop finding them.
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
          : [{flowType: 'ADMINISTRATION', handle: DELETION_HANDLE, id: DELETION_FLOW_ID}];
        return Promise.resolve({data: {flows}});
      }),
    };

    await expect(findAdministrationFlowId(http, SERVER, DELETION_HANDLE)).resolves.toBe(DELETION_FLOW_ID);
    expect(requested).toHaveLength(2);
    expect(requested[1]).toContain(`offset=${FLOW_PAGE_SIZE}`);
  });
});

describe('executeApplicationFlow', () => {
  // The executors read the target from targetApplicationId, not applicationId, which the execution
  // request already uses at its top level.
  it('should post the target under targetApplicationId', async () => {
    const seen: RecordedRequest[] = [];
    const http = makeHttp({'/flow/execute': {flowStatus: 'COMPLETE'}}, (config) => seen.push(config));

    await executeApplicationFlow(http, SERVER, DELETION_FLOW_ID, APP_ID, 'application deletion');

    expect(seen[0].data).toEqual({flowId: DELETION_FLOW_ID, inputs: {targetApplicationId: APP_ID}});
  });

  it('should return the values the flow produced', async () => {
    const http = makeHttp({
      '/flow/execute': {data: {additionalData: {clientSecret: NEW_SECRET}}, flowStatus: 'COMPLETE'},
    });

    await expect(
      executeApplicationFlow(http, SERVER, ROTATION_FLOW_ID, APP_ID, 'client secret regeneration'),
    ).resolves.toEqual({clientSecret: NEW_SECRET});
  });

  // Every required input is supplied up front, so a flow that still pauses is asking for something this
  // caller cannot provide. Reporting success would claim an action that never happened.
  it('should reject an incomplete execution', async () => {
    const http = makeHttp({'/flow/execute': {flowStatus: 'INCOMPLETE'}});

    await expect(
      executeApplicationFlow(http, SERVER, DELETION_FLOW_ID, APP_ID, 'application deletion'),
    ).rejects.toThrow('requires additional input');
  });

  // A refused step reports its executor error in `error`, and the code is what lets the console show
  // the specific reason instead of a generic failure, so it must survive on the thrown error.
  it('should carry the code and message of a refused step', async () => {
    const http = makeHttp({
      '/flow/execute': {
        error: {
          code: 'FET-1088',
          message: {defaultValue: 'Client secret regeneration not allowed', key: 'flows.executor.errors.x'},
        },
        flowStatus: 'ERROR',
      },
    });

    await expect(
      executeApplicationFlow(http, SERVER, ROTATION_FLOW_ID, APP_ID, 'client secret regeneration'),
    ).rejects.toThrow('Client secret regeneration not allowed');

    const failure = await executeApplicationFlow(
      http,
      SERVER,
      ROTATION_FLOW_ID,
      APP_ID,
      'client secret regeneration',
    ).catch((err: unknown) => err as {code?: string});

    expect(failure.code).toBe('FET-1088');
  });

  // A step can fail without an error envelope, and the action must still report a failure rather than
  // an empty message.
  it('should fall back to a named message when a failed step carries no error', async () => {
    const http = makeHttp({'/flow/execute': {flowStatus: 'ERROR'}});

    await expect(
      executeApplicationFlow(http, SERVER, DELETION_FLOW_ID, APP_ID, 'application deletion'),
    ).rejects.toThrow('The application deletion flow did not complete');
  });
});

describe('deleteApplicationViaFlow', () => {
  it('should delete through the configured flow', async () => {
    const seen: RecordedRequest[] = [];
    const http = makeHttp(
      {
        '/flow/execute': {flowStatus: 'COMPLETE'},
        '/flows': {flows: [{flowType: 'ADMINISTRATION', handle: DELETION_HANDLE, id: DELETION_FLOW_ID}]},
        '/server-config/flow': {merged: {applicationDeletionFlow: {defaultHandle: DELETION_HANDLE}}},
      },
      (config) => seen.push(config),
    );

    await deleteApplicationViaFlow(http, SERVER, APP_ID);

    expect(seen.some((request) => request.method === 'DELETE')).toBe(false);
    expect(seen.at(-1)?.url).toBe(`${SERVER}/flow/execute`);
  });

  it('should fall back to the native endpoint when no handle is configured', async () => {
    const seen: RecordedRequest[] = [];
    const http = makeHttp({'/applications/': {}, '/server-config/flow': {merged: {}}}, (config) => seen.push(config));

    await deleteApplicationViaFlow(http, SERVER, APP_ID);

    expect(seen.at(-1)).toMatchObject({method: 'DELETE', url: `${SERVER}/applications/${APP_ID}`});
  });

  // A configured handle naming a flow that no longer exists is not the same as none configured: the
  // deployment asked for flow-based revocation. Deleting through the native endpoint would strip the
  // application while leaving every token it issued valid, and report that as a success.
  it('should refuse rather than fall back when the configured handle resolves to nothing', async () => {
    const seen: RecordedRequest[] = [];
    const http = makeHttp(
      {
        '/applications/': {},
        '/flows': {flows: []},
        '/server-config/flow': {merged: {applicationDeletionFlow: {defaultHandle: DELETION_HANDLE}}},
      },
      (config) => seen.push(config),
    );

    await expect(deleteApplicationViaFlow(http, SERVER, APP_ID)).rejects.toThrow(DELETION_HANDLE);
    expect(seen.some((request) => request.method === 'DELETE')).toBe(false);
  });

  // A flow that exists but fails must never be downgraded to a native delete: that would remove the
  // application while leaving every token it issued valid.
  it('should not fall back when the flow itself fails', async () => {
    const seen: RecordedRequest[] = [];
    const http = makeHttp(
      {
        '/applications/': {},
        '/flow/execute': new Error('flow exploded'),
        '/flows': {flows: [{flowType: 'ADMINISTRATION', handle: DELETION_HANDLE, id: DELETION_FLOW_ID}]},
        '/server-config/flow': {merged: {applicationDeletionFlow: {defaultHandle: DELETION_HANDLE}}},
      },
      (config) => seen.push(config),
    );

    await expect(deleteApplicationViaFlow(http, SERVER, APP_ID)).rejects.toThrow('flow exploded');
    expect(seen.some((request) => request.method === 'DELETE')).toBe(false);
  });
});

describe('regenerateClientSecretViaFlow', () => {
  it('should return the secret the flow generated', async () => {
    const http = makeHttp({
      '/flow/execute': {data: {additionalData: {clientSecret: NEW_SECRET}}, flowStatus: 'COMPLETE'},
      '/flows': {flows: [{flowType: 'ADMINISTRATION', handle: ROTATION_HANDLE, id: ROTATION_FLOW_ID}]},
      '/server-config/flow': {merged: {clientSecretRegenerationFlow: {defaultHandle: ROTATION_HANDLE}}},
    });

    await expect(regenerateClientSecretViaFlow(http, SERVER, APP_ID)).resolves.toBe(NEW_SECRET);
  });

  // Null is the signal to fall back, and must mean "no flow" only. A flow that completed without a
  // secret rotated the credential, so falling back would rotate it a second time.
  it('should reject a completed flow that returned no secret', async () => {
    const http = makeHttp({
      '/flow/execute': {flowStatus: 'COMPLETE'},
      '/flows': {flows: [{flowType: 'ADMINISTRATION', handle: ROTATION_HANDLE, id: ROTATION_FLOW_ID}]},
      '/server-config/flow': {merged: {clientSecretRegenerationFlow: {defaultHandle: ROTATION_HANDLE}}},
    });

    await expect(regenerateClientSecretViaFlow(http, SERVER, APP_ID)).rejects.toThrow('without returning a secret');
  });

  it('should return null when no flow is configured, so the caller can fall back', async () => {
    const http = makeHttp({'/server-config/flow': {merged: {}}});

    await expect(regenerateClientSecretViaFlow(http, SERVER, APP_ID)).resolves.toBeNull();
  });

  // Rotating natively would mint a new secret while every token issued under the old one stayed
  // valid, which is the outcome configuring the flow was meant to prevent.
  it('should refuse rather than fall back when the configured handle resolves to nothing', async () => {
    const http = makeHttp({
      '/flows': {flows: []},
      '/server-config/flow': {merged: {clientSecretRegenerationFlow: {defaultHandle: ROTATION_HANDLE}}},
    });

    await expect(regenerateClientSecretViaFlow(http, SERVER, APP_ID)).rejects.toThrow(ROTATION_HANDLE);
  });
});
