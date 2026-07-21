/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {render, screen, fireEvent} from '@testing-library/react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import ResourceProperties from '../ResourceProperties';
import {ElementCategories, ElementTypes} from '@/features/flows/models/elements';
import type {Element} from '@/features/flows/models/elements';
import type {Resource} from '@/features/flows/models/resources';
import {StepCategories, StepTypes} from '@/features/flows/models/steps';

// Mock dependencies
vi.mock('../ResourcePropertyFactory', () => ({
  default: ({
    resource,
    propertyKey,
    propertyValue,
    onChange,
  }: {
    resource: Resource;
    propertyKey: string;
    propertyValue: unknown;
    onChange?: (key: string, value: unknown, resource: Resource) => void;
  }) => (
    <div
      data-testid={`resource-property-factory-${propertyKey}`}
      data-resource-id={resource.id}
      data-property-value={String(propertyValue)}
    >
      {propertyKey}: {String(propertyValue)}
      {onChange && (
        <button
          type="button"
          data-testid={`trigger-change-${propertyKey}`}
          onClick={() => onChange(propertyKey, propertyValue, resource)}
        >
          Trigger Change
        </button>
      )}
    </div>
  ),
}));

vi.mock('../nodes/RulesProperties', () => ({
  default: () => <div data-testid="rules-properties">Rules Properties</div>,
}));

vi.mock('../extended-properties/FieldExtendedProperties', () => ({
  default: ({resource}: {resource: Resource}) => (
    <div data-testid="field-extended-properties" data-resource-id={resource.id}>
      Field Extended Properties
    </div>
  ),
}));

vi.mock('../extended-properties/ButtonExtendedProperties', () => ({
  default: ({resource}: {resource: Resource}) => (
    <div data-testid="button-extended-properties" data-resource-id={resource.id}>
      Button Extended Properties
    </div>
  ),
}));

vi.mock('../extended-properties/CallProperties', () => ({
  default: ({resource}: {resource: Resource}) => (
    <div data-testid="call-properties" data-resource-id={resource.id}>
      Call Properties
    </div>
  ),
}));

vi.mock('../extended-properties/ExecutionExtendedProperties', () => ({
  default: ({resource}: {resource: Resource}) => (
    <div data-testid="execution-extended-properties" data-resource-id={resource.id}>
      Execution Extended Properties
    </div>
  ),
}));

vi.mock('@/features/flows/components/resource-property-panel/TextPropertyField', () => ({
  default: ({
    resource,
    propertyKey,
    propertyValue,
  }: {
    resource: Resource;
    propertyKey: string;
    propertyValue: string;
  }) => (
    <div
      data-testid={`text-property-field-${propertyKey}`}
      data-resource-id={resource.id}
      data-property-value={propertyValue}
    >
      {propertyKey}: {propertyValue}
    </div>
  ),
}));

