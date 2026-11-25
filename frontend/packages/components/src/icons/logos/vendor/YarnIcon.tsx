// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {JSX} from 'react';

export interface YarnIconProps {
  size?: number;
}

/** Yarn's logo, used to identify Yarn commands. */
export default function YarnIcon({size = 20}: YarnIconProps): JSX.Element {
  return (
    <svg width={size} height={size} viewBox="0 0 16 16" aria-hidden>
      <circle cx="8" cy="8" r="8" fill="#2C8EBB" />
      <path
        d="M5 4.5c1.4 1.6 2.4 2.9 3 4.4m0 0c.6-1.5 1.6-2.8 3-4.4M8 8.9V12"
        stroke="#FFFFFF"
        strokeWidth="1.5"
        fill="none"
        strokeLinecap="round"
      />
    </svg>
  );
}
