// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import formatTimestamp from '../formatTimestamp';
import generateGatewayKey from '../generateGatewayKey';

describe('formatTimestamp', () => {
  it('renders a dash when there is no timestamp', () => {
    expect(formatTimestamp(undefined)).toBe('-');
  });

  it('formats a valid timestamp for the locale', () => {
    expect(formatTimestamp('2026-01-02T03:04:05Z')).toBe(new Date('2026-01-02T03:04:05Z').toLocaleString());
  });

  it('keeps a value that is not a date as it is', () => {
    expect(formatTimestamp('not-a-date')).toBe('not-a-date');
  });
});

describe('generateGatewayKey', () => {
  it('generates distinct url-safe keys from 32 random bytes', () => {
    const first = generateGatewayKey();
    const second = generateGatewayKey();

    expect(first).toMatch(/^[A-Za-z0-9_-]{43}$/);
    expect(first).not.toBe(second);
  });
});
