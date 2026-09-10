// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Typography, Box} from '@wso2/oxygen-ui';
import {type ReactElement} from 'react';
import type {Element as FlowElement} from '@/features/flows/models/elements';

/**
 * QR code element type with properties at top level.
 */
export type QrCodeElement = FlowElement & {
  source?: string;
};

/**
 * Props interface for QrCodeAdapter
 */
export interface QrCodeAdapterPropsInterface {
  resource?: FlowElement;
}

/** Module count of a version 1 QR symbol, which the preview imitates. */
const MODULE_COUNT = 21;

/** Top-left corner of each of the three finder patterns. */
const FINDER_ORIGINS: readonly (readonly [number, number])[] = [
  [0, 0],
  [MODULE_COUNT - 7, 0],
  [0, MODULE_COUNT - 7],
];

/**
 * A finder pattern is a 7x7 square: a filled outer ring, a one module gap, and a
 * filled 3x3 core. Ring distance 2 is the gap, everything else within the square is dark.
 */
const isFinderModule = (row: number, col: number): boolean =>
  FINDER_ORIGINS.some(([originRow, originCol]: readonly [number, number]) => {
    const localRow: number = row - originRow;
    const localCol: number = col - originCol;

    if (localRow < 0 || localRow > 6 || localCol < 0 || localCol > 6) {
      return false;
    }

    return Math.max(Math.abs(localRow - 3), Math.abs(localCol - 3)) !== 2;
  });

/** The finder patterns plus their one module quiet zone, which carries no data. */
const isReservedModule = (row: number, col: number): boolean =>
  FINDER_ORIGINS.some(
    ([originRow, originCol]: readonly [number, number]) =>
      row >= originRow - 1 && row <= originRow + 7 && col >= originCol - 1 && col <= originCol + 7,
  );

/**
 * Fills the data area from the module coordinates alone. The preview stands in for a code
 * that only exists at runtime, so the pattern just has to look plausible and, more
 * importantly, stay identical across renders.
 */
const isDataModule = (row: number, col: number): boolean => (row * 7 + col * 13 + row * col * 3) % 5 < 2;

/**
 * A canvas placeholder for the QR Code element. The real code is generated at runtime from the
 * `additionalData` key named in `source`, which has no value while authoring, so the canvas shows
 * a representative symbol and the bound key instead.
 *
 * @param props - Custom props containing the resource.
 * @returns The QrCodeAdapter placeholder component.
 */
function QrCodeAdapter({resource = undefined}: QrCodeAdapterPropsInterface): ReactElement {
  const source: string | undefined = (resource as QrCodeElement)?.source;

  const modules: ReactElement[] = [];

  for (let row = 0; row < MODULE_COUNT; row += 1) {
    for (let col = 0; col < MODULE_COUNT; col += 1) {
      if (isFinderModule(row, col) || (!isReservedModule(row, col) && isDataModule(row, col))) {
        modules.push(<rect key={`${row}-${col}`} x={col} y={row} width={1} height={1} />);
      }
    }
  }

  return (
    <Box sx={{alignItems: 'center', display: 'flex', flexDirection: 'column', gap: 1, py: 1, width: '100%'}}>
      <Box
        component="svg"
        viewBox={`-1 -1 ${MODULE_COUNT + 2} ${MODULE_COUNT + 2}`}
        role="img"
        aria-label="QR Code"
        shapeRendering="crispEdges"
        sx={{
          backgroundColor: 'common.white',
          border: '1px solid',
          borderColor: 'divider',
          borderRadius: 1,
          fill: 'common.black',
          height: 120,
          width: 120,
        }}
      >
        {modules}
      </Box>
      <Typography variant="caption" color="textSecondary" sx={{fontFamily: 'monospace'}}>
        {source ? `{{${source}}}` : 'No source bound'}
      </Typography>
    </Box>
  );
}

export default QrCodeAdapter;
