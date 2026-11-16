// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {JSX} from 'react';

export interface PnpmIconProps {
  size?: number;
}

/** pnpm's logo, used to identify pnpm commands. */
export default function PnpmIcon({size = 20}: PnpmIconProps): JSX.Element {
  return (
    <svg width={size} height={size} viewBox="0 0 16 16" aria-hidden>
      <g fill="#F9AD00">
        <rect x="0" y="0" width="4.6" height="4.6" />
        <rect x="5.7" y="0" width="4.6" height="4.6" />
        <rect x="11.4" y="0" width="4.6" height="4.6" />
        <rect x="11.4" y="5.7" width="4.6" height="4.6" />
      </g>
      <g fill="#9AA3B5">
        <rect x="5.7" y="5.7" width="4.6" height="4.6" />
        <rect x="0" y="11.4" width="4.6" height="4.6" />
        <rect x="5.7" y="11.4" width="4.6" height="4.6" />
        <rect x="11.4" y="11.4" width="4.6" height="4.6" />
      </g>
    </svg>
  );
}
