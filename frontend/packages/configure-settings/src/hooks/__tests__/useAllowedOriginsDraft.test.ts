// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {act} from '@testing-library/react';
import {renderHook} from '@thunderid/test-utils';
import {describe, it, expect} from 'vitest';
import {AllowedOriginTypes} from '../../models/allowedOriginRow';
import type {AllowedOrigin, CorsConfigResponse} from '../../models/responses';
import useAllowedOriginsDraft from '../useAllowedOriginsDraft';

function makeData(readOnly: AllowedOrigin[], writable: AllowedOrigin[]): CorsConfigResponse {
  return {
    readOnly: {allowedOrigins: readOnly},
    writable: {allowedOrigins: writable},
    merged: {allowedOrigins: []},
  };
}

/** The typed shape of each row, with the generated id dropped so assertions stay readable. */
function shapeOf(draft: {type: string; value: string}[]): {type: string; value: string}[] {
  return draft.map(({type, value}) => ({type, value}));
}

describe('useAllowedOriginsDraft', () => {
  it('loads each writable entry with the type its wire shape declares', () => {
    const {result} = renderHook(() =>
      useAllowedOriginsDraft(makeData([], ['https://app.acme.com', {regex: '^https://[a-z]+\\.acme\\.io$'}])),
    );
    expect(shapeOf(result.current.draft)).toEqual([
      {type: AllowedOriginTypes.ORIGIN, value: 'https://app.acme.com'},
      {type: AllowedOriginTypes.REGEX, value: '^https://[a-z]+\\.acme\\.io$'},
    ]);
    expect(result.current.dirty).toBe(false);
    expect(result.current.hasErrors).toBe(false);
  });

  it('adding an empty row is not dirty; typing a value makes it dirty', () => {
    const {result} = renderHook(() => useAllowedOriginsDraft(makeData([], ['https://app.acme.com'])));
    act(() => result.current.addRow());
    expect(result.current.dirty).toBe(false);
    act(() => result.current.changeRow(result.current.draft[1].id, 'https://new.example.com'));
    expect(result.current.dirty).toBe(true);
  });

  it('adds a literal origin row, not a regex one', () => {
    const {result} = renderHook(() => useAllowedOriginsDraft(makeData([], [])));
    act(() => result.current.addRow());
    expect(result.current.draft[0].type).toBe(AllowedOriginTypes.ORIGIN);
  });

  it('removing a row marks the draft dirty', () => {
    const {result} = renderHook(() =>
      useAllowedOriginsDraft(makeData([], ['https://app.acme.com', 'https://other.acme.com'])),
    );
    act(() => result.current.removeRow(result.current.draft[0].id));
    expect(result.current.dirty).toBe(true);
    expect(shapeOf(result.current.draft)).toEqual([{type: AllowedOriginTypes.ORIGIN, value: 'https://other.acme.com'}]);
  });

  it('reset clears local edits and reverts to the saved value', () => {
    const {result} = renderHook(() => useAllowedOriginsDraft(makeData([], ['https://app.acme.com'])));
    act(() => result.current.changeRow(result.current.draft[0].id, 'https://changed.example.com'));
    expect(result.current.dirty).toBe(true);
    act(() => result.current.reset());
    expect(result.current.dirty).toBe(false);
    expect(shapeOf(result.current.draft)).toEqual([{type: AllowedOriginTypes.ORIGIN, value: 'https://app.acme.com'}]);
  });

  it('normalizes an origin row on blur (lowercase + trailing slash), preserving an explicit port', () => {
    const {result} = renderHook(() => useAllowedOriginsDraft(makeData([], ['https://app.acme.com'])));
    act(() => result.current.changeRow(result.current.draft[0].id, 'HTTPS://Example.COM:443/'));
    act(() => result.current.blurRow(result.current.draft[0].id));
    expect(result.current.draft[0].value).toBe('https://example.com:443');
    expect(result.current.hasErrors).toBe(false);
  });

  it('leaves a regex row untouched on blur, even when its text parses as an origin', () => {
    const {result} = renderHook(() => useAllowedOriginsDraft(makeData([], [{regex: 'https://APP.example.com/'}])));
    act(() => result.current.blurRow(result.current.draft[0].id));
    // Casing is significant to the matcher and `/` is an ordinary regex character, so neither may be rewritten.
    expect(result.current.draft[0].value).toBe('https://APP.example.com/');
  });

  it('flags an origin row whose value is not a valid origin, instead of treating it as a pattern', () => {
    const {result} = renderHook(() => useAllowedOriginsDraft(makeData([], ['https://example.com/path'])));
    let ok = true;
    act(() => {
      ok = result.current.validateAll();
    });
    expect(ok).toBe(false);
    expect(result.current.errors[result.current.draft[0].id]).toBeTruthy();
  });

  it('flags a regex row whose pattern does not compile', () => {
    const {result} = renderHook(() => useAllowedOriginsDraft(makeData([], [{regex: '(bad'}])));
    let ok = true;
    act(() => {
      ok = result.current.validateAll();
    });
    expect(ok).toBe(false);
    expect(result.current.errors[result.current.draft[0].id]).toBeTruthy();
  });

  it('accepts both a valid origin and a valid regex row', () => {
    const {result} = renderHook(() =>
      useAllowedOriginsDraft(makeData([], ['https://ok.example.com', {regex: '^https://.*\\.ok\\.io$'}])),
    );
    let ok = false;
    act(() => {
      ok = result.current.validateAll();
    });
    expect(ok).toBe(true);
    expect(result.current.hasErrors).toBe(false);
  });

  it('flags duplicates within the draft and clears them when a counterpart row is removed', () => {
    const {result} = renderHook(() =>
      useAllowedOriginsDraft(makeData([], ['https://dup.example.com', 'https://dup.example.com'])),
    );
    const [first, second] = result.current.draft;
    act(() => {
      result.current.validateAll();
    });
    expect(result.current.errors[first.id]).toBeTruthy();
    expect(result.current.errors[second.id]).toBeTruthy();

    act(() => result.current.removeRow(second.id));
    expect(result.current.errors[first.id]).toBeUndefined();
  });

  it('clears a duplicate error when the counterpart is edited to a unique value', () => {
    const {result} = renderHook(() =>
      useAllowedOriginsDraft(makeData([], ['https://dup.example.com', 'https://dup.example.com'])),
    );
    const [first, second] = result.current.draft;
    act(() => {
      result.current.validateAll();
    });
    expect(result.current.errors[first.id]).toBeTruthy();

    act(() => result.current.changeRow(second.id, 'https://unique.example.com'));
    expect(result.current.errors[first.id]).toBeUndefined();
  });

  it('treats a default port as distinct from the port-less origin (no false duplicate)', () => {
    const {result} = renderHook(() =>
      useAllowedOriginsDraft(makeData([], ['https://example.com', 'https://example.com:443'])),
    );
    act(() => {
      result.current.validateAll();
    });
    expect(result.current.hasErrors).toBe(false);
  });

  it('flags a custom origin that duplicates a read-only origin', () => {
    const {result} = renderHook(() =>
      useAllowedOriginsDraft(makeData(['https://console.example.com'], ['https://console.example.com'])),
    );
    act(() => {
      result.current.validateAll();
    });
    expect(result.current.errors[result.current.draft[0].id]).toBeTruthy();
  });

  it('flags a custom regex that duplicates a read-only regex', () => {
    const {result} = renderHook(() =>
      useAllowedOriginsDraft(
        makeData([{regex: '^https://[a-z]+\\.acme\\.io$'}], [{regex: '^https://[a-z]+\\.acme\\.io$'}]),
      ),
    );
    act(() => {
      result.current.validateAll();
    });
    expect(result.current.errors[result.current.draft[0].id]).toBeTruthy();
  });

  it('does not treat a read-only regex as a duplicate of a writable origin with the same text', () => {
    const {result} = renderHook(() =>
      useAllowedOriginsDraft(makeData([{regex: 'https://shared.example.com'}], ['https://shared.example.com'])),
    );
    act(() => {
      result.current.validateAll();
    });
    expect(result.current.hasErrors).toBe(false);
  });

  describe('changeRowType', () => {
    it('keeps the text the row already holds', () => {
      const {result} = renderHook(() => useAllowedOriginsDraft(makeData([], ['https://app.example.com'])));
      act(() => result.current.changeRowType(result.current.draft[0].id, AllowedOriginTypes.REGEX));
      expect(shapeOf(result.current.draft)).toEqual([
        {type: AllowedOriginTypes.REGEX, value: 'https://app.example.com'},
      ]);
    });

    it('re-canonicalizes for the new type, so a pattern-cased value becomes a valid origin', () => {
      const {result} = renderHook(() => useAllowedOriginsDraft(makeData([], [{regex: 'HTTPS://X.COM/'}])));
      act(() => result.current.changeRowType(result.current.draft[0].id, AllowedOriginTypes.ORIGIN));
      expect(result.current.draft[0].value).toBe('https://x.com');
      expect(result.current.hasErrors).toBe(false);
    });

    it('validates immediately, without waiting for a blur', () => {
      const {result} = renderHook(() => useAllowedOriginsDraft(makeData([], [{regex: '^https://x\\.io$'}])));
      act(() => result.current.changeRowType(result.current.draft[0].id, AllowedOriginTypes.ORIGIN));
      expect(result.current.errors[result.current.draft[0].id]).toBeTruthy();
    });

    it('marks the draft dirty even when only the type changed', () => {
      const {result} = renderHook(() => useAllowedOriginsDraft(makeData([], ['https://app.example.com'])));
      act(() => result.current.changeRowType(result.current.draft[0].id, AllowedOriginTypes.REGEX));
      expect(result.current.dirty).toBe(true);
    });

    it('retypes only the named row, leaving its siblings as they are', () => {
      const {result} = renderHook(() =>
        useAllowedOriginsDraft(makeData([], ['HTTPS://First.example.com/', 'https://second.example.com'])),
      );
      act(() => result.current.changeRowType(result.current.draft[1].id, AllowedOriginTypes.REGEX));
      // The untouched row keeps the text it was loaded with, rather than being re-canonicalized too.
      expect(shapeOf(result.current.draft)).toEqual([
        {type: AllowedOriginTypes.ORIGIN, value: 'HTTPS://First.example.com/'},
        {type: AllowedOriginTypes.REGEX, value: 'https://second.example.com'},
      ]);
    });
  });

  describe('unanchored regex warning', () => {
    it('warns without blocking the save', () => {
      const {result} = renderHook(() => useAllowedOriginsDraft(makeData([], [{regex: 'acme\\.io'}])));
      let ok = false;
      act(() => {
        ok = result.current.validateAll();
      });
      expect(ok).toBe(true);
      expect(result.current.hasErrors).toBe(false);
      expect(result.current.warnings[result.current.draft[0].id]).toBeTruthy();
    });

    it('is absent for an anchored pattern', () => {
      const {result} = renderHook(() => useAllowedOriginsDraft(makeData([], [{regex: '^https://x\\.io$'}])));
      act(() => {
        result.current.validateAll();
      });
      expect(result.current.warnings[result.current.draft[0].id]).toBeUndefined();
    });
  });

  describe('buildPayload', () => {
    it('emits the wire shape each row declares, dropping empty rows', () => {
      const {result} = renderHook(() =>
        useAllowedOriginsDraft(makeData([], ['https://app.example.com', {regex: '^https://[a-z]+\\.example\\.com$'}])),
      );
      // Add a trailing empty row that Save must drop.
      act(() => result.current.addRow());

      expect(result.current.buildPayload()).toEqual({
        allowedOrigins: ['https://app.example.com', {regex: '^https://[a-z]+\\.example\\.com$'}],
      });
    });

    it('preserves an explicit default port in the saved string entry', () => {
      const {result} = renderHook(() => useAllowedOriginsDraft(makeData([], ['https://example.com:443'])));
      expect(result.current.buildPayload()).toEqual({allowedOrigins: ['https://example.com:443']});
    });

    it('round-trips a loaded regex entry back to a {regex} entry', () => {
      const {result} = renderHook(() => useAllowedOriginsDraft(makeData([], [{regex: '^https://x\\.io$'}])));
      expect(result.current.draft[0].type).toBe(AllowedOriginTypes.REGEX);
      expect(result.current.buildPayload()).toEqual({allowedOrigins: [{regex: '^https://x\\.io$'}]});
    });

    it('keeps a regex whose pattern is itself a valid origin as a {regex} entry', () => {
      // A no-op save used to rewrite this to a literal, narrowing a substring match to an exact one.
      const {result} = renderHook(() => useAllowedOriginsDraft(makeData([], [{regex: 'https://example.com'}])));
      expect(result.current.buildPayload()).toEqual({allowedOrigins: [{regex: 'https://example.com'}]});
    });

    it('keeps the "null" literal as a string entry', () => {
      const {result} = renderHook(() => useAllowedOriginsDraft(makeData([], ['null'])));
      expect(result.current.buildPayload()).toEqual({allowedOrigins: ['null']});
    });
  });
});
