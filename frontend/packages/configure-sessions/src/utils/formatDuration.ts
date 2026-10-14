// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

const DURATION_UNITS: [string, number][] = [
  ['day', 24 * 60 * 60],
  ['hour', 60 * 60],
  ['minute', 60],
];

/**
 * Formats a length of time to the minute, such as "7h 59m", leaving out the leading units that are zero.
 * Seconds are dropped.
 *
 * @param totalSeconds - The length of time in whole seconds, at least zero
 * @param locale - The locale to format in
 * @returns The formatted length of time
 */
export default function formatDuration(totalSeconds: number, locale: string): string {
  let seconds = totalSeconds;
  const parts: string[] = [];
  for (const [unit, unitSeconds] of DURATION_UNITS) {
    const amount = Math.floor(seconds / unitSeconds);
    seconds -= amount * unitSeconds;
    if (amount > 0 || parts.length > 0 || unit === 'minute') {
      parts.push(new Intl.NumberFormat(locale, {style: 'unit', unit, unitDisplay: 'narrow'}).format(amount));
    }
  }
  return parts.join(' ');
}
