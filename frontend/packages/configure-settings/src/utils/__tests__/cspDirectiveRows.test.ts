// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import {CSP_DEFAULT_SCOPE, type CspScopeRow} from '../../models/cspDirectiveRow';
import type {CspConfigResponse, CspValue} from '../../models/responses';
import {
  buildCspPayload,
  buildScopeRows,
  createCspRow,
  cspScopes,
  parseSources,
  rowSources,
  scopeDirectives,
  toCspDraft,
} from '../cspDirectiveRows';

const readOnly: CspValue = {
  directives: {'frame-src': ["'self'"]},
  paths: [{location: '/console/', directives: {'font-src': ["'self'", 'data:'], 'img-src': ["'self'", 'data:']}}],
};

const find = (rows: CspScopeRow[], directive: string): CspScopeRow | undefined =>
  rows.find((row) => row.directive === directive);

describe('cspDirectiveRows', () => {
  it('creates rows with unique ids', () => {
    expect(createCspRow('img-src').id).not.toBe(createCspRow('img-src').id);
  });

  it('splits text into sources on any whitespace', () => {
    expect(parseSources("  'self'\thttps://a.example.com   data: ")).toEqual([
      "'self'",
      'https://a.example.com',
      'data:',
    ]);
    expect(parseSources('   ')).toEqual([]);
  });

  it('reads the directives of the default scope and of a path scope', () => {
    expect(scopeDirectives(readOnly, CSP_DEFAULT_SCOPE)).toEqual({'frame-src': ["'self'"]});
    expect(scopeDirectives(readOnly, '/console/')).toEqual({
      'font-src': ["'self'", 'data:'],
      'img-src': ["'self'", 'data:'],
    });
    expect(scopeDirectives(readOnly, '/gate/')).toBeUndefined();
  });

  it('lists the default scope, then each path location once, read-only first', () => {
    const data: CspConfigResponse = {
      readOnly: {
        paths: [
          {location: '/console/', directives: {}},
          {location: '/gate/', directives: {}},
        ],
      },
      writable: {
        paths: [
          {location: '/gate/', directives: {}},
          {location: '/custom/', directives: {}},
        ],
      },
      merged: {},
    };
    expect(cspScopes(data)).toEqual([CSP_DEFAULT_SCOPE, '/console/', '/gate/', '/custom/']);
    expect(cspScopes(undefined)).toEqual([CSP_DEFAULT_SCOPE]);
  });

  describe('buildScopeRows', () => {
    it('shows the baseline and the read-only directives for All pages', () => {
      const rows = buildScopeRows(readOnly, {}, CSP_DEFAULT_SCOPE);

      expect(rows.map((row) => row.directive)).toEqual([
        'default-src',
        'script-src',
        'style-src',
        'img-src',
        'connect-src',
        'frame-ancestors',
        'base-uri',
        'form-action',
        'frame-src',
      ]);
      expect(find(rows, 'img-src')?.base).toEqual(["'self'"]);
      expect(find(rows, 'frame-src')?.base).toEqual(["'self'"]);
    });

    it('applies an override in place of the default value', () => {
      const row = find(
        buildScopeRows(
          readOnly,
          {[CSP_DEFAULT_SCOPE]: [createCspRow('img-src', ['https://cdn.example.com'])]},
          CSP_DEFAULT_SCOPE,
        ),
        'img-src',
      )!;
      expect(rowSources(row)).toEqual(['https://cdn.example.com']);
      expect(row.draftId).toBeDefined();
    });

    it('starts a path from All pages, with its read-only directives replacing inherited ones', () => {
      const rows = buildScopeRows(
        readOnly,
        {[CSP_DEFAULT_SCOPE]: [createCspRow('connect-src', ["'self'", 'https://api.example.com'])]},
        '/console/',
      );

      expect(find(rows, 'connect-src')?.base).toEqual(["'self'", 'https://api.example.com']);
      expect(find(rows, 'img-src')?.base).toEqual(["'self'", 'data:']);
      expect(find(rows, 'frame-src')?.base).toEqual(["'self'"]);
      expect(find(rows, 'font-src')).toBeDefined();
    });

    it('lists a directive with no default value as a row of its own', () => {
      const row = find(buildScopeRows(readOnly, {'/console/': [createCspRow('media-src')]}, '/console/'), 'media-src')!;
      expect(row.base).toEqual([]);
      expect(rowSources(row)).toEqual([]);
    });
  });

  it('builds the draft from the saved writable layer', () => {
    const draft = toCspDraft({
      readOnly,
      writable: {
        directives: {'img-src': ['https:']},
        paths: [{location: '/console/', directives: {'font-src': ["'self'", 'data:', 'https:']}}],
      },
      merged: {},
    });

    expect(draft[CSP_DEFAULT_SCOPE].map((row) => [row.directive, row.sources])).toEqual([['img-src', ['https:']]]);
    expect(draft['/console/'].map((row) => [row.directive, row.sources])).toEqual([
      ['font-src', ["'self'", 'data:', 'https:']],
    ]);
  });

  describe('buildCspPayload', () => {
    it('saves each set directive and keeps fields the UI does not edit', () => {
      const payload = buildCspPayload(
        {reportUri: '/csp-report', reportOnly: true, directives: {'img-src': ["'self'"]}},
        {
          [CSP_DEFAULT_SCOPE]: [createCspRow('frame-src', ["'self'"])],
          '/console/': [createCspRow('connect-src', ["'self'", 'https://api.example.com'])],
          '/gate/': [createCspRow('media-src')],
        },
        undefined,
      );
      expect(payload).toEqual({
        reportUri: '/csp-report',
        reportOnly: true,
        directives: {'frame-src': ["'self'"]},
        paths: [{location: '/console/', directives: {'connect-src': ["'self'", 'https://api.example.com']}}],
      });
    });

    it('uses the edited report-only choice over the saved one', () => {
      expect(buildCspPayload({reportOnly: true}, {}, false)).toEqual({reportOnly: false});
    });

    it('omits report-only, directives, and paths when none are set', () => {
      expect(
        buildCspPayload(
          {directives: {'img-src': ["'self'"]}, paths: [{location: '/console/', directives: {'img-src': ["'self'"]}}]},
          {[CSP_DEFAULT_SCOPE]: [], '/console/': []},
          undefined,
        ),
      ).toEqual({});
      expect(buildCspPayload(undefined, {}, undefined)).toEqual({});
    });
  });
});
