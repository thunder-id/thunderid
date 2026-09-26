// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {AllowedOriginTypes, createRow} from '@thunderid/configure-settings';
import {describe, expect, it} from 'vitest';
import mergeCorsOrigins from '../mergeCorsOrigins';

const origin = (value: string) => createRow(AllowedOriginTypes.ORIGIN, value);
const regex = (value: string) => createRow(AllowedOriginTypes.REGEX, value);

describe('mergeCorsOrigins', () => {
  it('appends new valid origins to the existing writable list', () => {
    const result = mergeCorsOrigins(['https://existing.example.com'], [], [origin('https://new.example.com')]);

    expect(result).toEqual({allowedOrigins: ['https://existing.example.com', 'https://new.example.com']});
  });

  it('appends a regex addition as a {regex} entry', () => {
    const result = mergeCorsOrigins([], [], [regex('^https://.*\\.acme\\.io$')]);

    expect(result).toEqual({allowedOrigins: [{regex: '^https://.*\\.acme\\.io$'}]});
  });

  it('skips additions that already exist in the writable list', () => {
    const result = mergeCorsOrigins(['https://existing.example.com'], [], [origin('https://existing.example.com')]);

    expect(result).toEqual({allowedOrigins: ['https://existing.example.com']});
  });

  it('skips additions that already exist in the read-only list', () => {
    const result = mergeCorsOrigins([], ['https://readonly.example.com'], [origin('https://readonly.example.com')]);

    expect(result).toEqual({allowedOrigins: []});
  });

  it('normalizes origin additions before comparing and storing (trailing slash, casing)', () => {
    const result = mergeCorsOrigins(['https://existing.example.com'], [], [origin('HTTPS://EXISTING.example.com/')]);

    expect(result).toEqual({allowedOrigins: ['https://existing.example.com']});
  });

  it('does not treat a regex addition as a duplicate of a literal with the same text', () => {
    const result = mergeCorsOrigins(['https://app.example.com'], [], [regex('https://app.example.com')]);

    expect(result).toEqual({allowedOrigins: ['https://app.example.com', {regex: 'https://app.example.com'}]});
  });

  it('skips blank additions', () => {
    const result = mergeCorsOrigins([], [], [origin(''), origin('   '), regex('')]);

    expect(result).toEqual({allowedOrigins: []});
  });

  it('skips invalid additions as a backstop, since the wizard now blocks them up front', () => {
    const result = mergeCorsOrigins([], [], [origin('not-a-valid-origin'), origin('https://valid.example.com/path')]);

    expect(result).toEqual({allowedOrigins: []});
  });

  it('dedupes multiple identical additions in the same call', () => {
    const result = mergeCorsOrigins([], [], [origin('http://localhost:5173'), origin('http://localhost:5173')]);

    expect(result).toEqual({allowedOrigins: ['http://localhost:5173']});
  });

  it('leaves regex read-only/writable entries untouched and still compares against their text', () => {
    const result = mergeCorsOrigins([{regex: '^https://.*\\.example\\.com$'}], [], [origin('http://localhost:3000')]);

    expect(result).toEqual({
      allowedOrigins: [{regex: '^https://.*\\.example\\.com$'}, 'http://localhost:3000'],
    });
  });
});
