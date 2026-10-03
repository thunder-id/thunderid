// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import isLoopbackRedirectUri from '../isLoopbackRedirectUri';

describe('isLoopbackRedirectUri', () => {
  it.each([
    ['http://localhost/callback', true],
    ['http://127.0.0.1:33418/', true],
    ['http://[::1]/callback', true],
    ['https://localhost/callback', false],
    ['http://192.168.1.10/callback', false],
    ['https://client.example.com/callback', false],
    ['not a url', false],
  ])('%s is loopback: %s', (uri, expected) => {
    expect(isLoopbackRedirectUri(uri)).toBe(expected);
  });
});
