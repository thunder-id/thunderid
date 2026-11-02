// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {JSX, createElement} from 'react';
import getEcosystemIcon from '../iconRegistry';

/**
 * Renders an entry's logo by registry name.
 *
 * The component is chosen from data, so it is instantiated with `createElement`
 * rather than bound to a local and used as JSX: a capitalized local assigned
 * during render reads as a freshly declared component type, which would remount
 * the subtree on every render.
 */
export default function EntryIcon({name, size}: {name: string; size: number}): JSX.Element | null {
  const icon = getEcosystemIcon(name);
  return icon ? createElement(icon, {size}) : null;
}
