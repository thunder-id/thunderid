// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import {describe, it, expect, vi} from 'vitest';
import type {Element} from '../../../../models/elements';
import type {Resources} from '../../../../models/resources';
import type {Step} from '../../../../models/steps';
import {StepTypes, StepCategories} from '../../../../models/steps';
import StepFactory from '../StepFactory';

// Mock CommonStepFactory
vi.mock('../CommonStepFactory', () => ({
  default: ({
    resourceId,
    resources,
    allResources,
    onAddElement,
    onAddElementToForm,
  }: {
    resourceId: string;
    resources: Step[];
    allResources?: Resources;
    onAddElement?: (element: Element) => void;
    onAddElementToForm?: (element: Element, formId: string) => void;
  }) => (
    <div
      data-testid="common-step-factory"
      data-resource-id={resourceId}
      data-resources-count={resources?.length ?? 0}
      data-has-all-resources={!!allResources}
      data-has-on-add-element={!!onAddElement}
      data-has-on-add-element-to-form={!!onAddElementToForm}
    >
      Common Step Factory
    </div>
  ),
}));

describe('StepFactory', () => {
  const createMockStep = (overrides: Partial<Step> = {}): Step =>
    ({
      id: 'step-1',
      type: StepTypes.View,
      category: StepCategories.Interface,
      ...overrides,
    }) as Step;

  const createNodeProps = (overrides: Record<string, unknown> = {}) => ({
    id: 'node-1',
    type: 'step',
    position: {x: 0, y: 0},
    positionAbsoluteX: 0,
    positionAbsoluteY: 0,
    isConnectable: true,
    zIndex: 0,
    draggable: true,
    selectable: true,
    deletable: true,
    selected: false,
    dragging: false,
    data: {},
    ...overrides,
  });

  describe('Rendering', () => {
    it('should render CommonStepFactory', () => {
      const props = {
        ...createNodeProps(),
        resourceId: 'resource-1',
        resources: [createMockStep()],
      };

      render(<StepFactory {...props} />);

      expect(screen.getByTestId('common-step-factory')).toBeInTheDocument();
    });

    it('should pass resourceId to CommonStepFactory', () => {
      const props = {
        ...createNodeProps(),
        resourceId: 'my-resource-id',
        resources: [createMockStep()],
      };

      render(<StepFactory {...props} />);

      expect(screen.getByTestId('common-step-factory')).toHaveAttribute('data-resource-id', 'my-resource-id');
    });

    it('should pass resources to CommonStepFactory', () => {
      const props = {
        ...createNodeProps(),
        resourceId: 'resource-1',
        resources: [createMockStep(), createMockStep({id: 'step-2'})],
      };

      render(<StepFactory {...props} />);

      expect(screen.getByTestId('common-step-factory')).toHaveAttribute('data-resources-count', '2');
    });
  });

  describe('Optional Props', () => {
    it('should pass allResources when provided', () => {
      const allResources: Resources = {
        elements: [],
        steps: [],
        widgets: [],
        templates: [],
        executors: [],
      };
      const props = {
        ...createNodeProps(),
        resourceId: 'resource-1',
        resources: [createMockStep()],
        allResources,
      };

      render(<StepFactory {...props} />);

      expect(screen.getByTestId('common-step-factory')).toHaveAttribute('data-has-all-resources', 'true');
    });

    it('should handle undefined allResources', () => {
      const props = {
        ...createNodeProps(),
        resourceId: 'resource-1',
        resources: [createMockStep()],
      };

      render(<StepFactory {...props} />);

      expect(screen.getByTestId('common-step-factory')).toHaveAttribute('data-has-all-resources', 'false');
    });

    it('should pass onAddElement callback when provided', () => {
      const onAddElement = vi.fn();
      const props = {
        ...createNodeProps(),
        resourceId: 'resource-1',
        resources: [createMockStep()],
        onAddElement,
      };

      render(<StepFactory {...props} />);

      expect(screen.getByTestId('common-step-factory')).toHaveAttribute('data-has-on-add-element', 'true');
    });

    it('should pass onAddElementToForm callback when provided', () => {
      const onAddElementToForm = vi.fn();
      const props = {
        ...createNodeProps(),
        resourceId: 'resource-1',
        resources: [createMockStep()],
        onAddElementToForm,
      };

      render(<StepFactory {...props} />);

      expect(screen.getByTestId('common-step-factory')).toHaveAttribute('data-has-on-add-element-to-form', 'true');
    });

    it('should handle all optional props together', () => {
      const allResources: Resources = {
        elements: [],
        steps: [],
        widgets: [],
        templates: [],
        executors: [],
      };
      const onAddElement = vi.fn();
      const onAddElementToForm = vi.fn();
      const props = {
        ...createNodeProps(),
        resourceId: 'resource-1',
        resources: [createMockStep()],
        allResources,
        onAddElement,
        onAddElementToForm,
      };

      render(<StepFactory {...props} />);

      const factory = screen.getByTestId('common-step-factory');
      expect(factory).toHaveAttribute('data-has-all-resources', 'true');
      expect(factory).toHaveAttribute('data-has-on-add-element', 'true');
      expect(factory).toHaveAttribute('data-has-on-add-element-to-form', 'true');
    });
  });

  describe('Memoization', () => {
    it('should render with same props', () => {
      const props = {
        ...createNodeProps(),
        resourceId: 'resource-1',
        resources: [createMockStep()],
      };

      const {rerender} = render(<StepFactory {...props} />);

      expect(screen.getByTestId('common-step-factory')).toBeInTheDocument();

      rerender(<StepFactory {...props} />);

      expect(screen.getByTestId('common-step-factory')).toBeInTheDocument();
    });

    it('should render with different resource IDs', () => {
      const props1 = {
        ...createNodeProps(),
        resourceId: 'resource-1',
        resources: [createMockStep()],
      };

      const {rerender} = render(<StepFactory {...props1} />);

      expect(screen.getByTestId('common-step-factory')).toHaveAttribute('data-resource-id', 'resource-1');

      // Create props2 with different id (which is in memo comparison) to trigger re-render
      const props2 = {
        ...props1,
        id: 'node-2',
        resourceId: 'resource-2',
      };

      rerender(<StepFactory {...props2} />);

      expect(screen.getByTestId('common-step-factory')).toHaveAttribute('data-resource-id', 'resource-2');
    });
  });

  describe('Different Step Types', () => {
    it('should render with View step type', () => {
      const props = {
        ...createNodeProps(),
        resourceId: 'resource-1',
        resources: [createMockStep({type: StepTypes.View})],
      };

      render(<StepFactory {...props} />);

      expect(screen.getByTestId('common-step-factory')).toBeInTheDocument();
    });

    it('should render with Rule step type', () => {
      const props = {
        ...createNodeProps(),
        resourceId: 'resource-1',
        resources: [createMockStep({type: StepTypes.Rule, category: StepCategories.Decision})],
      };

      render(<StepFactory {...props} />);

      expect(screen.getByTestId('common-step-factory')).toBeInTheDocument();
    });

    it('should render with Execution step type', () => {
      const props = {
        ...createNodeProps(),
        resourceId: 'resource-1',
        resources: [createMockStep({type: StepTypes.Execution, category: StepCategories.Workflow})],
      };

      render(<StepFactory {...props} />);

      expect(screen.getByTestId('common-step-factory')).toBeInTheDocument();
    });

    it('should render with End step type', () => {
      const props = {
        ...createNodeProps(),
        resourceId: 'resource-1',
        resources: [createMockStep({type: StepTypes.End})],
      };

      render(<StepFactory {...props} />);

      expect(screen.getByTestId('common-step-factory')).toBeInTheDocument();
    });
  });

  describe('Memo Comparison', () => {
    it('should not re-render when props are the same', () => {
      const resources = [createMockStep()];
      const data = {test: 'value'};
      const allResources: Resources = {
        elements: [],
        steps: [],
        widgets: [],
        templates: [],
        executors: [],
      };
      const onAddElement = vi.fn();
      const onAddElementToForm = vi.fn();

      const props = {
        ...createNodeProps({data}),
        resourceId: 'resource-1',
        resources,
        allResources,
        onAddElement,
        onAddElementToForm,
      };

      const {rerender} = render(<StepFactory {...props} />);
      expect(screen.getByTestId('common-step-factory')).toBeInTheDocument();

      rerender(<StepFactory {...props} />);
      expect(screen.getByTestId('common-step-factory')).toBeInTheDocument();
    });

    it('should re-render when id changes', () => {
      const resources = [createMockStep()];
      const props1 = {
        ...createNodeProps({id: 'node-1'}),
        resourceId: 'resource-1',
        resources,
      };

      const {rerender} = render(<StepFactory {...props1} />);
      expect(screen.getByTestId('common-step-factory')).toBeInTheDocument();

      const props2 = {
        ...props1,
        id: 'node-2',
      };

      rerender(<StepFactory {...props2} />);
      expect(screen.getByTestId('common-step-factory')).toBeInTheDocument();
    });

    it('should re-render when data changes', () => {
      const resources = [createMockStep()];
      const props1 = {
        ...createNodeProps({data: {value: 1}}),
        resourceId: 'resource-1',
        resources,
      };

      const {rerender} = render(<StepFactory {...props1} />);
      expect(screen.getByTestId('common-step-factory')).toBeInTheDocument();

      const props2 = {
        ...props1,
        data: {value: 2},
      };

      rerender(<StepFactory {...props2} />);
      expect(screen.getByTestId('common-step-factory')).toBeInTheDocument();
    });

    it('should re-render when resources change', () => {
      const props1 = {
        ...createNodeProps(),
        resourceId: 'resource-1',
        resources: [createMockStep()],
      };

      const {rerender} = render(<StepFactory {...props1} />);
      expect(screen.getByTestId('common-step-factory')).toHaveAttribute('data-resources-count', '1');

      const props2 = {
        ...props1,
        resources: [createMockStep(), createMockStep({id: 'step-2'})],
      };

      rerender(<StepFactory {...props2} />);
      expect(screen.getByTestId('common-step-factory')).toHaveAttribute('data-resources-count', '2');
    });

    it('should re-render when allResources changes', () => {
      const props1 = {
        ...createNodeProps(),
        resourceId: 'resource-1',
        resources: [createMockStep()],
        allResources: undefined,
      };

      const {rerender} = render(<StepFactory {...props1} />);
      expect(screen.getByTestId('common-step-factory')).toHaveAttribute('data-has-all-resources', 'false');

      const props2 = {
        ...props1,
        allResources: {
          elements: [],
          steps: [],
          widgets: [],
          templates: [],
          executors: [],
        },
      };

      rerender(<StepFactory {...props2} />);
      expect(screen.getByTestId('common-step-factory')).toHaveAttribute('data-has-all-resources', 'true');
    });

    it('should re-render when onAddElement changes', () => {
      const onAddElement1 = vi.fn();
      const props1 = {
        ...createNodeProps(),
        resourceId: 'resource-1',
        resources: [createMockStep()],
        onAddElement: onAddElement1,
      };

      const {rerender} = render(<StepFactory {...props1} />);
      expect(screen.getByTestId('common-step-factory')).toHaveAttribute('data-has-on-add-element', 'true');

      const onAddElement2 = vi.fn();
      const props2 = {
        ...props1,
        onAddElement: onAddElement2,
      };

      rerender(<StepFactory {...props2} />);
      expect(screen.getByTestId('common-step-factory')).toHaveAttribute('data-has-on-add-element', 'true');
    });

    it('should re-render when onAddElementToForm changes', () => {
      const onAddElementToForm1 = vi.fn();
      const props1 = {
        ...createNodeProps(),
        resourceId: 'resource-1',
        resources: [createMockStep()],
        onAddElementToForm: onAddElementToForm1,
      };

      const {rerender} = render(<StepFactory {...props1} />);
      expect(screen.getByTestId('common-step-factory')).toHaveAttribute('data-has-on-add-element-to-form', 'true');

      const onAddElementToForm2 = vi.fn();
      const props2 = {
        ...props1,
        onAddElementToForm: onAddElementToForm2,
      };

      rerender(<StepFactory {...props2} />);
      expect(screen.getByTestId('common-step-factory')).toHaveAttribute('data-has-on-add-element-to-form', 'true');
    });

    it('should handle transition from undefined to defined callbacks', () => {
      const props1 = {
        ...createNodeProps(),
        resourceId: 'resource-1',
        resources: [createMockStep()],
        onAddElement: undefined,
        onAddElementToForm: undefined,
      };

      const {rerender} = render(<StepFactory {...props1} />);
      expect(screen.getByTestId('common-step-factory')).toHaveAttribute('data-has-on-add-element', 'false');
      expect(screen.getByTestId('common-step-factory')).toHaveAttribute('data-has-on-add-element-to-form', 'false');

      const props2 = {
        ...props1,
        onAddElement: vi.fn(),
        onAddElementToForm: vi.fn(),
      };

      rerender(<StepFactory {...props2} />);
      expect(screen.getByTestId('common-step-factory')).toHaveAttribute('data-has-on-add-element', 'true');
      expect(screen.getByTestId('common-step-factory')).toHaveAttribute('data-has-on-add-element-to-form', 'true');
    });
  });
});
