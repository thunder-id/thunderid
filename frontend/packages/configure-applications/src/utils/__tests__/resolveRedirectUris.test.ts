// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import resolveRedirectUris from '../resolveRedirectUris';

describe('resolveRedirectUris', () => {
  it('replaces the template URIs with the entered ones', () => {
    expect(resolveRedirectUris(['http://localhost:5173'], '', ['https://app.example.com/cb', ' '])).toEqual([
      'https://app.example.com/cb',
    ]);
  });

  it('keeps the callback URL ahead of the entered URIs, once', () => {
    expect(
      resolveRedirectUris(['http://localhost:5173'], 'https://app.example.com/cb', [
        'https://app.example.com/cb',
        'https://other.example.com/cb',
      ]),
    ).toEqual(['https://app.example.com/cb', 'https://other.example.com/cb']);
  });

  it('uses the callback URL, then the template URIs, when nothing is entered', () => {
    expect(resolveRedirectUris(['http://localhost:5173'], 'https://app.example.com/cb', [])).toEqual([
      'https://app.example.com/cb',
    ]);
    expect(resolveRedirectUris(['http://localhost:5173'], '', [''])).toEqual(['http://localhost:5173']);
  });
});
