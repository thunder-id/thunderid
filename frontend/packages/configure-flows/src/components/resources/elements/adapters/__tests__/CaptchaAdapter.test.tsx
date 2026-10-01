// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {Element as FlowElement} from '../../../../../models/elements';
import {ElementTypes} from '../../../../../models/elements';
import CaptchaAdapter from '../CaptchaAdapter';

describe('CaptchaAdapter', () => {
  const createMockElement = (overrides: Record<string, unknown> = {}): FlowElement =>
    ({
      id: 'captcha-1',
      resourceType: 'ELEMENT',
      type: 'CAPTCHA',
      category: 'FIELD',
      version: '1.0.0',
      deprecated: false,
      deletable: true,
      display: {
        label: 'Captcha',
        image: '',
        showOnResourcePanel: false,
      },
      config: {
        field: {name: 'captcha', type: ElementTypes},
        styles: {},
      },
      alt: 'reCAPTCHA',
      ...overrides,
    }) as unknown as FlowElement;

  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Rendering', () => {
    it('should render the captcha SVG', () => {
      const resource = createMockElement();

      const {container} = render(<CaptchaAdapter resource={resource} />);

      const svg = container.querySelector('svg');
      expect(svg).toBeInTheDocument();
    });

    it('should render captcha with correct dimensions', () => {
      const resource = createMockElement();

      const {container} = render(<CaptchaAdapter resource={resource} />);

      const svg = container.querySelector('svg');
      expect(svg).toHaveAttribute('width', '276');
      expect(svg).toHaveAttribute('height', '80');
    });

    it('should render centered in a flex container', () => {
      const resource = createMockElement();

      const {container} = render(<CaptchaAdapter resource={resource} />);

      const box = container.firstChild;
      expect(box).toBeInTheDocument();
    });
  });

  describe('Alt Text', () => {
    it('should render title with alt text from resource', () => {
      const resource = createMockElement({alt: 'Custom Captcha Title'});

      render(<CaptchaAdapter resource={resource} />);

      expect(screen.getByTitle('Custom Captcha Title')).toBeInTheDocument();
    });

    it('should handle undefined alt text', () => {
      const resource = createMockElement({alt: undefined});

      const {container} = render(<CaptchaAdapter resource={resource} />);

      const title = container.querySelector('title');
      expect(title).toBeInTheDocument();
    });
  });

  describe('SVG Structure', () => {
    it('should contain filter definitions', () => {
      const resource = createMockElement();

      const {container} = render(<CaptchaAdapter resource={resource} />);

      const defs = container.querySelector('defs');
      expect(defs).toBeInTheDocument();
    });

    it('should contain clipPath', () => {
      const resource = createMockElement();

      const {container} = render(<CaptchaAdapter resource={resource} />);

      const clipPath = container.querySelector('clipPath');
      expect(clipPath).toBeInTheDocument();
    });

    it('should contain mask elements', () => {
      const resource = createMockElement();

      const {container} = render(<CaptchaAdapter resource={resource} />);

      const mask = container.querySelector('mask');
      expect(mask).toBeInTheDocument();
    });

    it('should have proper viewBox', () => {
      const resource = createMockElement();

      const {container} = render(<CaptchaAdapter resource={resource} />);

      const svg = container.querySelector('svg');
      expect(svg).toHaveAttribute('viewBox', '0 0 276 80');
    });
  });

  describe('Different Resource IDs', () => {
    it('should render with different resource IDs', () => {
      const resource1 = createMockElement({id: 'captcha-1'});
      const resource2 = createMockElement({id: 'captcha-2'});

      const {container: container1} = render(<CaptchaAdapter resource={resource1} />);
      const {container: container2} = render(<CaptchaAdapter resource={resource2} />);

      expect(container1.querySelector('svg')).toBeInTheDocument();
      expect(container2.querySelector('svg')).toBeInTheDocument();
    });
  });
});
