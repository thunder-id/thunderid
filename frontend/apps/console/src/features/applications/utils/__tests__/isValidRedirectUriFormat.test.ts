// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import isValidRedirectUriFormat, {isValidRedirectUriTransport} from '../isValidRedirectUriFormat';

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

describe('isValidRedirectUriTransport', () => {
  it.each([
    'https://example.com/callback',
    'myapp:/callback',
    'http://localhost:3000/callback',
    'http://LOCALHOST:3000/callback',
    'http://127.0.0.1:3000/callback',
    'http://[::1]:3000/callback',
  ])('accepts %s', (uri) => {
    expect(isValidRedirectUriTransport(uri)).toBe(true);
  });

  it.each([
    'http://example.com/callback',
    'http://192.168.1.10/callback',
    'http://127.1/callback',
    'http://127.0.0.2/callback',
    'http://localhost.example.com/callback',
  ])('rejects %s', (uri) => {
    expect(isValidRedirectUriTransport(uri)).toBe(false);
  });
});
