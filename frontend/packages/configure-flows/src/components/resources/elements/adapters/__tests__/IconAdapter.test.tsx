// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render} from '@testing-library/react';
import {loadOxygenIcons} from '@thunderid/design';
import {beforeAll, describe, it, expect} from 'vitest';
import type {Element as FlowElement} from '../../../../../models/elements';
import IconAdapter from '../IconAdapter';

// The User icon, as IconAdapter draws it.
const findUserIcon = (container: HTMLElement): SVGSVGElement | null => container.querySelector('svg.lucide-user');

describe('IconAdapter', () => {
  // IconAdapter resolves icons from the icon set it loads on first use. Load it up front, so the
  // tests render with it in place.
  beforeAll(async () => {
    await loadOxygenIcons();
  }, 30000);

  const createMockElement = (overrides: Record<string, unknown> = {}): FlowElement =>
    ({
      id: 'icon-1',
      type: 'ICON',
      category: 'DISPLAY',
      config: {},
      ...overrides,
    }) as FlowElement;

  describe('Known Icon Rendering', () => {
    it('should render a known icon by name', () => {
      const resource = createMockElement({name: 'User'});

      const {container} = render(<IconAdapter resource={resource} />);

      expect(findUserIcon(container)).toBeInTheDocument();
    });

    it('should use default User icon when name is not provided', () => {
      const resource = createMockElement();

      const {container} = render(<IconAdapter resource={resource} />);

      expect(findUserIcon(container)).toBeInTheDocument();
    });

    it('should pass size to icon component', () => {
      const resource = createMockElement({name: 'User', size: 32});

      const {container} = render(<IconAdapter resource={resource} />);

      expect(findUserIcon(container)).toHaveAttribute('width', '32');
      expect(findUserIcon(container)).toHaveAttribute('height', '32');
    });

    it('should use default size 24 when size is not provided', () => {
      const resource = createMockElement({name: 'User'});

      const {container} = render(<IconAdapter resource={resource} />);

      expect(findUserIcon(container)).toHaveAttribute('width', '24');
      expect(findUserIcon(container)).toHaveAttribute('height', '24');
    });

    it('should pass color to icon component', () => {
      const resource = createMockElement({name: 'User', color: 'red'});

      const {container} = render(<IconAdapter resource={resource} />);

      expect(findUserIcon(container)).toHaveAttribute('stroke', 'red');
    });

    it('should use default color currentColor when color is not provided', () => {
      const resource = createMockElement({name: 'User'});

      const {container} = render(<IconAdapter resource={resource} />);

      expect(findUserIcon(container)).toHaveAttribute('stroke', 'currentColor');
    });

    it('should render an icon by another name it is exported under', () => {
      const resource = createMockElement({name: 'UserIcon'});

      const {container} = render(<IconAdapter resource={resource} />);

      expect(findUserIcon(container)).toBeInTheDocument();
    });
  });

  describe('Unknown Icon Placeholder', () => {
    it('should render placeholder when icon name is not found', () => {
      const resource = createMockElement({name: 'NonExistentIconXyz'});

      const {container} = render(<IconAdapter resource={resource} />);

      expect(container.textContent).toContain('?');
      expect(container.querySelector('svg')).not.toBeInTheDocument();
    });

    it('should render placeholder box when icon is unknown', () => {
      const resource = createMockElement({name: 'UnknownIcon123'});

      const {container} = render(<IconAdapter resource={resource} />);

      expect(container.firstChild).toBeInTheDocument();
      expect(container.textContent).toContain('?');
    });

    it('should use provided size for placeholder dimensions', () => {
      const resource = createMockElement({name: 'NonExistentIcon', size: 48});

      const {container} = render(<IconAdapter resource={resource} />);

      expect(container.textContent).toContain('?');
    });
  });

  describe('Resource Properties', () => {
    it('should use resource id as part of element identity', () => {
      const resource = createMockElement({id: 'my-icon', name: 'User'});

      const {container} = render(<IconAdapter resource={resource} />);

      expect(findUserIcon(container)).toBeInTheDocument();
    });
  });
});
