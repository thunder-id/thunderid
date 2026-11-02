// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {Widget} from '../models/widget';

export function widgetNeedsViewContainer(widget: Widget): boolean {
  const widgetSteps =
    (
      widget?.config?.data as {
        steps?: {__generationMeta__?: {strategy?: string}}[];
      }
    )?.steps ?? [];

  if (widgetSteps.length === 0) {
    return true;
  }

  // Widgets need a container when they have no generated steps yet, or when at
  // least one generated step must merge into an existing drop point. Missing
  // generation metadata is treated the same as "no merge strategy", so those
  // steps stay on the standalone path.
  return widgetSteps.some((step) => step?.__generationMeta__?.strategy === 'MERGE_WITH_DROP_POINT');
}
