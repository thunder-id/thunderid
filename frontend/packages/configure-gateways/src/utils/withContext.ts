// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {LineOp} from '../models/deploy';

/** How many unchanged lines are kept on each side of a changed one. */
const CONTEXT = 3;

export type Shown = {line: LineOp; index: number} | {gap: number; index: number};

/**
 * Keeps the changed lines and the context around them, and folds each longer run of unchanged lines
 * into a gap that says how many it hides.
 */
export default function withContext(lines: LineOp[]): Shown[] {
  const near = lines.map((_line: LineOp, index: number) =>
    lines.slice(Math.max(0, index - CONTEXT), index + CONTEXT + 1).some((line: LineOp) => line.kind !== ' '),
  );
  const shown: Shown[] = [];
  let hidden = 0;
  lines.forEach((line: LineOp, index: number) => {
    if (near[index]) {
      if (hidden > 0) shown.push({gap: hidden, index: index - 1});
      hidden = 0;
      shown.push({line, index});
    } else {
      hidden += 1;
    }
  });
  if (hidden > 0) shown.push({gap: hidden, index: lines.length});
  return shown;
}
