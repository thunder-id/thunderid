// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import generateHandle from '../generateHandle';

describe('generateHandle', () => {
  it('lowercases and joins words with a hyphen by default', () => {
    expect(generateHandle('Customer Accounts')).toBe('customer-accounts');
  });

  it('uses the given separator', () => {
    expect(generateHandle('Payments API', '_')).toBe('payments_api');
  });

  it('replaces runs of special characters with a single separator', () => {
    expect(generateHandle('My@Api#V2')).toBe('my-api-v2');
    expect(generateHandle('a  --  b')).toBe('a-b');
  });

  it('trims leading and trailing whitespace and separators', () => {
    expect(generateHandle('  hello world ')).toBe('hello-world');
    expect(generateHandle('--Hello--')).toBe('hello');
    expect(generateHandle('!!Hello!!')).toBe('hello');
  });

  it('keeps digits', () => {
    expect(generateHandle('Team 42')).toBe('team-42');
  });

  it('drops non ASCII letters', () => {
    expect(generateHandle('Café Users')).toBe('caf-users');
  });

  it('returns an empty string when nothing is left', () => {
    expect(generateHandle('')).toBe('');
    expect(generateHandle('@#$')).toBe('');
  });
});
