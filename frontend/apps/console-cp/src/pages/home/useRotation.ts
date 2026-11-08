// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMediaQuery} from '@wso2/oxygen-ui';
import {useEffect, useState, type FocusEvent} from 'react';
import {FADE_MS, ROTATE_MS, rotate, startRotation, type Rotation} from './rotation';

/** What a rotating row shows, and how its container holds it still while it is pointed at or focused. */
export interface RotationState {
  /** The ids the slots show, in slot order. */
  shown: string[];
  /** Whether the person asked for no motion; the row then neither rotates nor fades. */
  still: boolean;
  /** Spread on the row's container: it holds still while the pointer or focus is on it. */
  holdProps: {
    onPointerEnter: () => void;
    onPointerLeave: () => void;
    onFocus: () => void;
    onBlur: (event: FocusEvent<HTMLElement>) => void;
  };
}

/**
 * Shows `count` of the ids at once and turns one slot at a time to the id that waited longest. It holds still
 * while the pointer or focus is on the row, while the page is hidden, and when the person asked for no motion.
 */
export default function useRotation(ids: string[], count: number): RotationState {
  const still = useMediaQuery('(prefers-reduced-motion: reduce)');
  const key = ids.join(' ');
  const [rotation, setRotation] = useState<Rotation>(() => startRotation(ids, count));
  // The ids change only as what the admin may see is read; the slots then start again from the first.
  const [rotationKey, setRotationKey] = useState(key);
  if (rotationKey !== key) {
    setRotationKey(key);
    setRotation(startRotation(ids, count));
  }

  const [paused, setPaused] = useState(false);
  const [hidden, setHidden] = useState(() => typeof document !== 'undefined' && document.hidden);
  useEffect(() => {
    const onVisibility = (): void => setHidden(document.hidden);
    document.addEventListener('visibilitychange', onVisibility);
    return () => document.removeEventListener('visibilitychange', onVisibility);
  }, []);

  const canRotate = !still && !paused && !hidden && rotation.waiting.length > 0;
  useEffect(() => {
    if (!canRotate) return undefined;
    let slot = 0;
    const timer = setInterval(() => {
      setRotation((current: Rotation) => rotate(current, slot));
      slot = (slot + 1) % count;
    }, ROTATE_MS);
    return () => clearInterval(timer);
  }, [canRotate, key, count]);

  return {
    shown: rotation.shown,
    still,
    holdProps: {
      onPointerEnter: () => setPaused(true),
      onPointerLeave: () => setPaused(false),
      onFocus: () => setPaused(true),
      onBlur: (event: FocusEvent<HTMLElement>) => {
        if (!event.currentTarget.contains(event.relatedTarget as Node | null)) setPaused(false);
      },
    },
  };
}

/**
 * A slot's fade: while its id has changed, what it still shows fades out; once swapped, the next fades in.
 * Returns the id to show and the style that fades it.
 */
export function useSlotFade(
  id: string,
  still: boolean,
): {heldId: string; fade: {opacity: number; transform: string; transition: string}} {
  const [heldId, setHeldId] = useState(id);
  const changing = !still && heldId !== id;
  useEffect(() => {
    if (!changing) return undefined;
    const timer = setTimeout(() => setHeldId(id), FADE_MS);
    return () => clearTimeout(timer);
  }, [changing, id]);
  return {
    heldId: still ? id : heldId,
    fade: {
      opacity: changing ? 0 : 1,
      transform: changing ? 'translateY(4px)' : 'none',
      transition: still ? 'none' : `opacity ${FADE_MS}ms ease, transform ${FADE_MS}ms ease`,
    },
  };
}
