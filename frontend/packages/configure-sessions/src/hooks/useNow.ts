// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useCallback, useSyncExternalStore} from 'react';

interface Ticker {
  now: number;
  listeners: Set<() => void>;
  timer?: ReturnType<typeof setInterval>;
}

// One timer per interval, shared by every component showing a time that moves at that pace.
const tickers = new Map<number, Ticker>();

const tickerFor = (intervalMs: number): Ticker => {
  let ticker = tickers.get(intervalMs);
  if (!ticker) {
    ticker = {now: Date.now(), listeners: new Set()};
    tickers.set(intervalMs, ticker);
  }
  return ticker;
};

/**
 * The current time, refreshed every `intervalMs`, so times shown relative to now keep moving. Every
 * component asking for the same interval shares one timer, which stops when the last one unmounts.
 *
 * @param intervalMs - How often to refresh, in milliseconds
 * @returns The current time in milliseconds since the epoch
 */
export default function useNow(intervalMs: number): number {
  const subscribe = useCallback(
    (onTick: () => void): (() => void) => {
      const ticker = tickerFor(intervalMs);
      ticker.listeners.add(onTick);
      if (!ticker.timer) {
        ticker.now = Date.now();
        ticker.timer = setInterval(() => {
          ticker.now = Date.now();
          ticker.listeners.forEach((listener) => listener());
        }, intervalMs);
      }
      return (): void => {
        ticker.listeners.delete(onTick);
        if (ticker.listeners.size === 0) {
          clearInterval(ticker.timer);
          ticker.timer = undefined;
        }
      };
    },
    [intervalMs],
  );

  return useSyncExternalStore(subscribe, () => tickerFor(intervalMs).now);
}
