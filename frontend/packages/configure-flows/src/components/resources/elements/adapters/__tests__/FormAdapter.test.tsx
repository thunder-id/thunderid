// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, fireEvent} from '@testing-library/react';
import type {ReactNode} from 'react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import {ElementCategories, type Element as FlowElement} from '../../../../../models/elements';
import FormAdapter from '../FormAdapter';

// Mock dependencies

vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (key: string) => key,
  }),
  Trans: ({children}: {children: ReactNode}) => children,
}));

vi.mock('../../../../../hooks/useFlowPlugins', () => ({
  default: () => ({
    onPropertyChange: vi.fn().mockReturnValue(vi.fn()),
    emitPropertyChange: vi.fn().mockReturnValue(true),
    onPropertyPanelOpen: vi.fn().mockReturnValue(vi.fn()),
    emitPropertyPanelOpen: vi.fn().mockReturnValue(true),
    onElementFilter: vi.fn().mockReturnValue(vi.fn()),
    emitElementFilter: vi.fn().mockReturnValue(true),
    onEdgeDelete: vi.fn().mockReturnValue(vi.fn()),
    emitEdgeDelete: vi.fn().mockReturnValue(true),
    onNodeDelete: vi.fn().mockReturnValue(vi.fn()),
    emitNodeDelete: vi.fn().mockReturnValue(true),
    onNodeElementDelete: vi.fn().mockReturnValue(vi.fn()),
    emitNodeElementDelete: vi.fn().mockReturnValue(true),
    onTemplateLoad: vi.fn().mockReturnValue(vi.fn()),
    emitTemplateLoad: vi.fn().mockReturnValue(true),
  }),
}));

vi.mock('../../../../../utils/generateResourceId', () => ({
  default: (prefix: string) => `${prefix}-generated`,
}));

vi.mock('../../../steps/view/ReorderableElement', () => ({
  default: ({element, id}: {element: FlowElement; id: string}) => (
    <div data-testid={`reorderable-element-${id}`}>{element.id}</div>
  ),
}));

vi.mock('../../../../dnd/Droppable', () => ({
  default: ({children, id}: {children: ReactNode; id: string}) => (
    <div data-testid="droppable" data-droppable-id={id}>
      {children}
    </div>
  ),
}));

