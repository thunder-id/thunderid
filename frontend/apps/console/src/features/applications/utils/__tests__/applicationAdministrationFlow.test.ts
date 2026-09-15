// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it, vi} from 'vitest';
import {deleteApplicationViaFlow, regenerateClientSecretViaFlow, type HttpLike} from '../applicationAdministrationFlow';

const SERVER = 'https://localhost:8090';
const APP_ID = '01a03248-56f6-75a8-a821-eb93cd60fc7b';
const DELETION_FLOW_ID = '01900000-0000-7000-8000-000000000079';
const ROTATION_FLOW_ID = '01900000-0000-7000-8000-00000000007a';
const DELETION_HANDLE = 'default-application-deletion-flow';
const ROTATION_HANDLE = 'default-secret-regeneration-flow';
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
    // The executor declares this input, so the key is part of the contract rather than a detail.
    expect(seen.at(-1)?.data).toMatchObject({inputs: {targetApplicationId: APP_ID}});
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
      '/server-config/flow': {merged: {secretRegenerationFlow: {defaultHandle: ROTATION_HANDLE}}},
    });

    await expect(regenerateClientSecretViaFlow(http, SERVER, APP_ID)).resolves.toBe(NEW_SECRET);
  });

  // Null is the signal to fall back, and must mean "no flow" only. A flow that completed without a
  // secret rotated the credential, so falling back would rotate it a second time.
  it('should reject a completed flow that returned no secret', async () => {
    const http = makeHttp({
      '/flow/execute': {flowStatus: 'COMPLETE'},
      '/flows': {flows: [{flowType: 'ADMINISTRATION', handle: ROTATION_HANDLE, id: ROTATION_FLOW_ID}]},
      '/server-config/flow': {merged: {secretRegenerationFlow: {defaultHandle: ROTATION_HANDLE}}},
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
      '/server-config/flow': {merged: {secretRegenerationFlow: {defaultHandle: ROTATION_HANDLE}}},
    });

    await expect(regenerateClientSecretViaFlow(http, SERVER, APP_ID)).rejects.toThrow(ROTATION_HANDLE);
  });
});
