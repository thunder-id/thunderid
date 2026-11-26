// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * The units a duration can be entered in.
 *
 * @public
 */
export type DurationUnit = 'seconds' | 'minutes' | 'hours' | 'days';

/**
 * Seconds in one of each unit, largest first so {@link toDurationParts} picks the largest unit that
 * divides a value exactly.
 *
 * @public
 */
export const DurationUnitSeconds: Record<DurationUnit, number> = {
  days: 86400,
  hours: 3600,
  minutes: 60,
  seconds: 1,
};

const UNITS_LARGEST_FIRST: DurationUnit[] = ['days', 'hours', 'minutes', 'seconds'];

/**
 * A duration as a number and the unit it reads best in.
 *
 * @public
 */
export interface DurationParts {
  value: number;
  unit: DurationUnit;
}

/**
 * Splits a number of seconds into the largest unit that divides it exactly.
 *
 * Exactly, not approximately: 90 minutes stays 5400 seconds shown as 90 minutes rather than becoming
 * "1 hour" and losing half of itself on the next save. A configuration value the form rewrites is
 * worse than one it shows awkwardly.
 *
 * @param seconds - The stored value
 * @returns The value and the unit to show it in
 *
 * @public
 */
export function toDurationParts(seconds: number): DurationParts {
  if (!Number.isFinite(seconds) || seconds <= 0) {
    return {value: 0, unit: 'seconds'};
  }
  const unit: DurationUnit =
    UNITS_LARGEST_FIRST.find((candidate) => seconds % DurationUnitSeconds[candidate] === 0) ?? 'seconds';
  return {value: seconds / DurationUnitSeconds[unit], unit};
}

/**
 * Converts a value and unit back to seconds.
 *
 * @param value - The number entered
 * @param unit - The unit selected
 * @returns The duration in seconds
 *
 * @public
 */
export function toSeconds(value: number, unit: DurationUnit): number {
  if (!Number.isFinite(value) || value < 0) {
    return 0;
  }
  return Math.round(value) * DurationUnitSeconds[unit];
}
