// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

const UNITS: [Intl.RelativeTimeFormatUnit, number][] = [
  ['year', 365 * 24 * 3600],
  ['month', 30 * 24 * 3600],
  ['week', 7 * 24 * 3600],
  ['day', 24 * 3600],
  ['hour', 3600],
  ['minute', 60],
];

/**
 * How long ago an API timestamp was, such as "2 hours ago" or "yesterday", in the given locale. Empty
 * when there is no time, or Go's zero time, which a gateway declared in a file reports.
 */
export default function relativeTime(value: string | undefined, now: number = Date.now(), locale?: string): string {
  if (!value) return '';
  const date = new Date(value);
  if (Number.isNaN(date.getTime()) || date.getUTCFullYear() <= 1) return '';
  const seconds = Math.round((date.getTime() - now) / 1000);
  const format = new Intl.RelativeTimeFormat(locale, {numeric: 'auto'});
  for (const [unit, size] of UNITS) {
    if (Math.abs(seconds) >= size) return format.format(Math.round(seconds / size), unit);
  }
  return format.format(0, 'second');
}
