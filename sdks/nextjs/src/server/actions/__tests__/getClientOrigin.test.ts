/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

// src/server/actions/__tests__/getClientOrigin.test.ts
import {headers} from 'next/headers';
import {describe, it, expect, vi, beforeEach, afterEach, type Mock} from 'vitest';

// Import SUT and mocked dep
import getClientOrigin from '../getClientOrigin';

// Mock next/headers BEFORE importing the SUT
vi.mock('next/headers', () => ({
  headers: vi.fn(),
}));

// Helper: build a Headers-like object. get() should be case-insensitive.
interface HLike {
  get: (name: string) => string | null;
}
const makeHeaders = (map: Record<string, string | null | undefined>): HLike => {
  const normalized: Record<string, string | null | undefined> = {};
  Object.entries(map).forEach(([k, val]: [string, string | null | undefined]) => {
    normalized[k.toLowerCase()] = val;
  });
  return {
    get: (name: string): string | null => {
      const v: string | null | undefined = normalized[name.toLowerCase()];
      return v == null ? null : v; // emulate real Headers.get(): string | null
    },
  };
};

describe('getClientOrigin', () => {
  beforeEach(() => {
    vi.resetAllMocks();
    // by default return empty headers
    (headers as unknown as Mock).mockResolvedValue(makeHeaders({}));
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('should return https origin when x-forwarded-proto is https and host is present', async () => {
    (headers as unknown as Mock).mockResolvedValue(makeHeaders({host: 'example.com', 'x-forwarded-proto': 'https'}));

    const origin: string = await getClientOrigin();

    expect(headers).toHaveBeenCalledTimes(1);
    expect(origin).toBe('https://example.com');
  });

  it('should fall back to http when x-forwarded-proto is missing', async () => {
    (headers as unknown as Mock).mockResolvedValue(
      makeHeaders({host: 'svc.internal' /* x-forwarded-proto: missing */}),
    );

    const origin: string = await getClientOrigin();

    expect(origin).toBe('http://svc.internal');
  });

  it('should return "protocol://null" when host is missing', async () => {
    // host header absent -> get('host') returns null -> interpolates as "null"
    (headers as unknown as Mock).mockResolvedValue(makeHeaders({'x-forwarded-proto': 'https'}));

    const origin: string = await getClientOrigin();

    expect(origin).toBe('https://null');
  });

  it('should propagate errors when headers() rejects', async () => {
    (headers as unknown as Mock).mockRejectedValue(new Error('headers not available'));

    await expect(getClientOrigin()).rejects.toThrow('headers not available');
  });
});
