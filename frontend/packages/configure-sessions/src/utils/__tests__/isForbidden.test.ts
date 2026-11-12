// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import isForbidden from '../isForbidden';

describe('isForbidden', () => {
  it('is true for a 403 response', () => {
    expect(isForbidden({response: {status: 403}})).toBe(true);
  });

  it('is false for another status or an error without a response', () => {
    expect(isForbidden({response: {status: 500}})).toBe(false);
    expect(isForbidden(new Error('network'))).toBe(false);
    expect(isForbidden(undefined)).toBe(false);
  });
});
