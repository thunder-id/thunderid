// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {act, renderHook} from '@testing-library/react';
import {describe, it, expect} from 'vitest';
import type {SessionConfigResponse} from '../../models/session';
import useSessionTimeoutsDraft, {isSessionDeclarative} from '../useSessionTimeoutsDraft';

/** Idle left to its two-hour default; the absolute timeout set here to twelve hours. */
const data: SessionConfigResponse = {
  readOnly: {},
  writable: {absoluteTimeoutSeconds: 43200},
  merged: {idleTimeoutSeconds: 7200, absoluteTimeoutSeconds: 43200},
};

describe('useSessionTimeoutsDraft', () => {
  it('shows the values in effect and which of them are set here', () => {
    const {result} = renderHook(() => useSessionTimeoutsDraft(data));

    expect(result.current.values).toEqual({idleTimeoutSeconds: 7200, absoluteTimeoutSeconds: 43200});
    expect(result.current.custom).toEqual({idleTimeoutSeconds: false, absoluteTimeoutSeconds: true});
    expect(result.current.dirty).toBe(false);
    expect(result.current.errors).toEqual({});
  });

  // The PUT replaces the writable layer, so a save carries what earlier saves set.
  it('sets a field on top of the saved values', () => {
    const {result} = renderHook(() => useSessionTimeoutsDraft(data));

    act(() => result.current.change('idleTimeoutSeconds', 3600));

    expect(result.current.values.idleTimeoutSeconds).toBe(3600);
    expect(result.current.custom.idleTimeoutSeconds).toBe(true);
    expect(result.current.dirty).toBe(true);
    expect(result.current.buildPayload()).toEqual({absoluteTimeoutSeconds: 43200, idleTimeoutSeconds: 3600});
  });

  // A field left out of the PUT returns to its default, which is known only once the save returns
  // the value in effect, so the field shows none until then and is not checked.
  it('resets a field to its default by leaving it out of the save', () => {
    const {result} = renderHook(() => useSessionTimeoutsDraft(data));

    act(() => result.current.resetToDefault('absoluteTimeoutSeconds'));

    expect(result.current.values.absoluteTimeoutSeconds).toBe(0);
    expect(result.current.custom.absoluteTimeoutSeconds).toBe(false);
    expect(result.current.dirty).toBe(true);
    expect(result.current.errors).toEqual({});
    expect(result.current.buildPayload()).toEqual({});
  });

  it('is not dirty once a change is put back', () => {
    const {result} = renderHook(() => useSessionTimeoutsDraft(data));

    act(() => result.current.change('absoluteTimeoutSeconds', 3600 * 24));
    act(() => result.current.change('absoluteTimeoutSeconds', 43200));

    expect(result.current.dirty).toBe(false);
  });

  it('discards the draft on reset', () => {
    const {result} = renderHook(() => useSessionTimeoutsDraft(data));

    act(() => result.current.change('idleTimeoutSeconds', 3600));
    act(() => result.current.reset());

    expect(result.current.dirty).toBe(false);
    expect(result.current.values.idleTimeoutSeconds).toBe(7200);
    expect(result.current.buildPayload()).toEqual(data.writable);
  });

  it('requires whole numbers greater than zero', () => {
    const {result} = renderHook(() => useSessionTimeoutsDraft(data));

    for (const refused of [0, -60, 90.5]) {
      act(() => result.current.change('idleTimeoutSeconds', refused));
      expect(result.current.errors.idleTimeoutSeconds).toBe('Enter a whole number greater than zero.');
      expect(result.current.hasErrors).toBe(true);
    }
    act(() => result.current.change('idleTimeoutSeconds', 60));
    expect(result.current.errors).toEqual({});
  });

  // The message names both fields, so it goes where the operator just typed.
  it('refuses an idle timeout longer than the absolute timeout on the field changed last', () => {
    const {result} = renderHook(() => useSessionTimeoutsDraft(data));
    const message = "The idle timeout can't be longer than the absolute timeout.";

    act(() => result.current.change('idleTimeoutSeconds', 5400));
    expect(result.current.errors).toEqual({});

    act(() => result.current.change('absoluteTimeoutSeconds', 3600));
    expect(result.current.errors).toEqual({absoluteTimeoutSeconds: message});

    act(() => result.current.change('idleTimeoutSeconds', 7200));
    expect(result.current.errors).toEqual({idleTimeoutSeconds: message});

    act(() => result.current.change('idleTimeoutSeconds', 3600));
    expect(result.current.errors).toEqual({});
  });

  // As on the server, an absolute timeout set on its own also cuts the idle timeout in effect, so it is
  // not compared with the default idle timeout.
  it('accepts an absolute timeout shorter than the default idle timeout', () => {
    const {result} = renderHook(() =>
      useSessionTimeoutsDraft({
        ...data,
        writable: {},
        merged: {idleTimeoutSeconds: 7200, absoluteTimeoutSeconds: 28800},
      }),
    );

    act(() => result.current.change('absoluteTimeoutSeconds', 3600));

    expect(result.current.errors).toEqual({});
    expect(result.current.buildPayload()).toEqual({absoluteTimeoutSeconds: 3600});
  });

  // A field changed and changed back leaves no edit, so a later read shows its value.
  it('forgets an edit changed back to the saved value', () => {
    const {result, rerender} = renderHook(({section}) => useSessionTimeoutsDraft(section), {
      initialProps: {section: data},
    });

    act(() => result.current.change('absoluteTimeoutSeconds', 86400));
    act(() => result.current.change('absoluteTimeoutSeconds', 43200));
    rerender({
      section: {
        ...data,
        writable: {absoluteTimeoutSeconds: 36000},
        merged: {idleTimeoutSeconds: 7200, absoluteTimeoutSeconds: 36000},
      },
    });

    expect(result.current.dirty).toBe(false);
    expect(result.current.values.absoluteTimeoutSeconds).toBe(36000);
  });

  // The server refuses every write once a declarative resource sets a timeout, so the draft is dropped
  // and the values in effect show again.
  it('drops the draft when a later read finds a declarative resource setting a timeout', () => {
    const {result, rerender} = renderHook(({section}) => useSessionTimeoutsDraft(section), {
      initialProps: {section: data},
    });

    act(() => result.current.change('idleTimeoutSeconds', 3600));
    rerender({section: {...data, readOnly: {absoluteTimeoutSeconds: 43200}, writable: {}}});

    expect(result.current.dirty).toBe(false);
    expect(result.current.values).toEqual({idleTimeoutSeconds: 7200, absoluteTimeoutSeconds: 43200});
  });

  // While a declarative resource sets the section, it is what is set here; the writable layer is ignored.
  it('reads a declarative resource as the value set here', () => {
    const {result} = renderHook(() =>
      useSessionTimeoutsDraft({
        ...data,
        readOnly: {idleTimeoutSeconds: 3600},
        writable: {absoluteTimeoutSeconds: 43200},
        merged: {idleTimeoutSeconds: 3600, absoluteTimeoutSeconds: 28800},
      }),
    );

    expect(result.current.values).toEqual({idleTimeoutSeconds: 3600, absoluteTimeoutSeconds: 28800});
    expect(result.current.custom).toEqual({idleTimeoutSeconds: true, absoluteTimeoutSeconds: false});
  });

  it('shows nothing set while the section loads', () => {
    const {result} = renderHook(() => useSessionTimeoutsDraft(undefined));

    expect(result.current.custom).toEqual({idleTimeoutSeconds: false, absoluteTimeoutSeconds: false});
    expect(result.current.dirty).toBe(false);
  });
});

describe('isSessionDeclarative', () => {
  it('is true when a declarative resource sets either timeout', () => {
    expect(isSessionDeclarative({...data, readOnly: {idleTimeoutSeconds: 3600}})).toBe(true);
    expect(isSessionDeclarative({...data, readOnly: {absoluteTimeoutSeconds: 43200}})).toBe(true);
  });

  it('is false when no declarative resource sets a timeout, or while loading', () => {
    expect(isSessionDeclarative(data)).toBe(false);
    expect(isSessionDeclarative(undefined)).toBe(false);
  });
});
