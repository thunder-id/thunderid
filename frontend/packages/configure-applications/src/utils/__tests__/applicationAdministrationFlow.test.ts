// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it, vi} from 'vitest';
import {
  deleteApplicationNatively,
  regenerateClientSecretNatively,
  type HttpLike,
} from '../applicationAdministrationFlow';

const SERVER = 'https://localhost:8090';
const APP_ID = '01a03248-56f6-75a8-a821-eb93cd60fc7b';
const NEW_SECRET = 'KdJL7PrfDTh-XwT8kGtg1nMydcDQg_L5an25vV52Ej8';

/**
 * The request shape the module issues.
 */
interface RecordedRequest {
  url: string;
  method: string;
  data?: unknown;
}

/**
 * Builds an http double that answers every request with response, recording each one.
 */
function makeHttp(response: Promise<{data?: unknown}>): {http: HttpLike; requests: RecordedRequest[]} {
  const requests: RecordedRequest[] = [];
  return {
    http: {
      request: vi.fn((config: unknown): Promise<{data?: unknown}> => {
        requests.push(config as RecordedRequest);
        return response;
      }),
    },
    requests,
  };
}

describe('regenerateClientSecretNatively', () => {
  it('should ask the server for a new secret and return it', async () => {
    const {http, requests} = makeHttp(Promise.resolve({data: {clientSecret: NEW_SECRET}}));

    await expect(regenerateClientSecretNatively(http, SERVER, APP_ID)).resolves.toEqual({
      clientSecret: NEW_SECRET,
      revocationIncomplete: false,
    });

    expect(requests).toHaveLength(1);
    expect(requests[0].method).toBe('POST');
    expect(requests[0].url).toBe(`${SERVER}/applications/${APP_ID}/client-secret/regenerate`);
    expect(requests[0].data).toBeUndefined();
  });

  // The server replaced the secret, so a response without it leaves the user with nothing to copy.
  it('should reject a response that carries no secret', async () => {
    const {http} = makeHttp(Promise.resolve({data: {}}));

    await expect(regenerateClientSecretNatively(http, SERVER, APP_ID)).rejects.toThrow('without returning a secret');
  });

  it('should propagate a refused request', async () => {
    const {http} = makeHttp(Promise.reject(new Error('Application has no client secret')));

    await expect(regenerateClientSecretNatively(http, SERVER, APP_ID)).rejects.toThrow(
      'Application has no client secret',
    );
  });
});

describe('deleteApplicationNatively', () => {
  it('should delete through the application endpoint', async () => {
    const {http, requests} = makeHttp(Promise.resolve({}));

    await deleteApplicationNatively(http, SERVER, APP_ID);

    expect(requests).toHaveLength(1);
    expect(requests[0].method).toBe('DELETE');
    expect(requests[0].url).toBe(`${SERVER}/applications/${APP_ID}`);
  });
});
