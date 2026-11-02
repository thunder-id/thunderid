// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {type ReactElement} from 'react';
import {BezierEdgeIcon, SmoothStepEdgeIcon, StepEdgeIcon} from '../components/visual-flow/EdgeStyleIcons';
import {EdgeStyleTypes, type EdgeStyleTypes as EdgeStyleTypesType} from '../models/steps';

/**
 * Returns the appropriate icon component for the given edge style
 */
export default function getEdgeStyleIcon(style: EdgeStyleTypesType): ReactElement {
  switch (style) {
    case EdgeStyleTypes.Bezier:
      return <BezierEdgeIcon />;
    case EdgeStyleTypes.SmoothStep:
      return <SmoothStepEdgeIcon />;
    case EdgeStyleTypes.Step:
      return <StepEdgeIcon />;
    default:
      return <SmoothStepEdgeIcon />;
  }
}