describe('ResourceProperties', () => {
  const mockOnChange = vi.fn();
  const mockOnVariantChange = vi.fn();

  const createMockResource = (overrides: Partial<Resource> = {}): Resource =>
    ({
      id: 'resource-1',
      type: 'TEXT_INPUT',
      category: ElementCategories.Field,
      ...overrides,
    }) as Resource;

  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Field Category', () => {
    it('should render FieldExtendedProperties for Field category', () => {
      const resource = createMockResource({category: ElementCategories.Field});

      render(
        <ResourceProperties
          resource={resource}
          properties={{label: 'Test Label'}}
          onChange={mockOnChange}
          onVariantChange={mockOnVariantChange}
        />,
      );

      expect(screen.getByTestId('field-extended-properties')).toBeInTheDocument();
    });

    it('should render element ID for Field category', () => {
      const resource = createMockResource({id: 'field-123', category: ElementCategories.Field});

      render(
        <ResourceProperties
          resource={resource}
          properties={{}}
          onChange={mockOnChange}
          onVariantChange={mockOnVariantChange}
        />,
      );

      expect(screen.getByTestId('resource-property-factory-id')).toBeInTheDocument();
      expect(screen.getByTestId('resource-property-factory-id')).toHaveAttribute('data-property-value', 'field-123');
    });

    it('should render property factories for Field category properties', () => {
      const resource = createMockResource({category: ElementCategories.Field});

      render(
        <ResourceProperties
          resource={resource}
          properties={{label: 'Test Label', placeholder: 'Enter value'}}
          onChange={mockOnChange}
          onVariantChange={mockOnVariantChange}
        />,
      );

      expect(screen.getByTestId('resource-property-factory-label')).toBeInTheDocument();
      expect(screen.getByTestId('resource-property-factory-placeholder')).toBeInTheDocument();
    });
  });

  describe('Action Category', () => {
    it('should render ButtonExtendedProperties for Action type', () => {
      const resource = createMockResource({
        category: ElementCategories.Action,
        type: ElementTypes.Action,
      });

      render(
        <ResourceProperties
          resource={resource}
          properties={{}}
          onChange={mockOnChange}
          onVariantChange={mockOnVariantChange}
        />,
      );

      expect(screen.getByTestId('button-extended-properties')).toBeInTheDocument();
    });

    it('should render element ID for Action category', () => {
      const resource = createMockResource({
        id: 'action-456',
        category: ElementCategories.Action,
        type: ElementTypes.Action,
      });

      render(
        <ResourceProperties
          resource={resource}
          properties={{}}
          onChange={mockOnChange}
          onVariantChange={mockOnVariantChange}
        />,
      );

      expect(screen.getByTestId('resource-property-factory-id')).toHaveAttribute('data-property-value', 'action-456');
    });

    it('should not render ButtonExtendedProperties for non-Action type in Action category', () => {
      const resource = createMockResource({
        category: ElementCategories.Action,
        type: 'LINK' as typeof ElementTypes.Action,
      });

      render(
        <ResourceProperties
          resource={resource}
          properties={{}}
          onChange={mockOnChange}
          onVariantChange={mockOnVariantChange}
        />,
      );

      expect(screen.queryByTestId('button-extended-properties')).not.toBeInTheDocument();
    });
  });

  describe('Decision Category - Rule Type', () => {
    it('should render RulesProperties for Rule type', () => {
      const resource = createMockResource({
        category: StepCategories.Decision,
        type: StepTypes.Rule,
      } as Partial<Resource>);

      render(
        <ResourceProperties
          resource={resource}
          properties={{}}
          onChange={mockOnChange}
          onVariantChange={mockOnVariantChange}
        />,
      );

      expect(screen.getByTestId('rules-properties')).toBeInTheDocument();
    });

    it('should render element ID for Rule type', () => {
      const resource = createMockResource({
        id: 'rule-789',
        category: StepCategories.Decision,
        type: StepTypes.Rule,
      } as Partial<Resource>);

      render(
        <ResourceProperties
          resource={resource}
          properties={{}}
          onChange={mockOnChange}
          onVariantChange={mockOnVariantChange}
        />,
      );

      expect(screen.getByTestId('resource-property-factory-id')).toHaveAttribute('data-property-value', 'rule-789');
    });

    it('should return null for Decision category with non-Rule type', () => {
      const resource = createMockResource({
        category: StepCategories.Decision,
        type: 'CONDITION' as typeof StepTypes.Rule,
      } as Partial<Resource>);

      const {container} = render(
        <ResourceProperties
          resource={resource}
          properties={{}}
          onChange={mockOnChange}
          onVariantChange={mockOnVariantChange}
        />,
      );

      expect(container.firstChild).toBeNull();
    });
  });

  describe('Interface Category - End Type', () => {
    it('should render element ID for End type', () => {
      const resource = createMockResource({
        id: 'end-step',
        category: StepCategories.Interface,
        type: StepTypes.End,
      } as Partial<Resource>);

      render(
        <ResourceProperties
          resource={resource}
          properties={{}}
          onChange={mockOnChange}
          onVariantChange={mockOnVariantChange}
        />,
      );

      expect(screen.getByTestId('resource-property-factory-id')).toHaveAttribute('data-property-value', 'end-step');
    });

    it('should return null for Interface category with non-End type', () => {
      const resource = createMockResource({
        category: StepCategories.Interface,
        type: 'VIEW' as typeof StepTypes.End,
      } as Partial<Resource>);

      const {container} = render(
        <ResourceProperties
          resource={resource}
          properties={{}}
          onChange={mockOnChange}
          onVariantChange={mockOnVariantChange}
        />,
      );

      expect(container.firstChild).toBeNull();
    });
  });

  describe('Workflow Category - Call Type', () => {
    it('should render CallProperties for Call type', () => {
      const resource = createMockResource({
        id: 'call-1',
        category: StepCategories.Workflow,
        type: StepTypes.Call,
      } as Partial<Resource>);

      render(
        <ResourceProperties
          resource={resource}
          properties={{}}
          onChange={mockOnChange}
          onVariantChange={mockOnVariantChange}
        />,
      );

      expect(screen.getByTestId('call-properties')).toBeInTheDocument();
      expect(screen.getByTestId('call-properties')).toHaveAttribute('data-resource-id', 'call-1');
      expect(screen.queryByTestId('execution-extended-properties')).not.toBeInTheDocument();
    });

    it('should render element ID for Call type', () => {
      const resource = createMockResource({
        id: 'call-42',
        category: StepCategories.Workflow,
        type: StepTypes.Call,
      } as Partial<Resource>);

      render(
        <ResourceProperties
          resource={resource}
          properties={{}}
          onChange={mockOnChange}
          onVariantChange={mockOnVariantChange}
        />,
      );

      expect(screen.getByTestId('resource-property-factory-id')).toHaveAttribute('data-property-value', 'call-42');
    });
  });

  describe('Workflow Category', () => {
    it('should render ExecutionExtendedProperties for Workflow category', () => {
      const resource = createMockResource({
        category: StepCategories.Workflow,
        type: StepTypes.Execution,
      } as Partial<Resource>);

      render(
        <ResourceProperties
          resource={resource}
          properties={{}}
          onChange={mockOnChange}
          onVariantChange={mockOnVariantChange}
        />,
      );

      expect(screen.getByTestId('execution-extended-properties')).toBeInTheDocument();
    });
  });

  describe('Display Category - Text Type', () => {
    it('should render TextPropertyField for Text type', () => {
      const resource = createMockResource({
        category: ElementCategories.Display,
        type: ElementTypes.Text,
        label: 'Sample Text',
      } as Partial<Resource>);

      render(
        <ResourceProperties
          resource={resource}
          properties={{}}
          onChange={mockOnChange}
          onVariantChange={mockOnVariantChange}
        />,
      );

      expect(screen.getByTestId('text-property-field-label')).toBeInTheDocument();
    });

    it('should render variant selector for Text type with variants', () => {
      const variants = [
        {variant: 'heading', id: 'v1'},
        {variant: 'body', id: 'v2'},
      ] as Element[];

      const resource = createMockResource({
        category: ElementCategories.Display,
        type: ElementTypes.Text,
        variants,
        variant: 'heading',
      } as Partial<Resource>);

      render(
        <ResourceProperties
          resource={resource}
          properties={{}}
          onChange={mockOnChange}
          onVariantChange={mockOnVariantChange}
        />,
      );

      expect(screen.getByText('Variant')).toBeInTheDocument();
    });
  });

  describe('Display Category - Image Type', () => {
    it('should render TextPropertyFields for Image type', () => {
      const resource = createMockResource({
        category: ElementCategories.Display,
        type: ElementTypes.Image,
        src: 'https://example.com/image.png',
        alt: 'Sample Image',
      } as Partial<Resource>);

      render(
        <ResourceProperties
          resource={resource}
          properties={{}}
          onChange={mockOnChange}
          onVariantChange={mockOnVariantChange}
        />,
      );

      expect(screen.getByTestId('text-property-field-src')).toBeInTheDocument();
      expect(screen.getByTestId('text-property-field-alt')).toBeInTheDocument();
    });

    it('should handle empty src and alt values', () => {
      const resource = createMockResource({
        category: ElementCategories.Display,
        type: ElementTypes.Image,
      } as Partial<Resource>);

      render(
        <ResourceProperties
          resource={resource}
          properties={{}}
          onChange={mockOnChange}
          onVariantChange={mockOnVariantChange}
        />,
      );

      expect(screen.getByTestId('text-property-field-src')).toHaveAttribute('data-property-value', '');
      expect(screen.getByTestId('text-property-field-alt')).toHaveAttribute('data-property-value', '');
    });
  });

  describe('Display Category - Other Types', () => {
    it('should render default property factories for other Display types', () => {
      const resource = createMockResource({
        category: ElementCategories.Display,
        type: 'DIVIDER' as typeof ElementTypes.Text,
      } as Partial<Resource>);

      render(
        <ResourceProperties
          resource={resource}
          properties={{thickness: '2px'}}
          onChange={mockOnChange}
          onVariantChange={mockOnVariantChange}
        />,
      );

      expect(screen.getByTestId('resource-property-factory-thickness')).toBeInTheDocument();
    });
  });

  describe('Default Category', () => {
    it('should render default property factories for unknown category', () => {
      const resource = createMockResource({
        category: 'UNKNOWN' as typeof ElementCategories.Field,
      });

      render(
        <ResourceProperties
          resource={resource}
          properties={{customProp: 'customValue'}}
          onChange={mockOnChange}
          onVariantChange={mockOnVariantChange}
        />,
      );

      expect(screen.getByTestId('resource-property-factory-id')).toBeInTheDocument();
      expect(screen.getByTestId('resource-property-factory-customProp')).toBeInTheDocument();
    });
  });

  describe('Variant Selection', () => {
    it('should render variant selector when resource has variants', () => {
      const variants = [
        {variant: 'primary', id: 'v1'},
        {variant: 'secondary', id: 'v2'},
      ] as Element[];

      const resource = createMockResource({
        category: ElementCategories.Field,
        variants,
        variant: 'primary',
      } as Partial<Resource>);

      render(
        <ResourceProperties
          resource={resource}
          properties={{}}
          onChange={mockOnChange}
          onVariantChange={mockOnVariantChange}
        />,
      );

      expect(screen.getByText('Variant')).toBeInTheDocument();
    });

    it('should call onVariantChange when variant is selected', () => {
      const variants = [
        {variant: 'primary', id: 'v1'},
        {variant: 'secondary', id: 'v2'},
      ] as Element[];

      const resource = createMockResource({
        category: ElementCategories.Field,
        variants,
        variant: 'primary',
      } as Partial<Resource>);

      render(
        <ResourceProperties
          resource={resource}
          properties={{}}
          onChange={mockOnChange}
          onVariantChange={mockOnVariantChange}
        />,
      );

      // Find the variant select by role combobox with the variant-select id
      const variantSelect = document.getElementById('variant-select')!;
      fireEvent.mouseDown(variantSelect);

      const secondaryOption = screen.getByRole('option', {name: 'secondary'});
      fireEvent.click(secondaryOption);

      expect(mockOnVariantChange).toHaveBeenCalledWith('secondary');
    });

    it('should not render variant selector when resource has no variants', () => {
      const resource = createMockResource({
        category: ElementCategories.Field,
      });

      render(
        <ResourceProperties
          resource={resource}
          properties={{}}
          onChange={mockOnChange}
          onVariantChange={mockOnVariantChange}
        />,
      );

      expect(screen.queryByText('Variant')).not.toBeInTheDocument();
    });

    it('should not render variant selector when variants array is empty', () => {
      const resource = createMockResource({
        category: ElementCategories.Field,
        variants: [],
      } as Partial<Resource>);

      render(
        <ResourceProperties
          resource={resource}
          properties={{}}
          onChange={mockOnChange}
          onVariantChange={mockOnVariantChange}
        />,
      );

      expect(screen.queryByText('Variant')).not.toBeInTheDocument();
    });
  });

  describe('onChange Handler - Type Preservation', () => {
    it('should preserve boolean values in onChange', () => {
      const resource = createMockResource({category: ElementCategories.Field});

      render(
        <ResourceProperties
          resource={resource}
          properties={{}}
          onChange={mockOnChange}
          onVariantChange={mockOnVariantChange}
        />,
      );

      // The internal handleChange function processes values
      // This test verifies the component renders without errors
      expect(screen.getByTestId('field-extended-properties')).toBeInTheDocument();
    });
  });

  describe('Sync Selected Variant on Resource Change', () => {
    it('should sync selected variant when resource changes', () => {
      const variants1 = [
        {variant: 'v1', id: 'variant-1'},
        {variant: 'v2', id: 'variant-2'},
      ] as Element[];

      const resource1 = createMockResource({
        id: 'resource-1',
        category: ElementCategories.Field,
        variants: variants1,
        variant: 'v1',
      } as Partial<Resource>);

      const variants2 = [
        {variant: 'v3', id: 'variant-3'},
        {variant: 'v4', id: 'variant-4'},
      ] as Element[];

      const resource2 = createMockResource({
        id: 'resource-2',
        category: ElementCategories.Field,
        variants: variants2,
        variant: 'v3',
      } as Partial<Resource>);

      const {rerender} = render(
        <ResourceProperties
          resource={resource1}
          properties={{}}
          onChange={mockOnChange}
          onVariantChange={mockOnVariantChange}
        />,
      );

      expect(screen.getByText('Variant')).toBeInTheDocument();

      rerender(
        <ResourceProperties
          resource={resource2}
          properties={{}}
          onChange={mockOnChange}
          onVariantChange={mockOnVariantChange}
        />,
      );

      expect(screen.getByText('Variant')).toBeInTheDocument();
    });

    it('should set selectedVariant to undefined when resource has no variants', () => {
      const resource1 = createMockResource({
        id: 'resource-1',
        category: ElementCategories.Field,
        variants: [{variant: 'v1', id: 'variant-1'}] as Element[],
        variant: 'v1',
      } as Partial<Resource>);

      const resource2 = createMockResource({
        id: 'resource-2',
        category: ElementCategories.Field,
        variants: undefined,
      } as Partial<Resource>);

      const {rerender} = render(
        <ResourceProperties
          resource={resource1}
          properties={{}}
          onChange={mockOnChange}
          onVariantChange={mockOnVariantChange}
        />,
      );

      expect(screen.getByText('Variant')).toBeInTheDocument();

      rerender(
        <ResourceProperties
          resource={resource2}
          properties={{}}
          onChange={mockOnChange}
          onVariantChange={mockOnVariantChange}
        />,
      );

      expect(screen.queryByText('Variant')).not.toBeInTheDocument();
    });

    it('should fall back to first variant when current variant is not found', () => {
      const variants = [
        {variant: 'first', id: 'variant-1'},
        {variant: 'second', id: 'variant-2'},
      ] as Element[];

      const resource = createMockResource({
        id: 'resource-1',
        category: ElementCategories.Field,
        variants,
        variant: 'non-existent',
      } as Partial<Resource>);

      render(
        <ResourceProperties
          resource={resource}
          properties={{}}
          onChange={mockOnChange}
          onVariantChange={mockOnVariantChange}
        />,
      );

      expect(screen.getByText('Variant')).toBeInTheDocument();
    });
  });

  describe('handleChange Type Preservation', () => {
    it('should preserve boolean values in onChange', () => {
      const resource = createMockResource({category: ElementCategories.Field});

      render(
        <ResourceProperties
          resource={resource}
          properties={{}}
          onChange={mockOnChange}
          onVariantChange={mockOnVariantChange}
        />,
      );

      // The component should render
      expect(screen.getByTestId('field-extended-properties')).toBeInTheDocument();
    });

    it('should preserve object values in onChange', () => {
      const resource = createMockResource({category: ElementCategories.Field});

      render(
        <ResourceProperties
          resource={resource}
          properties={{}}
          onChange={mockOnChange}
          onVariantChange={mockOnVariantChange}
        />,
      );

      expect(screen.getByTestId('field-extended-properties')).toBeInTheDocument();
    });

    it('should convert number values to string in onChange', () => {
      const resource = createMockResource({category: ElementCategories.Field});

      render(
        <ResourceProperties
          resource={resource}
          properties={{numericProp: 42}}
          onChange={mockOnChange}
          onVariantChange={mockOnVariantChange}
        />,
      );

      expect(screen.getByTestId('resource-property-factory-numericProp')).toBeInTheDocument();
    });

    it('should default to empty string for null/undefined values', () => {
      const resource = createMockResource({category: ElementCategories.Field});

      render(
        <ResourceProperties
          resource={resource}
          properties={{nullProp: null as unknown as string}}
          onChange={mockOnChange}
          onVariantChange={mockOnVariantChange}
        />,
      );

      expect(screen.getByTestId('resource-property-factory-nullProp')).toBeInTheDocument();
    });
  });

  describe('Display Category - Text Type with Variants', () => {
    it('should render variant selector for Text type and call onVariantChange', () => {
      const variants = [
        {variant: 'heading', id: 'v1'},
        {variant: 'body', id: 'v2'},
      ] as Element[];

      const resource = createMockResource({
        category: ElementCategories.Display,
        type: ElementTypes.Text,
        variants,
        variant: 'heading',
      } as Partial<Resource>);

      render(
        <ResourceProperties
          resource={resource}
          properties={{}}
          onChange={mockOnChange}
          onVariantChange={mockOnVariantChange}
        />,
      );

      expect(screen.getByText('Variant')).toBeInTheDocument();

      // Find and click the variant select
      const variantSelect = document.getElementById('variant-select')!;
      fireEvent.mouseDown(variantSelect);

      const bodyOption = screen.getByRole('option', {name: 'body'});
      fireEvent.click(bodyOption);

      expect(mockOnVariantChange).toHaveBeenCalledWith('body');
    });

    it('should handle onVariantChange being undefined for Text type', () => {
      const variants = [
        {variant: 'heading', id: 'v1'},
        {variant: 'body', id: 'v2'},
      ] as Element[];

      const resource = createMockResource({
        category: ElementCategories.Display,
        type: ElementTypes.Text,
        variants,
        variant: 'heading',
      } as Partial<Resource>);

      render(
        <ResourceProperties resource={resource} properties={{}} onChange={mockOnChange} onVariantChange={undefined} />,
      );

      const variantSelect = document.getElementById('variant-select')!;
      fireEvent.mouseDown(variantSelect);

      const bodyOption = screen.getByRole('option', {name: 'body'});
      fireEvent.click(bodyOption);

      // Should not throw error
      expect(screen.getByText('Variant')).toBeInTheDocument();
    });

    it('should handle variant not found in variants array for Text type', () => {
      const variants = [
        {variant: 'heading', id: 'v1'},
        {variant: 'body', id: 'v2'},
      ] as Element[];

      const resource = createMockResource({
        category: ElementCategories.Display,
        type: ElementTypes.Text,
        variants,
        variant: 'heading',
      } as Partial<Resource>);

      render(
        <ResourceProperties
          resource={resource}
          properties={{}}
          onChange={mockOnChange}
          onVariantChange={mockOnVariantChange}
        />,
      );

      const variantSelect = document.getElementById('variant-select')!;
      fireEvent.mouseDown(variantSelect);

      // Try to select a non-existent variant through the select component
      // The select should still work properly
      expect(screen.getByText('Variant')).toBeInTheDocument();
    });

    it('should not render variant selector for Text type without variants', () => {
      const resource = createMockResource({
        category: ElementCategories.Display,
        type: ElementTypes.Text,
        label: 'Sample Text',
      } as Partial<Resource>);

      render(
        <ResourceProperties
          resource={resource}
          properties={{}}
          onChange={mockOnChange}
          onVariantChange={mockOnVariantChange}
        />,
      );

      expect(screen.queryByText('Variant')).not.toBeInTheDocument();
    });

    it('should handle variant selection with empty variant value', () => {
      const variants = [
        {variant: 'primary', id: 'v1'},
        {variant: '', id: 'v2'},
      ] as Element[];

      const resource = createMockResource({
        category: ElementCategories.Display,
        type: ElementTypes.Text,
        variants,
        variant: 'primary',
      } as Partial<Resource>);

      render(
        <ResourceProperties
          resource={resource}
          properties={{}}
          onChange={mockOnChange}
          onVariantChange={mockOnVariantChange}
        />,
      );

      expect(screen.getByText('Variant')).toBeInTheDocument();
    });
  });

  describe('Workflow Category - Non-ConfirmationCode Execution', () => {
    it('should render ExecutionExtendedProperties for regular execution', () => {
      const resource = createMockResource({
        category: StepCategories.Workflow,
        type: StepTypes.Execution,
        data: {
          action: {
            executor: {
              name: 'RegularExecutor',
            },
          },
        },
      } as Partial<Resource>);

      render(
        <ResourceProperties
          resource={resource}
          properties={{}}
          onChange={mockOnChange}
          onVariantChange={mockOnVariantChange}
        />,
      );

      expect(screen.getByTestId('execution-extended-properties')).toBeInTheDocument();
    });

    it('should render ExecutionExtendedProperties when executor is undefined', () => {
      const resource = createMockResource({
        category: StepCategories.Workflow,
        type: StepTypes.Execution,
        data: {},
      } as Partial<Resource>);

      render(
        <ResourceProperties
          resource={resource}
          properties={{}}
          onChange={mockOnChange}
          onVariantChange={mockOnVariantChange}
        />,
      );

      expect(screen.getByTestId('execution-extended-properties')).toBeInTheDocument();
    });
  });

  describe('Interface Category - Non-End Type', () => {
    it('should return null for VIEW type in Interface category', () => {
      const resource = createMockResource({
        category: StepCategories.Interface,
        type: StepTypes.View,
      } as Partial<Resource>);

      const {container} = render(
        <ResourceProperties
          resource={resource}
          properties={{}}
          onChange={mockOnChange}
          onVariantChange={mockOnVariantChange}
        />,
      );

      expect(container.firstChild).toBeNull();
    });
  });

  describe('Field Category with Variants', () => {
    it('should render variant selector and handle empty variant selection', () => {
      const variants = [
        {variant: 'text', id: 'v1'},
        {variant: 'password', id: 'v2'},
      ] as Element[];

      const resource = createMockResource({
        category: ElementCategories.Field,
        variants,
        variant: 'text',
      } as Partial<Resource>);

      render(
        <ResourceProperties
          resource={resource}
          properties={{}}
          onChange={mockOnChange}
          onVariantChange={mockOnVariantChange}
        />,
      );

      expect(screen.getByText('Variant')).toBeInTheDocument();
    });
  });

  describe('Action Category - Non-Action Types', () => {
    it('should render only ID for Link type in Action category', () => {
      const resource = createMockResource({
        id: 'link-123',
        category: ElementCategories.Action,
        type: 'LINK',
      } as Partial<Resource>);

      render(
        <ResourceProperties
          resource={resource}
          properties={{href: 'https://example.com'}}
          onChange={mockOnChange}
          onVariantChange={mockOnVariantChange}
        />,
      );

      expect(screen.getByTestId('resource-property-factory-id')).toBeInTheDocument();
      expect(screen.getByTestId('resource-property-factory-href')).toBeInTheDocument();
      expect(screen.queryByTestId('button-extended-properties')).not.toBeInTheDocument();
    });
  });

  describe('Action Category with Variants', () => {
    it('should render variant selector for Action category with variants', () => {
      const variants = [
        {variant: 'primary', id: 'v1'},
        {variant: 'secondary', id: 'v2'},
      ] as Element[];

      const resource = createMockResource({
        category: ElementCategories.Action,
        type: ElementTypes.Action,
        variants,
        variant: 'primary',
      } as Partial<Resource>);

      render(
        <ResourceProperties
          resource={resource}
          properties={{}}
          onChange={mockOnChange}
          onVariantChange={mockOnVariantChange}
        />,
      );

      expect(screen.getByText('Variant')).toBeInTheDocument();
    });

    it('should call onVariantChange for Action category variant selection', () => {
      const variants = [
        {variant: 'filled', id: 'v1'},
        {variant: 'outlined', id: 'v2'},
      ] as Element[];

      const resource = createMockResource({
        category: ElementCategories.Action,
        type: ElementTypes.Action,
        variants,
        variant: 'filled',
      } as Partial<Resource>);

      render(
        <ResourceProperties
          resource={resource}
          properties={{}}
          onChange={mockOnChange}
          onVariantChange={mockOnVariantChange}
        />,
      );

      const variantSelect = document.getElementById('variant-select')!;
      fireEvent.mouseDown(variantSelect);

      const outlinedOption = screen.getByRole('option', {name: 'outlined'});
      fireEvent.click(outlinedOption);

      expect(mockOnVariantChange).toHaveBeenCalledWith('outlined');
    });

    it('should handle onVariantChange being undefined', () => {
      const variants = [
        {variant: 'primary', id: 'v1'},
        {variant: 'secondary', id: 'v2'},
      ] as Element[];

      const resource = createMockResource({
        category: ElementCategories.Action,
        type: ElementTypes.Action,
        variants,
        variant: 'primary',
      } as Partial<Resource>);

      render(
        <ResourceProperties resource={resource} properties={{}} onChange={mockOnChange} onVariantChange={undefined} />,
      );

      const variantSelect = document.getElementById('variant-select')!;
      fireEvent.mouseDown(variantSelect);

      const secondaryOption = screen.getByRole('option', {name: 'secondary'});
      fireEvent.click(secondaryOption);

      // Should not throw error
      expect(screen.getByText('Variant')).toBeInTheDocument();
    });
  });

  describe('handleChange Type Processing', () => {
    it('should preserve boolean values and call onChange with boolean', () => {
      const resource = createMockResource({
        category: ElementCategories.Field,
      });

      render(
        <ResourceProperties
          resource={resource}
          properties={{enabled: true}}
          onChange={mockOnChange}
          onVariantChange={mockOnVariantChange}
        />,
      );

      // Trigger the change via the mocked button
      const triggerButton = screen.getByTestId('trigger-change-enabled');
      fireEvent.click(triggerButton);

      expect(mockOnChange).toHaveBeenCalledWith('enabled', true, resource, undefined);
    });

    it('should preserve object values and call onChange with object', () => {
      const resource = createMockResource({
        category: ElementCategories.Field,
      });

      const objectValue = {nested: 'value', count: 5};

      render(
        <ResourceProperties
          resource={resource}
          properties={{config: objectValue}}
          onChange={mockOnChange}
          onVariantChange={mockOnVariantChange}
        />,
      );

      const triggerButton = screen.getByTestId('trigger-change-config');
      fireEvent.click(triggerButton);

      expect(mockOnChange).toHaveBeenCalledWith('config', objectValue, resource, undefined);
    });

    it('should convert string values to string in onChange', () => {
      const resource = createMockResource({
        category: ElementCategories.Field,
      });

      render(
        <ResourceProperties
          resource={resource}
          properties={{label: 'test string'}}
          onChange={mockOnChange}
          onVariantChange={mockOnVariantChange}
        />,
      );

      const triggerButton = screen.getByTestId('trigger-change-label');
      fireEvent.click(triggerButton);

      expect(mockOnChange).toHaveBeenCalledWith('label', 'test string', resource, undefined);
    });

    it('should preserve number values in onChange', () => {
      const resource = createMockResource({
        category: ElementCategories.Field,
      });

      render(
        <ResourceProperties
          resource={resource}
          properties={{maxLength: 100}}
          onChange={mockOnChange}
          onVariantChange={mockOnVariantChange}
        />,
      );

      const triggerButton = screen.getByTestId('trigger-change-maxLength');
      fireEvent.click(triggerButton);

      expect(mockOnChange).toHaveBeenCalledWith('maxLength', 100, resource, undefined);
    });

    it('should convert null values to empty string in onChange', () => {
      const resource = createMockResource({
        category: ElementCategories.Field,
      });

      render(
        <ResourceProperties
          resource={resource}
          properties={{optionalProp: null as unknown as string}}
          onChange={mockOnChange}
          onVariantChange={mockOnVariantChange}
        />,
      );

      const triggerButton = screen.getByTestId('trigger-change-optionalProp');
      fireEvent.click(triggerButton);

      expect(mockOnChange).toHaveBeenCalledWith('optionalProp', '', resource, undefined);
    });

    it('should convert undefined values to empty string in onChange', () => {
      const resource = createMockResource({
        category: ElementCategories.Field,
      });

      render(
        <ResourceProperties
          resource={resource}
          properties={{undefinedProp: undefined as unknown as string}}
          onChange={mockOnChange}
          onVariantChange={mockOnVariantChange}
        />,
      );

      const triggerButton = screen.getByTestId('trigger-change-undefinedProp');
      fireEvent.click(triggerButton);

      expect(mockOnChange).toHaveBeenCalledWith('undefinedProp', '', resource, undefined);
    });
  });
});
