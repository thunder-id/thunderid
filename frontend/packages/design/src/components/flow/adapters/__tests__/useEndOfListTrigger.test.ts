// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {act, cleanup, renderHook} from '@testing-library/react';
import {afterEach, beforeEach, describe, it, expect, vi} from 'vitest';
import useEndOfListTrigger, {END_OF_LIST_MARGIN_PX, type EndOfListTriggerArgs} from '../useEndOfListTrigger';

type ObserverCallback = (entries: {isIntersecting: boolean}[]) => void;

class FakeIntersectionObserver {
  readonly observe = vi.fn();

  readonly disconnect = vi.fn();

  constructor(
    readonly callback: ObserverCallback,
    readonly options: IntersectionObserverInit,
  ) {
    observers.push(this);
  }
}

let observers: FakeIntersectionObserver[] = [];

const latestObserver = (): FakeIntersectionObserver => observers[observers.length - 1];

const setEndVisible = (isIntersecting: boolean): void => {
  act(() => latestObserver().callback([{isIntersecting}]));
};

const root = document.createElement('div');
const sentinel = document.createElement('div');

const argsWith = (overrides: Partial<EndOfListTriggerArgs> = {}): EndOfListTriggerArgs => ({
  enabled: true,
  itemCount: 10,
  loading: false,
  onLoadMore: vi.fn(),
  root,
  sentinel,
  ...overrides,
});

const render = (args: EndOfListTriggerArgs) =>
  renderHook((props: EndOfListTriggerArgs) => useEndOfListTrigger(props), {initialProps: args});

describe('useEndOfListTrigger', () => {
  beforeEach(() => {
    observers = [];
    vi.stubGlobal('IntersectionObserver', FakeIntersectionObserver);
  });

  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  describe('observing', () => {
    it('observes the end marker once the list is enabled', () => {
      render(argsWith());

      expect(observers).toHaveLength(1);
      expect(latestObserver().observe).toHaveBeenCalledWith(sentinel);
    });

    it('watches the list with a margin below it, so loading starts a little early', () => {
      render(argsWith());

      expect(latestObserver().options).toMatchObject({root, rootMargin: `0px 0px ${END_OF_LIST_MARGIN_PX}px 0px`});
    });

    it('observes nothing while disabled', () => {
      render(argsWith({enabled: false}));

      expect(observers).toHaveLength(0);
    });

    it('observes nothing before the list element exists', () => {
      render(argsWith({root: null}));

      expect(observers).toHaveLength(0);
    });

    it('observes nothing before the end marker exists', () => {
      render(argsWith({sentinel: null}));

      expect(observers).toHaveLength(0);
    });

    it('stops observing when it becomes disabled', () => {
      const {rerender} = render(argsWith());
      const {disconnect} = latestObserver();

      rerender(argsWith({enabled: false}));

      expect(disconnect).toHaveBeenCalledTimes(1);
    });

    it('stops observing when it unmounts', () => {
      const {unmount} = render(argsWith());
      const {disconnect} = latestObserver();

      unmount();

      expect(disconnect).toHaveBeenCalledTimes(1);
    });
  });

  describe('asking for more', () => {
    it('asks when the end becomes visible', () => {
      const onLoadMore = vi.fn();
      render(argsWith({onLoadMore}));

      setEndVisible(true);

      expect(onLoadMore).toHaveBeenCalledTimes(1);
    });

    it('does not ask while loading', () => {
      const onLoadMore = vi.fn();
      render(argsWith({loading: true, onLoadMore}));

      setEndVisible(true);

      expect(onLoadMore).not.toHaveBeenCalled();
    });

    it('asks once loading finishes while the end is still visible', () => {
      const onLoadMore = vi.fn();
      const {rerender} = render(argsWith({loading: true, onLoadMore}));
      setEndVisible(true);

      rerender(argsWith({loading: false, onLoadMore}));
      setEndVisible(true);

      expect(onLoadMore).toHaveBeenCalledTimes(1);
    });

    it('asks only once for the same list length, however often it is observed again', () => {
      const onLoadMore = vi.fn();
      const {rerender} = render(argsWith({onLoadMore}));
      setEndVisible(true);

      rerender(argsWith({loading: true, onLoadMore}));
      rerender(argsWith({loading: false, onLoadMore}));
      setEndVisible(true);

      expect(onLoadMore).toHaveBeenCalledTimes(1);
    });

    it('asks again after the list grew', () => {
      const onLoadMore = vi.fn();
      const {rerender} = render(argsWith({onLoadMore}));
      setEndVisible(true);

      rerender(argsWith({itemCount: 20, onLoadMore}));
      setEndVisible(true);

      expect(onLoadMore).toHaveBeenCalledTimes(2);
    });

    it('asks again after the end was scrolled out of view and back', () => {
      const onLoadMore = vi.fn();
      render(argsWith({onLoadMore}));

      setEndVisible(true);
      setEndVisible(false);
      setEndVisible(true);

      expect(onLoadMore).toHaveBeenCalledTimes(2);
    });

    it('asks again when the list is opened again', () => {
      const onLoadMore = vi.fn();
      const {rerender} = render(argsWith({onLoadMore}));
      setEndVisible(true);

      rerender(argsWith({onLoadMore, root: document.createElement('div')}));
      setEndVisible(true);

      expect(onLoadMore).toHaveBeenCalledTimes(2);
    });

    it('calls the callback from the latest render', () => {
      const first = vi.fn();
      const second = vi.fn();
      const {rerender} = render(argsWith({onLoadMore: first}));

      rerender(argsWith({onLoadMore: second}));
      setEndVisible(true);

      expect(first).not.toHaveBeenCalled();
      expect(second).toHaveBeenCalledTimes(1);
    });

    it('does nothing when there is no callback', () => {
      render(argsWith({onLoadMore: undefined}));

      expect(() => setEndVisible(true)).not.toThrow();
    });
  });
});
