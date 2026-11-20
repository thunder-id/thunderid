// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {JSX} from 'react';

export interface BunIconProps {
  size?: number;
}

/** Bun's logo, used to identify Bun commands. */
export default function BunIcon({size = 20}: BunIconProps): JSX.Element {
  return (
    <svg width={size} height={size} viewBox="0 0 16 16" aria-hidden>
      <ellipse cx="8" cy="9" rx="7.2" ry="5.8" fill="#FBF0DF" stroke="#3B2A1A" strokeWidth="0.8" />
      <circle cx="6" cy="9" r="0.8" fill="#3B2A1A" />
      <circle cx="10" cy="9" r="0.8" fill="#3B2A1A" />
      <path d="M7 10.6c.6.5 1.4.5 2 0" stroke="#3B2A1A" strokeWidth="0.7" fill="none" strokeLinecap="round" />
    </svg>
  );
}
