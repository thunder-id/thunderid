// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

const RELATIVE_UNITS: [Intl.RelativeTimeFormatUnit, number][] = [
  ['year', 365 * 24 * 60 * 60],
  ['month', 30 * 24 * 60 * 60],
  ['day', 24 * 60 * 60],
  ['hour', 60 * 60],
  ['minute', 60],
];

/**
 * Formats a timestamp relative to now, such as "5 minutes ago". Anything under a minute reads as now.
 *
 * @param value - An ISO 8601 timestamp
 * @param locale - The locale to format in
 * @param now - The current time, for tests
 * @returns The relative time
 */
export default function formatRelativeTime(value: string, locale: string, now: Date = new Date()): string {
  const formatter = new Intl.RelativeTimeFormat(locale, {numeric: 'auto'});
  const seconds = Math.round((new Date(value).getTime() - now.getTime()) / 1000);
  for (const [unit, unitSeconds] of RELATIVE_UNITS) {
    if (Math.abs(seconds) >= unitSeconds) {
      return formatter.format(Math.trunc(seconds / unitSeconds), unit);
    }
  }
  return formatter.format(0, 'second');
}
