// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {JSX} from 'react';

export interface NpmIconProps {
  size?: number;
}

/** npm's logo, used to identify npm commands. */
export default function NpmIcon({size = 20}: NpmIconProps): JSX.Element {
  return (
    <svg width={size} height={size} viewBox="0 0 16 16" aria-hidden>
      <rect width="16" height="16" rx="2" fill="#CB3837" />
      <path fill="#FFFFFF" d="M3 3.5h10v9h-3V6.5H8v6H3z" />
    </svg>
  );
}
