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

/**
 * A gateway's store listing as the server answers it: narrowed by `names` and a `name sw` filter, then
 * paged by `limit` and `offset`, with how many match in all.
 */
export function storeListing(collection: 'variables' | 'secrets', values: {name: string}[]): (call: Call) => unknown {
  return (call: Call): unknown => {
    const params = new URL(call.url).searchParams;
    let matched = [...values];
    const names = params.get('names');
    if (names) {
      const wanted = new Set(names.split(','));
      matched = matched.filter((value) => wanted.has(value.name));
    }
    const prefix = /^name sw "(.*)"$/.exec(params.get('filter') ?? '')?.[1];
    if (prefix) matched = matched.filter((value) => value.name.startsWith(prefix));
    const limit = Number(params.get('limit') ?? 30);
    const offset = Number(params.get('offset') ?? 0);
    return {totalResults: matched.length, [collection]: matched.slice(offset, offset + limit)};
  };
}

/** The query a call was made with, as a plain object. */
export function queryOf(call: Call): Record<string, string> {
  return Object.fromEntries(new URL(call.url).searchParams.entries());
}
