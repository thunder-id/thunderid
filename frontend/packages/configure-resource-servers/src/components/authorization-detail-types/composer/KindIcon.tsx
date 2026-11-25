// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Banknote, Braces, Calendar, CalendarClock, Hash, List, ToggleLeft, Type} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import type {PaletteItem} from '../../../utils/schema-composer';

const ICONS = {
  string: Type,
  number: Hash,
  date: Calendar,
  dateTime: CalendarClock,
  boolean: ToggleLeft,
  array: List,
  object: Braces,
  money: Banknote,
} as const;

export interface KindIconProps {
  kind: PaletteItem;
  size?: number;
}

/** The icon of a field kind, as the palette and the field tree show it. */
export default function KindIcon({kind, size = 16}: KindIconProps): JSX.Element {
  const Icon = ICONS[kind];
  return <Icon size={size} aria-hidden />;
}
