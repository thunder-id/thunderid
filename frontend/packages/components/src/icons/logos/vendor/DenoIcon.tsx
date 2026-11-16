// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {JSX} from 'react';

export interface DenoIconProps {
  size?: number;
}

/** Deno's logo, used to identify Deno commands. */
export default function DenoIcon({size = 20}: DenoIconProps): JSX.Element {
  return (
    <svg width={size} height={size} viewBox="0 0 16 16" aria-hidden>
      <circle cx="8" cy="8" r="7.25" fill="none" stroke="currentColor" strokeWidth="1.5" />
      <path
        d="M5 12.5c.5-2.5 1.5-4 3.5-4.5 1.6-.4 2.5-1.4 2.5-2.6 0-1.3-1.1-2.2-2.7-2.2"
        stroke="currentColor"
        strokeWidth="1.3"
        fill="none"
        strokeLinecap="round"
      />
      <circle cx="8.9" cy="5.2" r="0.7" fill="currentColor" />
    </svg>
  );
}
