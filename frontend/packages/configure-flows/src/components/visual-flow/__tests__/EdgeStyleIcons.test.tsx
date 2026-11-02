// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render} from '@testing-library/react';
import {describe, it, expect} from 'vitest';
import {BezierEdgeIcon, SmoothStepEdgeIcon, StepEdgeIcon} from '../EdgeStyleIcons';

describe('EdgeStyleIcons', () => {
  describe('BezierEdgeIcon', () => {
    it('should render an SVG element', () => {
      const {container} = render(<BezierEdgeIcon />);

      const svg = container.querySelector('svg');
      expect(svg).toBeInTheDocument();
    });

    it('should have correct dimensions', () => {
      const {container} = render(<BezierEdgeIcon />);

      const svg = container.querySelector('svg');
      expect(svg).toHaveAttribute('width', '20');
      expect(svg).toHaveAttribute('height', '20');
    });

    it('should have correct viewBox', () => {
      const {container} = render(<BezierEdgeIcon />);

      const svg = container.querySelector('svg');
      expect(svg).toHaveAttribute('viewBox', '0 0 24 24');
    });

    it('should have correct stroke properties', () => {
      const {container} = render(<BezierEdgeIcon />);

      const svg = container.querySelector('svg');
      expect(svg).toHaveAttribute('stroke', 'currentColor');
      expect(svg).toHaveAttribute('stroke-width', '2');
      expect(svg).toHaveAttribute('fill', 'none');
    });

    it('should contain a path element with bezier curve', () => {
      const {container} = render(<BezierEdgeIcon />);

      const path = container.querySelector('path');
      expect(path).toBeInTheDocument();
      expect(path).toHaveAttribute('d', 'M4 12 C 8 4, 16 20, 20 12');
      expect(path).toHaveAttribute('stroke-linecap', 'round');
    });
  });

  describe('SmoothStepEdgeIcon', () => {
    it('should render an SVG element', () => {
      const {container} = render(<SmoothStepEdgeIcon />);

      const svg = container.querySelector('svg');
      expect(svg).toBeInTheDocument();
    });

    it('should have correct dimensions', () => {
      const {container} = render(<SmoothStepEdgeIcon />);

      const svg = container.querySelector('svg');
      expect(svg).toHaveAttribute('width', '20');
      expect(svg).toHaveAttribute('height', '20');
    });

    it('should have correct viewBox', () => {
      const {container} = render(<SmoothStepEdgeIcon />);

      const svg = container.querySelector('svg');
      expect(svg).toHaveAttribute('viewBox', '0 0 24 24');
    });

    it('should have correct stroke properties', () => {
      const {container} = render(<SmoothStepEdgeIcon />);

      const svg = container.querySelector('svg');
      expect(svg).toHaveAttribute('stroke', 'currentColor');
      expect(svg).toHaveAttribute('stroke-width', '2');
      expect(svg).toHaveAttribute('fill', 'none');
    });

    it('should contain a path element with smooth step curve', () => {
      const {container} = render(<SmoothStepEdgeIcon />);

      const path = container.querySelector('path');
      expect(path).toBeInTheDocument();
      expect(path).toHaveAttribute('d', 'M4 6 H 10 Q 12 6, 12 8 V 16 Q 12 18, 14 18 H 20');
      expect(path).toHaveAttribute('stroke-linecap', 'round');
    });
  });

  describe('StepEdgeIcon', () => {
    it('should render an SVG element', () => {
      const {container} = render(<StepEdgeIcon />);

      const svg = container.querySelector('svg');
      expect(svg).toBeInTheDocument();
    });

    it('should have correct dimensions', () => {
      const {container} = render(<StepEdgeIcon />);

      const svg = container.querySelector('svg');
      expect(svg).toHaveAttribute('width', '20');
      expect(svg).toHaveAttribute('height', '20');
    });

    it('should have correct viewBox', () => {
      const {container} = render(<StepEdgeIcon />);

      const svg = container.querySelector('svg');
      expect(svg).toHaveAttribute('viewBox', '0 0 24 24');
    });

    it('should have correct stroke properties', () => {
      const {container} = render(<StepEdgeIcon />);

      const svg = container.querySelector('svg');
      expect(svg).toHaveAttribute('stroke', 'currentColor');
      expect(svg).toHaveAttribute('stroke-width', '2');
      expect(svg).toHaveAttribute('fill', 'none');
    });

    it('should contain a path element with step curve', () => {
      const {container} = render(<StepEdgeIcon />);

      const path = container.querySelector('path');
      expect(path).toBeInTheDocument();
      expect(path).toHaveAttribute('d', 'M4 6 H 12 V 18 H 20');
      expect(path).toHaveAttribute('stroke-linecap', 'round');
      expect(path).toHaveAttribute('stroke-linejoin', 'miter');
    });
  });

  describe('Icon Consistency', () => {
    it('all icons should have the same dimensions', () => {
      const {container: bezierContainer} = render(<BezierEdgeIcon />);
      const {container: smoothStepContainer} = render(<SmoothStepEdgeIcon />);
      const {container: stepContainer} = render(<StepEdgeIcon />);

      const bezierSvg = bezierContainer.querySelector('svg');
      const smoothStepSvg = smoothStepContainer.querySelector('svg');
      const stepSvg = stepContainer.querySelector('svg');

      expect(bezierSvg?.getAttribute('width')).toBe(smoothStepSvg?.getAttribute('width'));
      expect(smoothStepSvg?.getAttribute('width')).toBe(stepSvg?.getAttribute('width'));
      expect(bezierSvg?.getAttribute('height')).toBe(smoothStepSvg?.getAttribute('height'));
      expect(smoothStepSvg?.getAttribute('height')).toBe(stepSvg?.getAttribute('height'));
    });

    it('all icons should have the same stroke width', () => {
      const {container: bezierContainer} = render(<BezierEdgeIcon />);
      const {container: smoothStepContainer} = render(<SmoothStepEdgeIcon />);
      const {container: stepContainer} = render(<StepEdgeIcon />);

      const bezierSvg = bezierContainer.querySelector('svg');
      const smoothStepSvg = smoothStepContainer.querySelector('svg');
      const stepSvg = stepContainer.querySelector('svg');

      expect(bezierSvg?.getAttribute('stroke-width')).toBe(smoothStepSvg?.getAttribute('stroke-width'));
      expect(smoothStepSvg?.getAttribute('stroke-width')).toBe(stepSvg?.getAttribute('stroke-width'));
    });

    it('all icons should use currentColor for stroke', () => {
      const {container: bezierContainer} = render(<BezierEdgeIcon />);
      const {container: smoothStepContainer} = render(<SmoothStepEdgeIcon />);
      const {container: stepContainer} = render(<StepEdgeIcon />);

      const bezierSvg = bezierContainer.querySelector('svg');
      const smoothStepSvg = smoothStepContainer.querySelector('svg');
      const stepSvg = stepContainer.querySelector('svg');

      expect(bezierSvg).toHaveAttribute('stroke', 'currentColor');
      expect(smoothStepSvg).toHaveAttribute('stroke', 'currentColor');
      expect(stepSvg).toHaveAttribute('stroke', 'currentColor');
    });
  });
});
