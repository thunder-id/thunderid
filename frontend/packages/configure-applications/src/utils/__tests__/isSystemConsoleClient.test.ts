// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import isSystemConsoleClient from '../isSystemConsoleClient';

describe('isSystemConsoleClient', () => {
  it.each<[string | undefined, string | undefined, boolean]>([
    ['CONSOLE', undefined, true],
    ['console', undefined, true],
    ['my-console', 'MY-CONSOLE', true],
    ['MY-CONSOLE', 'my-console', true],
    ['CONSOLE', 'my-console', false],
    ['other-app', undefined, false],
    [undefined, undefined, false],
    [undefined, 'my-console', false],
  ])('client %s with Console client %s is the Console: %s', (clientId, consoleClientId, expected) => {
    expect(isSystemConsoleClient(clientId, consoleClientId)).toBe(expected);
  });
});
