// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import isValidRedirectUriFormat from '../isValidRedirectUriFormat';

describe('isValidRedirectUriFormat', () => {
  it('accepts well-formed URIs', () => {
    expect(isValidRedirectUriFormat('https://example.com/callback')).toBe(true);
    expect(isValidRedirectUriFormat('http://localhost:3000/cb')).toBe(true);
  });

  it('accepts host wildcards', () => {
    expect(isValidRedirectUriFormat('https://*.example.com/callback')).toBe(true);
    expect(isValidRedirectUriFormat('https://app-*.example.com/cb')).toBe(true);
  });

  it('accepts path wildcards', () => {
    expect(isValidRedirectUriFormat('https://example.com/callback/*')).toBe(true);
  });

  it('rejects empty or whitespace-only input', () => {
    expect(isValidRedirectUriFormat('')).toBe(false);
    expect(isValidRedirectUriFormat('   ')).toBe(false);
  });

  it('rejects malformed URIs', () => {
    expect(isValidRedirectUriFormat('not a uri')).toBe(false);
    expect(isValidRedirectUriFormat('://missing-scheme')).toBe(false);
  });
});
