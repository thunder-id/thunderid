// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {act} from '@testing-library/react';
import {renderHook} from '@thunderid/test-utils';
import {describe, it, expect} from 'vitest';
import {CSP_DEFAULT_SCOPE, type CspScopeRow} from '../../models/cspDirectiveRow';
import type {CspConfigResponse} from '../../models/responses';
import {rowSources} from '../../utils/cspDirectiveRows';
import useCspDraft from '../useCspDraft';

function makeData(overrides?: Partial<CspConfigResponse>): CspConfigResponse {
  return {
    readOnly: {reportOnly: false, paths: [{location: '/console/', directives: {'font-src': ["'self'", 'data:']}}]},
    writable: {reportUri: '/csp-report', directives: {'frame-src': ["'self'"]}},
    merged: {reportOnly: false},
    ...overrides,
  };
}

const find = (rows: CspScopeRow[], directive: string): CspScopeRow => rows.find((row) => row.directive === directive)!;

describe('useCspDraft', () => {
  it('builds the scopes and rows, and starts clean', () => {
    const {result} = renderHook(() => useCspDraft(makeData()));

    expect(result.current.scopes).toEqual([CSP_DEFAULT_SCOPE, '/console/']);
    expect(rowSources(find(result.current.rowsFor(CSP_DEFAULT_SCOPE), 'frame-src'))).toEqual(["'self'"]);
    expect(rowSources(find(result.current.rowsFor('/console/'), 'font-src'))).toEqual(["'self'", 'data:']);
    expect(result.current.reportOnly).toBe(false);
    expect(result.current.dirty).toBe(false);
  });

  it('treats an unset report-only value as report-only', () => {
    const {result} = renderHook(() => useCspDraft(makeData({merged: {}})));
    expect(result.current.reportOnly).toBe(true);
  });

  it('marks the draft dirty when report-only changes, and clean again when it is changed back', () => {
    const {result} = renderHook(() => useCspDraft(makeData()));

    act(() => result.current.setReportOnly(true));
    expect(result.current.dirty).toBe(true);
    expect(result.current.buildPayload().reportOnly).toBe(true);

    act(() => result.current.setReportOnly(false));
    expect(result.current.dirty).toBe(false);
  });

  it('overrides a baseline directive, and resets it to its default', () => {
    const {result} = renderHook(() => useCspDraft(makeData()));

    act(() => result.current.setSources(CSP_DEFAULT_SCOPE, 'img-src', ['https://cdn.example.com']));
    const row = find(result.current.rowsFor(CSP_DEFAULT_SCOPE), 'img-src');
    expect(rowSources(row)).toEqual(['https://cdn.example.com']);
    expect(result.current.buildPayload().directives).toEqual({
      'frame-src': ["'self'"],
      'img-src': ['https://cdn.example.com'],
    });

    act(() => result.current.removeRow(CSP_DEFAULT_SCOPE, row.draftId!));
    expect(rowSources(find(result.current.rowsFor(CSP_DEFAULT_SCOPE), 'img-src'))).toEqual(["'self'"]);
    expect(result.current.dirty).toBe(false);
  });

  it("replaces 'none' when a source is added to it", () => {
    const {result} = renderHook(() => useCspDraft(makeData()));

    act(() => result.current.setSources(CSP_DEFAULT_SCOPE, 'default-src', ["'none'", "'self'"]));

    expect(rowSources(find(result.current.rowsFor(CSP_DEFAULT_SCOPE), 'default-src'))).toEqual(["'self'"]);
  });

  it('extends a read-only directive on a path, saving its existing sources too', () => {
    const {result} = renderHook(() => useCspDraft(makeData()));

    act(() => result.current.setSources('/console/', 'font-src', ["'self'", 'data:', 'https://fonts.example.com']));

    expect(result.current.buildPayload()).toEqual({
      reportUri: '/csp-report',
      directives: {'frame-src': ["'self'"]},
      paths: [{location: '/console/', directives: {'font-src': ["'self'", 'data:', 'https://fonts.example.com']}}],
    });
  });

  it('adds a directive the scope does not have yet, without making the draft dirty', () => {
    const {result} = renderHook(() => useCspDraft(makeData()));

    expect(result.current.availableDirectives('/console/')).not.toContain('font-src');
    act(() => result.current.addRow('/console/'));

    const added = result.current.rowsFor('/console/').find((row) => row.base.length === 0)!;
    expect(added.directive).toBe('script-src-elem');
    expect(result.current.dirty).toBe(false);

    act(() => result.current.changeDirective('/console/', added.draftId!, 'media-src'));
    act(() => result.current.setSources('/console/', 'media-src', ["'self'"]));
    expect(result.current.dirty).toBe(true);
    expect(result.current.buildPayload().paths).toEqual([
      {location: '/console/', directives: {'media-src': ["'self'"]}},
    ]);
  });

  it('reports errors for the directives the admin set, and clears them once fixed', () => {
    const {result} = renderHook(() => useCspDraft(makeData()));

    act(() => result.current.setSources(CSP_DEFAULT_SCOPE, 'img-src', []));
    const imgId = find(result.current.rowsFor(CSP_DEFAULT_SCOPE), 'img-src').draftId!;
    expect(result.current.errors[imgId]).toBe('Add at least one source, or reset the directive to its default.');
    expect(result.current.hasErrors).toBe(true);

    act(() => result.current.setSources(CSP_DEFAULT_SCOPE, 'img-src', ["'self'", "'unsafe-inline'"]));
    expect(result.current.hasErrors).toBe(false);
    expect(result.current.errors[imgId]).toBeUndefined();
  });

  it('blocks emptying a saved directive that has no default value', () => {
    const {result} = renderHook(() => useCspDraft(makeData()));
    const frameId = find(result.current.rowsFor(CSP_DEFAULT_SCOPE), 'frame-src').draftId!;

    act(() => result.current.setSources(CSP_DEFAULT_SCOPE, 'frame-src', []));

    expect(result.current.errors[frameId]).toBe('Add at least one source, or remove the directive.');
    expect(result.current.dirty).toBe(true);

    act(() => result.current.removeRow(CSP_DEFAULT_SCOPE, frameId));
    expect(result.current.hasErrors).toBe(false);
    expect(result.current.buildPayload().directives).toBeUndefined();
  });

  it('reverts every edit on reset', () => {
    const {result} = renderHook(() => useCspDraft(makeData()));

    act(() => result.current.setReportOnly(true));
    act(() => result.current.setSources(CSP_DEFAULT_SCOPE, 'frame-src', ['https:']));
    expect(result.current.dirty).toBe(true);

    act(() => result.current.reset());
    expect(result.current.dirty).toBe(false);
    expect(result.current.reportOnly).toBe(false);
    expect(rowSources(find(result.current.rowsFor(CSP_DEFAULT_SCOPE), 'frame-src'))).toEqual(["'self'"]);
  });

  it('edits and renames one of several added directives', () => {
    const {result} = renderHook(() => useCspDraft(makeData()));

    act(() => result.current.addRow('/console/'));
    act(() => result.current.addRow('/console/'));
    const [first, second] = result.current.rowsFor('/console/').filter((row) => row.base.length === 0);
    act(() => result.current.changeDirective('/console/', second.draftId!, 'media-src'));
    act(() => result.current.setSources('/console/', 'media-src', ["'self'"]));

    const rows = result.current.rowsFor('/console/').filter((row) => row.base.length === 0);
    expect(rows.map((row) => [row.directive, row.override])).toEqual([
      [first.directive, []],
      ['media-src', ["'self'"]],
    ]);
  });

  it('does not add a directive once the scope has every one', () => {
    const {result} = renderHook(() => useCspDraft(makeData()));

    while (result.current.availableDirectives(CSP_DEFAULT_SCOPE).length > 0) {
      act(() => result.current.addRow(CSP_DEFAULT_SCOPE));
    }
    const count = result.current.rowsFor(CSP_DEFAULT_SCOPE).length;
    act(() => result.current.addRow(CSP_DEFAULT_SCOPE));

    expect(result.current.rowsFor(CSP_DEFAULT_SCOPE)).toHaveLength(count);
  });

  it('ignores a rename or removal of a row that does not exist', () => {
    const {result} = renderHook(() => useCspDraft(makeData()));

    act(() => result.current.changeDirective(CSP_DEFAULT_SCOPE, 'missing', 'media-src'));
    act(() => result.current.removeRow(CSP_DEFAULT_SCOPE, 'missing'));
    act(() => result.current.changeDirective('/unknown/', 'missing', 'media-src'));
    act(() => result.current.removeRow('/unknown/', 'missing'));

    expect(result.current.dirty).toBe(false);
    expect(result.current.buildPayload()).toEqual({reportUri: '/csp-report', directives: {'frame-src': ["'self'"]}});
  });
});
