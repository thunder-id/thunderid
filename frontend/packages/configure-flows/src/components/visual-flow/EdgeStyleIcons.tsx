// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {type ReactElement} from 'react';

/**
 * Icon component for Bezier edge style
 */
export function BezierEdgeIcon(): ReactElement {
  return (
    <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
      <path d="M4 12 C 8 4, 16 20, 20 12" strokeLinecap="round" />
    </svg>
  );
}

/**
 * Icon component for Smooth Step edge style
 */
export function SmoothStepEdgeIcon(): ReactElement {
  return (
    <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
      <path d="M4 6 H 10 Q 12 6, 12 8 V 16 Q 12 18, 14 18 H 20" strokeLinecap="round" />
    </svg>
  );
}

/**
 * Icon component for Step edge style
 */
export function StepEdgeIcon(): ReactElement {
  return (
    <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
      <path d="M4 6 H 12 V 18 H 20" strokeLinecap="round" strokeLinejoin="miter" />
    </svg>
  );
}
