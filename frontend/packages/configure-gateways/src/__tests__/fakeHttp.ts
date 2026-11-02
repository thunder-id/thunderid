// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {vi} from 'vitest';

export const SERVER = 'https://localhost:8090';

export interface Call {
  url: string;
  method: string;
  data?: unknown;
}

/** A stand-in for the console's HTTP client, answering by `METHOD path` and recording every call. */
export interface FakeHttp {
  request: (call: Call) => Promise<unknown>;
  calls: Call[];
}

export function fakeHttp(routes: Record<string, unknown>): FakeHttp {
  const calls: Call[] = [];
  const request = vi.fn((call: Call): Promise<unknown> => {
    calls.push(call);
    const [route] = call.url.replace(SERVER, '').split('?');
    const key = `${call.method} ${route}`;
    const answer = routes[key];
    if (answer === undefined) {
      return Promise.reject(Object.assign(new Error(`Unexpected ${key}`), {response: {status: 404}}));
    }
    const data = typeof answer === 'function' ? (answer as (call: Call) => unknown)(call) : answer;
    return data instanceof Error ? Promise.reject(data) : Promise.resolve({data});
  });
  return {request, calls};
}