describe('FormAdapter', () => {
  const createMockElement = (overrides: Partial<FlowElement> = {}): FlowElement =>
    ({
      id: 'form-1',
      type: 'BLOCK',
      category: 'BLOCK',
      config: {},
      ...overrides,
    }) as FlowElement;

  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Rendering', () => {
    it('should render the form adapter with Badge', () => {
      const resource = createMockElement();

      render(<FormAdapter resource={resource} stepId="step-1" />);

      expect(screen.getByTestId('form-adapter')).toBeInTheDocument();
    });

    it('should render Badge with form label', () => {
      const resource = createMockElement();

      render(<FormAdapter resource={resource} stepId="step-1" />);

      expect(screen.getByText('flows:core.adapters.form.badgeLabel')).toBeInTheDocument();
    });

    it('should render Droppable component', () => {
      const resource = createMockElement();

      render(<FormAdapter resource={resource} stepId="step-1" />);

      expect(screen.getByTestId('droppable')).toBeInTheDocument();
    });
  });

  describe('Placeholder Display', () => {
    it('should show placeholder when no FIELD components exist', () => {
      const resource = createMockElement({components: []});

      render(<FormAdapter resource={resource} stepId="step-1" />);

      expect(screen.getByText('flows:core.adapters.form.placeholder')).toBeInTheDocument();
    });

    it('should show placeholder when components is undefined', () => {
      const resource = createMockElement({components: undefined});

      render(<FormAdapter resource={resource} stepId="step-1" />);

      expect(screen.getByText('flows:core.adapters.form.placeholder')).toBeInTheDocument();
    });

    it('should not show placeholder when non-FIELD components exist', () => {
      const components = [
        createMockElement({id: 'comp-1', category: ElementCategories.Action}),
        createMockElement({id: 'comp-2', category: ElementCategories.Display}),
      ];
      const resource = createMockElement({components});

      render(<FormAdapter resource={resource} stepId="step-1" />);

      expect(screen.queryByText('flows:core.adapters.form.placeholder')).not.toBeInTheDocument();
    });

    it('should show placeholder when form is empty', () => {
      const resource = createMockElement({components: []});

      render(<FormAdapter resource={resource} stepId="step-1" />);

      expect(screen.getByText('flows:core.adapters.form.placeholder')).toBeInTheDocument();
    });

    it('should not show placeholder when FIELD components exist', () => {
      const components = [createMockElement({id: 'comp-1', category: ElementCategories.Field})];
      const resource = createMockElement({components});

      render(<FormAdapter resource={resource} stepId="step-1" />);

      expect(screen.queryByText('flows:core.adapters.form.placeholder')).not.toBeInTheDocument();
    });
  });

  describe('Components Rendering', () => {
    it('should render ReorderableFlowElement for each component', () => {
      const components = [
        createMockElement({id: 'comp-1', category: ElementCategories.Field}),
        createMockElement({id: 'comp-2', category: ElementCategories.Field}),
      ];
      const resource = createMockElement({components});

      render(<FormAdapter resource={resource} stepId="step-1" />);

      expect(screen.getByTestId('reorderable-element-comp-1')).toBeInTheDocument();
      expect(screen.getByTestId('reorderable-element-comp-2')).toBeInTheDocument();
    });

    it('should pass availableElements to ReorderableFlowElement', () => {
      const components = [createMockElement({id: 'comp-1', category: ElementCategories.Field})];
      const resource = createMockElement({components});
      const availableElements = [createMockElement({id: 'available-1'})];

      render(<FormAdapter resource={resource} stepId="step-1" availableElements={availableElements} />);

      expect(screen.getByTestId('reorderable-element-comp-1')).toBeInTheDocument();
    });

    it('should pass onAddElementToForm callback', () => {
      const components = [createMockElement({id: 'comp-1', category: ElementCategories.Field})];
      const resource = createMockElement({components});
      const onAddElementToForm = vi.fn();

      render(<FormAdapter resource={resource} stepId="step-1" onAddElementToForm={onAddElementToForm} />);

      expect(screen.getByTestId('reorderable-element-comp-1')).toBeInTheDocument();
    });
  });

  describe('Droppable Configuration', () => {
    it('should have unique droppable ID based on stepId', () => {
      const resource = createMockElement();

      render(<FormAdapter resource={resource} stepId="step-123" />);

      const droppable = screen.getByTestId('droppable');
      expect(droppable.getAttribute('data-droppable-id')).toContain('step-123');
    });
  });

  describe('Default Props', () => {
    it('should work with undefined availableElements', () => {
      const resource = createMockElement();

      render(<FormAdapter resource={resource} stepId="step-1" />);

      expect(screen.getByTestId('form-adapter')).toBeInTheDocument();
    });

    it('should work with undefined onAddElementToForm', () => {
      const resource = createMockElement();

      render(<FormAdapter resource={resource} stepId="step-1" />);

      expect(screen.getByTestId('form-adapter')).toBeInTheDocument();
    });
  });

  describe('Filtering', () => {
    it('should filter components through useFlowPlugins', () => {
      const components = [
        createMockElement({id: 'comp-1', category: ElementCategories.Field}),
        createMockElement({id: 'comp-2', category: ElementCategories.Field}),
      ];
      const resource = createMockElement({components});

      render(<FormAdapter resource={resource} stepId="step-1" />);

      // All components should render since our mock returns true
      expect(screen.getByTestId('reorderable-element-comp-1')).toBeInTheDocument();
      expect(screen.getByTestId('reorderable-element-comp-2')).toBeInTheDocument();
    });
  });

  describe('Add field', () => {
    const formElements = [
      {id: 'text-input', type: 'TEXT_INPUT', display: {label: 'Text Input', showOnResourcePanel: true}},
      {id: 'hidden', type: 'TEXT_INPUT', display: {label: 'Hidden', showOnResourcePanel: false}},
      {id: 'captcha', type: 'CAPTCHA', display: {label: 'Captcha', showOnResourcePanel: true}},
    ] as never[];

    it('should render the add field button inside the form outline', () => {
      render(<FormAdapter resource={createMockElement()} stepId="step-1" availableElements={formElements} />);

      expect(screen.getByTestId('form-add-field-button')).toBeInTheDocument();
    });

    it('should offer only form-compatible elements shown on the resource panel', () => {
      render(<FormAdapter resource={createMockElement()} stepId="step-1" availableElements={formElements} />);

      fireEvent.click(screen.getByTestId('form-add-field-button'));

      expect(screen.getByText('Text Input')).toBeInTheDocument();
      expect(screen.queryByText('Hidden')).not.toBeInTheDocument();
      expect(screen.queryByText('Captcha')).not.toBeInTheDocument();
    });

    it('should add the chosen field to this form', () => {
      const onAddElementToForm = vi.fn();
      render(
        <FormAdapter
          resource={createMockElement()}
          stepId="step-1"
          availableElements={formElements}
          onAddElementToForm={onAddElementToForm}
        />,
      );

      fireEvent.click(screen.getByTestId('form-add-field-button'));
      fireEvent.click(screen.getByText('Text Input'));

      expect(onAddElementToForm).toHaveBeenCalledWith(
        expect.objectContaining({type: 'TEXT_INPUT'}),
        createMockElement().id,
      );
    });
  });
});
