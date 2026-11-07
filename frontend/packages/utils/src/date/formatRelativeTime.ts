// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

const UNITS: [Intl.RelativeTimeFormatUnit, number][] = [
  ['year', 365 * 24 * 3600],
  ['month', 30 * 24 * 3600],
  ['day', 24 * 3600],
  ['hour', 3600],
  ['minute', 60],
];

/**
 * How far a moment is from now, in its largest whole unit: "3 hours ago", "in 2 days", "now".
 *
 * @param date - The moment to describe.
 * @param locale - The locale to format in. Defaults to the runtime's locale.
 * @returns The relative time.
 */
export default function formatRelativeTime(date: Date, locale?: string): string {
  const seconds = Math.round((date.getTime() - Date.now()) / 1000);
  const format = new Intl.RelativeTimeFormat(locale, {numeric: 'auto'});
  for (const [unit, size] of UNITS) {
    if (Math.abs(seconds) >= size) return format.format(Math.round(seconds / size), unit);
  }
  return format.format(seconds, 'second');
}
