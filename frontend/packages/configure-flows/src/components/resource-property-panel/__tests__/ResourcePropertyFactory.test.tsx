// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import {describe, it, expect, vi} from 'vitest';
import {ResourceTypes, type Resource} from '../../../models/resources';
import ResourcePropertyFactory from '../ResourcePropertyFactory';

// Mock child factories
vi.mock('../CommonElementPropertyFactory', () => ({
  default: ({resource, propertyKey}: {resource: Resource; propertyKey: string}) => (
    <div data-testid="element-property-factory" data-resource-id={resource.id} data-property-key={propertyKey}>
      Element Property Factory
    </div>
  ),
}));

vi.mock('../StepPropertyFactory', () => ({
  default: ({resource, propertyKey}: {resource: Resource; propertyKey: string}) => (
    <div data-testid="step-property-factory" data-resource-id={resource.id} data-property-key={propertyKey}>
      Step Property Factory
    </div>
  ),
}));

vi.mock('../CommonWidgetPropertyFactory', () => ({
  default: ({resource, propertyKey}: {resource: Resource; propertyKey: string}) => (
    <div data-testid="widget-property-factory" data-resource-id={resource.id} data-property-key={propertyKey}>
      Widget Property Factory
    </div>
  ),
}));

describe('ResourcePropertyFactory', () => {
  const mockOnChange = vi.fn();

  const createMockResource = (resourceType: string, overrides: Partial<Resource> = {}): Resource =>
    ({
      id: 'resource-1',
      type: 'TEXT_INPUT',
      category: 'FIELD',
      resourceType,
      ...overrides,
    }) as Resource;

  describe('Element Resource Type', () => {
    it('should render ElementPropertyFactory for Element resources', () => {
      const resource = createMockResource(ResourceTypes.Element);

      render(
        <ResourcePropertyFactory
          resource={resource}
          propertyKey="label"
          propertyValue="Test"
          onChange={mockOnChange}
        />,
      );

      expect(screen.getByTestId('element-property-factory')).toBeInTheDocument();
    });

    it('should pass props to ElementPropertyFactory', () => {
      const resource = createMockResource(ResourceTypes.Element, {id: 'element-123'});

      render(
        <ResourcePropertyFactory
          resource={resource}
          propertyKey="placeholder"
          propertyValue="Enter value"
          onChange={mockOnChange}
        />,
      );

      const factory = screen.getByTestId('element-property-factory');
      expect(factory).toHaveAttribute('data-resource-id', 'element-123');
      expect(factory).toHaveAttribute('data-property-key', 'placeholder');
    });
  });

  describe('Step Resource Type', () => {
    it('should render StepPropertyFactory for Step resources', () => {
      const resource = createMockResource(ResourceTypes.Step);

      render(
        <ResourcePropertyFactory
          resource={resource}
          propertyKey="name"
          propertyValue="Step Name"
          onChange={mockOnChange}
        />,
      );

      expect(screen.getByTestId('step-property-factory')).toBeInTheDocument();
    });

    it('should pass props to StepPropertyFactory', () => {
      const resource = createMockResource(ResourceTypes.Step, {id: 'step-456'});

      render(
        <ResourcePropertyFactory
          resource={resource}
          propertyKey="description"
          propertyValue="Step description"
          onChange={mockOnChange}
        />,
      );

      const factory = screen.getByTestId('step-property-factory');
      expect(factory).toHaveAttribute('data-resource-id', 'step-456');
      expect(factory).toHaveAttribute('data-property-key', 'description');
    });
  });

  describe('Widget Resource Type', () => {
    it('should render WidgetPropertyFactory for Widget resources', () => {
      const resource = createMockResource(ResourceTypes.Widget);

      render(
        <ResourcePropertyFactory
          resource={resource}
          propertyKey="title"
          propertyValue="Widget Title"
          onChange={mockOnChange}
        />,
      );

      expect(screen.getByTestId('widget-property-factory')).toBeInTheDocument();
    });

    it('should pass props to WidgetPropertyFactory', () => {
      const resource = createMockResource(ResourceTypes.Widget, {id: 'widget-789'});

      render(
        <ResourcePropertyFactory
          resource={resource}
          propertyKey="config"
          propertyValue="Config value"
          onChange={mockOnChange}
        />,
      );

      const factory = screen.getByTestId('widget-property-factory');
      expect(factory).toHaveAttribute('data-resource-id', 'widget-789');
      expect(factory).toHaveAttribute('data-property-key', 'config');
    });
  });

  describe('Unknown Resource Type', () => {
    it('should return null for unknown resource types', () => {
      const resource = createMockResource('UNKNOWN_TYPE');

      const {container} = render(
        <ResourcePropertyFactory
          resource={resource}
          propertyKey="label"
          propertyValue="Test"
          onChange={mockOnChange}
        />,
      );

      expect(container.firstChild).toBeNull();
    });
  });
});
