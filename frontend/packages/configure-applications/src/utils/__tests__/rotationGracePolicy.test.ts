// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it, vi} from 'vitest';
import {fetchRotationGracePolicy, ROTATION_GRACE_UNAVAILABLE, type HttpLike} from '../rotationGracePolicy';

const SERVER = 'https://localhost:8090';
const SECTION = '/server-config/oauth';

/**
 * Builds an http double answering the section read with a fixed payload, or rejecting.
 */
function makeHttp(value: unknown): HttpLike {
  return {
    request: vi.fn((config: unknown): Promise<{data?: unknown}> => {
      const {url} = config as {url: string};
      if (!url.includes(SECTION)) {
        return Promise.reject(new Error(`unexpected request: ${url}`));
      }
      if (value instanceof Error) {
        return Promise.reject(value);
      }
      return Promise.resolve({data: value});
    }),
  };
}

describe('fetchRotationGracePolicy', () => {
  it('should read the enabled policy and its ceiling from the merged layer', async () => {
    const http = makeHttp({merged: {refreshToken: {graceEnabled: true, graceCeilingSeconds: 30}}});

    await expect(fetchRotationGracePolicy(http, SERVER)).resolves.toEqual({enabled: true, ceilingSeconds: 30});
  });

  // Only the merged layer is the effective configuration. Reading the envelope directly, or the
  // declarative layer alone, would report a policy the server is not applying.
  it('should ignore layers other than merged', async () => {
    const http = makeHttp({
      readOnly: {refreshToken: {graceEnabled: true, graceCeilingSeconds: 300}},
      writable: {refreshToken: {graceEnabled: false, graceCeilingSeconds: 0}},
      merged: {refreshToken: {graceEnabled: false, graceCeilingSeconds: 0}},
    });

    await expect(fetchRotationGracePolicy(http, SERVER)).resolves.toEqual(ROTATION_GRACE_UNAVAILABLE);
  });

  // The server grants no window while the feature is off, whatever ceiling is stored, so the
  // console must not offer a field against a stale ceiling.
  it('should report unavailable when the feature is disabled despite a stored ceiling', async () => {
    const http = makeHttp({merged: {refreshToken: {graceEnabled: false, graceCeilingSeconds: 30}}});

    await expect(fetchRotationGracePolicy(http, SERVER)).resolves.toEqual(ROTATION_GRACE_UNAVAILABLE);
  });

  it('should report unavailable when enabled with no ceiling', async () => {
    const http = makeHttp({merged: {refreshToken: {graceEnabled: true, graceCeilingSeconds: 0}}});

    await expect(fetchRotationGracePolicy(http, SERVER)).resolves.toEqual(ROTATION_GRACE_UNAVAILABLE);
  });

  // The oauth section holds one block per sub-domain. A section that carries other blocks but no
  // refresh token block has not enabled the feature.
  it('should report unavailable when the section has no refresh token block', async () => {
    const http = makeHttp({merged: {otherBlock: {enabled: true}}});

    await expect(fetchRotationGracePolicy(http, SERVER)).resolves.toEqual(ROTATION_GRACE_UNAVAILABLE);
  });

  it('should report unavailable for an unset section', async () => {
    const http = makeHttp({});

    await expect(fetchRotationGracePolicy(http, SERVER)).resolves.toEqual(ROTATION_GRACE_UNAVAILABLE);
  });

  // The section is admin scoped, so a console user without that scope legitimately cannot read it.
  // Hiding the field is the right answer; failing the token settings page is not.
  it('should report unavailable rather than propagating a read failure', async () => {
    const http = makeHttp(new Error('forbidden'));

    await expect(fetchRotationGracePolicy(http, SERVER)).resolves.toEqual(ROTATION_GRACE_UNAVAILABLE);
  });
});
