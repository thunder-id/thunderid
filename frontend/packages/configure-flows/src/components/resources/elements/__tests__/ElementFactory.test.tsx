// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import {describe, it, expect, vi} from 'vitest';
import type {Element} from '../../../../models/elements';
import {ResourceTypes} from '../../../../models/resources';
import ElementFactory from '../ElementFactory';

// Mock CommonElementFactory
vi.mock('../CommonElementFactory', () => ({
  default: ({resource, stepId}: {resource: Element; stepId: string}) => (
    <div data-testid="common-element-factory" data-resource-id={resource.id} data-step-id={stepId}>
      Common Element Factory
    </div>
  ),
}));

describe('ElementFactory', () => {
  const createMockResource = (overrides: Partial<Element> = {}): Element =>
    ({
      id: 'element-1',
      type: 'TEXT_INPUT',
      category: 'FIELD',
      resourceType: ResourceTypes.Element,
      ...overrides,
    }) as Element;

  describe('Valid Element Resource', () => {
    it('should render CommonElementFactory for Element resources', () => {
      const resource = createMockResource();

      render(<ElementFactory resource={resource} stepId="step-1" />);

      expect(screen.getByTestId('common-element-factory')).toBeInTheDocument();
    });

    it('should pass resource to CommonElementFactory', () => {
      const resource = createMockResource({id: 'custom-element'});

      render(<ElementFactory resource={resource} stepId="step-1" />);

      expect(screen.getByTestId('common-element-factory')).toHaveAttribute('data-resource-id', 'custom-element');
    });

    it('should pass stepId to CommonElementFactory', () => {
      const resource = createMockResource();

      render(<ElementFactory resource={resource} stepId="step-123" />);

      expect(screen.getByTestId('common-element-factory')).toHaveAttribute('data-step-id', 'step-123');
    });

    it('should render for resources without resourceType (template/widget components)', () => {
      const resource = {
        id: 'template-element',
        type: 'TEXT_INPUT',
        category: 'FIELD',
      } as Element;

      render(<ElementFactory resource={resource} stepId="step-1" />);

      expect(screen.getByTestId('common-element-factory')).toBeInTheDocument();
    });
  });

  describe('Invalid Resources', () => {
    it('should return null when resource is null', () => {
      const {container} = render(<ElementFactory resource={null as unknown as Element} stepId="step-1" />);

      expect(container.firstChild).toBeNull();
    });

    it('should return null when resource is undefined', () => {
      const {container} = render(<ElementFactory resource={undefined as unknown as Element} stepId="step-1" />);

      expect(container.firstChild).toBeNull();
    });

    it('should return null for Step resource type', () => {
      const resource = createMockResource({resourceType: ResourceTypes.Step} as Partial<Element>);

      const {container} = render(<ElementFactory resource={resource} stepId="step-1" />);

      expect(container.firstChild).toBeNull();
    });

    it('should return null for Widget resource type', () => {
      const resource = createMockResource({resourceType: ResourceTypes.Widget} as Partial<Element>);

      const {container} = render(<ElementFactory resource={resource} stepId="step-1" />);

      expect(container.firstChild).toBeNull();
    });
  });
});
