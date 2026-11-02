// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import {describe, it, expect, vi} from 'vitest';
import type {Resource} from '../../../models/resources';
import {StepTypes} from '../../../models/steps';
import StepPropertyFactory from '../StepPropertyFactory';

// Mock CommonStepPropertyFactory
vi.mock('../CommonStepPropertyFactory', () => ({
  default: ({resource, propertyKey}: {resource: Resource; propertyKey: string}) => (
    <div data-testid="common-step-property-factory" data-resource-id={resource.id} data-property-key={propertyKey}>
      Common Step Property Factory
    </div>
  ),
}));

// Mock RulesProperties
vi.mock('../nodes/RulesProperties', () => ({
  default: () => <div data-testid="rules-properties">Rules Properties</div>,
}));

describe('StepPropertyFactory', () => {
  const mockOnChange = vi.fn();

  const createMockResource = (type: string, overrides: Partial<Resource> = {}): Resource =>
    ({
      id: 'step-1',
      type,
      category: 'DECISION',
      resourceType: 'STEP',
      ...overrides,
    }) as Resource;

  describe('Rule Step Type', () => {
    it('should render RulesProperties for Rule step type', () => {
      const resource = createMockResource(StepTypes.Rule);

      render(
        <StepPropertyFactory
          resource={resource}
          propertyKey="condition"
          propertyValue="test"
          onChange={mockOnChange}
        />,
      );

      expect(screen.getByTestId('rules-properties')).toBeInTheDocument();
    });

    it('should not render CommonStepPropertyFactory for Rule step type', () => {
      const resource = createMockResource(StepTypes.Rule);

      render(
        <StepPropertyFactory
          resource={resource}
          propertyKey="condition"
          propertyValue="test"
          onChange={mockOnChange}
        />,
      );

      expect(screen.queryByTestId('common-step-property-factory')).not.toBeInTheDocument();
    });
  });

  describe('Other Step Types', () => {
    it('should render CommonStepPropertyFactory for View step type', () => {
      const resource = createMockResource(StepTypes.View);

      render(
        <StepPropertyFactory
          resource={resource}
          propertyKey="name"
          propertyValue="Step Name"
          onChange={mockOnChange}
        />,
      );

      expect(screen.getByTestId('common-step-property-factory')).toBeInTheDocument();
    });

    it('should render CommonStepPropertyFactory for Execution step type', () => {
      const resource = createMockResource(StepTypes.Execution);

      render(
        <StepPropertyFactory
          resource={resource}
          propertyKey="executor"
          propertyValue="executor-1"
          onChange={mockOnChange}
        />,
      );

      expect(screen.getByTestId('common-step-property-factory')).toBeInTheDocument();
    });

    it('should pass props to CommonStepPropertyFactory', () => {
      const resource = createMockResource(StepTypes.View, {id: 'view-step-123'});

      render(
        <StepPropertyFactory
          resource={resource}
          propertyKey="description"
          propertyValue="Step description"
          onChange={mockOnChange}
        />,
      );

      const factory = screen.getByTestId('common-step-property-factory');
      expect(factory).toHaveAttribute('data-resource-id', 'view-step-123');
      expect(factory).toHaveAttribute('data-property-key', 'description');
    });
  });
});
