// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

export const SERVER_URL = 'https://localhost:8090';

export interface HttpCall {
  url: string;
  method: string;
  data?: unknown;
}

type Handler = (call: HttpCall) => unknown;

/**
 * Builds an `http.request` stand-in that answers by `METHOD path`, so a test states the API it
 * expects rather than the order calls arrive in. A handler that throws rejects the request.
 */
export function createHttpRouter(routes: Record<string, unknown>): (call: HttpCall) => Promise<unknown> {
  return async (call: HttpCall): Promise<unknown> => {
    const path = call.url.replace(SERVER_URL, '');
    const route = routes[`${call.method} ${path}`] ?? routes[`${call.method} ${path.split('?')[0]}`];
    if (route === undefined) {
      throw new Error(`Unexpected request: ${call.method} ${path}`);
    }
    const data = typeof route === 'function' ? (route as Handler)(call) : route;
    return Promise.resolve({data});
  };
}

/**
 * An error shaped like the one the HTTP client rejects with for an API error envelope.
 */
export function apiError(status: number, code?: string): Error {
  return Object.assign(new Error('raw server text'), {
    response: {status, data: code ? {code, message: {key: 'x', defaultValue: 'raw server text'}} : undefined},
  });
}
