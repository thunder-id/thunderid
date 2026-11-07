// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box} from '@wso2/oxygen-ui';
import type {JSX, ReactNode} from 'react';

/**
 * The width of the composer from which the side panel sits beside the fields panel, leaving the card about
 * as wide as the consent screen lays it out. Measured on the composer rather than the viewport, since the
 * Console's sidebar takes a different share of the viewport when expanded or collapsed.
 */
const SIDE_BESIDE_MIN_WIDTH = 880;

export interface ComposerLayoutProps {
  main: ReactNode;
  /** The side panel; the main panel takes the full width without one. */
  side?: ReactNode;
}

/** Lays out the fields panel and the side panel next to each other, stacking them on narrow screens. */
export default function ComposerLayout({main, side = undefined}: ComposerLayoutProps): JSX.Element {
  const beside = `@container (min-width: ${SIDE_BESIDE_MIN_WIDTH}px)`;
  return (
    <Box sx={{containerType: 'inline-size'}}>
      <Box
        sx={{
          display: 'grid',
          gap: 3,
          alignItems: 'start',
          gridTemplateColumns: 'minmax(0, 1fr)',
          ...(side && {[beside]: {gridTemplateColumns: 'minmax(0, 1fr) 400px'}}),
        }}
      >
        {main}
        {side && <Box sx={{minWidth: 0, [beside]: {position: 'sticky', top: 24}}}>{side}</Box>}
      </Box>
    </Box>
  );
}
