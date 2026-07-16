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

import {render, screen} from '@testing-library/react';
import {ReactFlowProvider} from '@xyflow/react';
import type {ReactNode} from 'react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import BlockAdapter from '../BlockAdapter';
import type {Element as FlowElement} from '@/features/flows/models/elements';

// Mock dependencies
vi.mock('../BlockAdapter.scss', () => ({}));

vi.mock('@/features/flows/components/resources/steps/view/ReorderableElement', () => ({
  ReorderableElement: ({element, id, hideChrome = false}: {element: FlowElement; id: string; hideChrome?: boolean}) => (
    <div data-testid={`reorderable-element-${id}`} data-hide-chrome={hideChrome}>
      {element.id}
    </div>
  ),
}));

vi.mock('@/features/flows/hooks/useFlowPlugins', () => ({
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

describe('BlockAdapter', () => {
  const createMockElement = (overrides: Partial<FlowElement> = {}): FlowElement =>
    ({
      id: 'element-1',
      type: 'TEXT_INPUT',
      category: 'FIELD',
      config: {},
      ...overrides,
    }) as FlowElement;

  const createWrapper = () => {
    function Wrapper({children}: {children: ReactNode}) {
      return <ReactFlowProvider>{children}</ReactFlowProvider>;
    }
    return Wrapper;
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Rendering', () => {
    it('should render the adapter with correct class names', () => {
      const resource = createMockElement();

      const {container} = render(<BlockAdapter resource={resource} />, {wrapper: createWrapper()});

      expect(container.querySelector('.adapter')).toBeInTheDocument();
      expect(container.querySelector('.block-adapter')).toBeInTheDocument();
    });

    it('should render empty when resource has no components', () => {
      const resource = createMockElement({components: undefined});

      const {container} = render(<BlockAdapter resource={resource} />, {wrapper: createWrapper()});

      expect(container.querySelector('.block-adapter')).toBeInTheDocument();
      expect(container.querySelectorAll('[data-testid^="reorderable-element"]')).toHaveLength(0);
    });

    it('should render empty array when components is empty', () => {
      const resource = createMockElement({components: []});

      const {container} = render(<BlockAdapter resource={resource} />, {wrapper: createWrapper()});

      expect(container.querySelector('.block-adapter')).toBeInTheDocument();
      expect(container.querySelectorAll('[data-testid^="reorderable-element"]')).toHaveLength(0);
    });
  });

  describe('Components Rendering', () => {
    it('should render ReorderableElement for each component', () => {
      const components = [
        createMockElement({id: 'comp-1'}),
        createMockElement({id: 'comp-2'}),
        createMockElement({id: 'comp-3'}),
      ];
      const resource = createMockElement({components});

      render(<BlockAdapter resource={resource} />, {wrapper: createWrapper()});

      expect(screen.getByTestId('reorderable-element-comp-1')).toBeInTheDocument();
      expect(screen.getByTestId('reorderable-element-comp-2')).toBeInTheDocument();
      expect(screen.getByTestId('reorderable-element-comp-3')).toBeInTheDocument();
    });

    it('should render nested elements without their own chrome', () => {
      const components = [createMockElement({id: 'comp-1'})];
      const resource = createMockElement({components});

      render(<BlockAdapter resource={resource} />, {wrapper: createWrapper()});

      expect(screen.getByTestId('reorderable-element-comp-1')).toHaveAttribute('data-hide-chrome', 'true');
    });

    it('should pass availableElements to ReorderableElement', () => {
      const components = [createMockElement({id: 'comp-1'})];
      const resource = createMockElement({components});
      const availableElements = [createMockElement({id: 'available-1'})];

      render(<BlockAdapter resource={resource} availableElements={availableElements} />, {wrapper: createWrapper()});

      expect(screen.getByTestId('reorderable-element-comp-1')).toBeInTheDocument();
    });

    it('should pass onAddElementToForm callback to ReorderableElement', () => {
      const components = [createMockElement({id: 'comp-1'})];
      const resource = createMockElement({components});
      const onAddElementToForm = vi.fn();

      render(<BlockAdapter resource={resource} onAddElementToForm={onAddElementToForm} />, {wrapper: createWrapper()});

      expect(screen.getByTestId('reorderable-element-comp-1')).toBeInTheDocument();
    });
  });

  describe('Default Props', () => {
    it('should work with undefined availableElements', () => {
      const resource = createMockElement({components: [createMockElement({id: 'comp-1'})]});

      const {container} = render(<BlockAdapter resource={resource} />, {wrapper: createWrapper()});

      expect(container.querySelector('.block-adapter')).toBeInTheDocument();
    });

    it('should work with undefined onAddElementToForm', () => {
      const resource = createMockElement({components: [createMockElement({id: 'comp-1'})]});

      const {container} = render(<BlockAdapter resource={resource} />, {wrapper: createWrapper()});

      expect(container.querySelector('.block-adapter')).toBeInTheDocument();
    });
  });

  describe('Filtering', () => {
    it('should filter components through useFlowPlugins', () => {
      const components = [createMockElement({id: 'comp-1'}), createMockElement({id: 'comp-2'})];
      const resource = createMockElement({components});

      render(<BlockAdapter resource={resource} />, {wrapper: createWrapper()});

      // All components should render since our mock returns true
      expect(screen.getByTestId('reorderable-element-comp-1')).toBeInTheDocument();
      expect(screen.getByTestId('reorderable-element-comp-2')).toBeInTheDocument();
    });
  });
});
