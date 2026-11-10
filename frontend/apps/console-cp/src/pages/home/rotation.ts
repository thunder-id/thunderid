// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/** How many features the overview shows at once. */
export const FEATURES_SHOWN = 3;

/** How long each slot shows its feature before the next slot changes. */
export const ROTATE_MS = 6000;

/** How long a card takes to fade out, and the next to fade in. */
export const FADE_MS = 400;

/** What the slots show, and the features waiting their turn, the one hidden longest first. */
export interface Rotation {
  shown: string[];
  waiting: string[];
}

/** The slots filled from the first features, the rest waiting in order. */
export function startRotation(ids: string[], count: number = FEATURES_SHOWN): Rotation {
  return {shown: ids.slice(0, count), waiting: ids.slice(count)};
}

/**
 * Turns one slot to the feature that has waited longest; the one it replaces waits last. With nothing waiting,
 * every feature is already shown and nothing changes.
 */
export function rotate(rotation: Rotation, slot: number): Rotation {
  const [next, ...rest] = rotation.waiting;
  if (next === undefined || rotation.shown.length === 0) return rotation;
  const at = slot % rotation.shown.length;
  const shown = [...rotation.shown];
  const out = shown[at];
  shown[at] = next;
  return {shown, waiting: [...rest, out]};
}
