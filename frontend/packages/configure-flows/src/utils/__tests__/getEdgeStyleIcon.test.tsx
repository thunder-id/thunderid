// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render} from '@testing-library/react';
import {describe, it, expect} from 'vitest';
import {EdgeStyleTypes} from '../../models/steps';
import getEdgeStyleIcon from '../getEdgeStyleIcon';

describe('getEdgeStyleIcon', () => {
  it('should return BezierEdgeIcon for Bezier style', () => {
    const icon = getEdgeStyleIcon(EdgeStyleTypes.Bezier);
    const {container} = render(icon);

    const svg = container.querySelector('svg');
    expect(svg).toBeInTheDocument();

    // Bezier has a C (curve) command in the path
    const path = container.querySelector('path');
    expect(path?.getAttribute('d')).toContain('C');
  });

  it('should return SmoothStepEdgeIcon for SmoothStep style', () => {
    const icon = getEdgeStyleIcon(EdgeStyleTypes.SmoothStep);
    const {container} = render(icon);

    const svg = container.querySelector('svg');
    expect(svg).toBeInTheDocument();

    // SmoothStep has Q (quadratic) commands in the path
    const path = container.querySelector('path');
    expect(path?.getAttribute('d')).toContain('Q');
  });

  it('should return StepEdgeIcon for Step style', () => {
    const icon = getEdgeStyleIcon(EdgeStyleTypes.Step);
    const {container} = render(icon);

    const svg = container.querySelector('svg');
    expect(svg).toBeInTheDocument();

    // Step has only H and V (horizontal and vertical) commands
    const path = container.querySelector('path');
    const d = path?.getAttribute('d') ?? '';
    expect(d).toContain('H');
    expect(d).toContain('V');
    expect(d).not.toContain('C');
    expect(d).not.toContain('Q');
  });

  it('should return SmoothStepEdgeIcon as default for unknown style', () => {
    // @ts-expect-error Testing with invalid style
    const icon = getEdgeStyleIcon('unknown-style');
    const {container} = render(icon);

    const svg = container.querySelector('svg');
    expect(svg).toBeInTheDocument();

    // Default is SmoothStep which has Q commands
    const path = container.querySelector('path');
    expect(path?.getAttribute('d')).toContain('Q');
  });

  it('should return a ReactElement for all valid styles', () => {
    const styles = [EdgeStyleTypes.Bezier, EdgeStyleTypes.SmoothStep, EdgeStyleTypes.Step];

    styles.forEach((style) => {
      const icon = getEdgeStyleIcon(style);
      expect(icon).toBeDefined();
      expect(icon.type).toBeDefined();
    });
  });

  it('should return icons with consistent SVG structure', () => {
    const styles = [EdgeStyleTypes.Bezier, EdgeStyleTypes.SmoothStep, EdgeStyleTypes.Step];

    styles.forEach((style) => {
      const icon = getEdgeStyleIcon(style);
      const {container} = render(icon);

      const svg = container.querySelector('svg');
      expect(svg).toHaveAttribute('width', '20');
      expect(svg).toHaveAttribute('height', '20');
      expect(svg).toHaveAttribute('viewBox', '0 0 24 24');
      expect(svg).toHaveAttribute('stroke', 'currentColor');
    });
  });
});
