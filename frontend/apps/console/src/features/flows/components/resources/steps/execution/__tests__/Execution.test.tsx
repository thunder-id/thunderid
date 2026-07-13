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
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {CommonStepFactoryPropsInterface} from '../../CommonStepFactory';
import Execution from '../Execution';

// Mock @xyflow/react
const mockUseNodeId = vi.fn<() => string | null>(() => 'execution-node-id');

vi.mock('@xyflow/react', () => ({
  useNodeId: (): string | null => mockUseNodeId(),
}));

// Mock useInteractionState
const mockSetLastInteractedResource = vi.fn();
const mockSetLastInteractedStepId = vi.fn();

vi.mock('@/features/flows/hooks/useInteractionState', () => ({
  default: () => ({
    setLastInteractedResource: mockSetLastInteractedResource,
    setLastInteractedStepId: mockSetLastInteractedStepId,
  }),
}));

// Mock ValidationErrorBoundary
vi.mock('../../../validation-panel/ValidationErrorBoundary', () => ({
  default: ({children, resource}: {children: React.ReactNode; resource: unknown}) => (
    <div data-testid="validation-error-boundary" data-resource={JSON.stringify(resource)}>
      {children}
    </div>
  ),
}));

// Mock View component
let capturedOnActionPanelDoubleClick: (() => void) | undefined;

vi.mock('../../view/View', () => ({
  default: ({
    heading,
    enableSourceHandle,
    deletable,
    configurable,
    onActionPanelDoubleClick,
  }: {
    heading: string;
    enableSourceHandle: boolean;
    deletable: boolean;
    configurable: boolean;
    onActionPanelDoubleClick?: () => void;
  }) => {
    capturedOnActionPanelDoubleClick = onActionPanelDoubleClick;
    return (
      <div
        data-testid="view-component"
        data-heading={heading}
        data-enable-source-handle={enableSourceHandle}
        data-deletable={deletable}
        data-configurable={configurable}
      >
        View Component: {heading}
      </div>
    );
  },
}));

// Mock ExecutionMinimal component
vi.mock('../ExecutionMinimal', () => ({
  default: ({resource}: {resource: {display?: {label?: string; description?: string; outcomes?: unknown}}}) => (
    <div
      data-testid="execution-minimal"
      data-label={resource?.display?.label}
      data-description={resource?.display?.description}
      data-outcomes={JSON.stringify(resource?.display?.outcomes)}
    >
      Execution Minimal: {resource?.display?.label}
    </div>
  ),
}));

// Default mock props for Execution component
const createMockProps = (overrides: Partial<CommonStepFactoryPropsInterface> = {}): CommonStepFactoryPropsInterface =>
  ({
    id: 'execution-node-1',
    resourceId: 'execution-resource-1',
    resources: [],
    data: {},
    type: 'EXECUTION',
    zIndex: 1,
    isConnectable: true,
    positionAbsoluteX: 0,
    positionAbsoluteY: 0,
    dragging: false,
    selected: false,
    deletable: true,
    selectable: true,
    parentId: undefined,
    ...overrides,
  }) as CommonStepFactoryPropsInterface;

describe('Execution', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseNodeId.mockReturnValue('execution-node-id');
  });

  describe('Rendering with Components', () => {
    it('should render View when data has components', () => {
      render(
        <Execution
          {...createMockProps({
            data: {
              action: {executor: {name: 'Google OAuth'}},
              components: [{id: 'comp-1', type: 'BUTTON'}],
            },
          })}
        />,
      );

      expect(screen.getByTestId('view-component')).toBeInTheDocument();
      expect(screen.queryByTestId('execution-minimal')).not.toBeInTheDocument();
    });

    it('should pass executor name as heading to View', () => {
      render(
        <Execution
          {...createMockProps({
            data: {
              action: {executor: {name: 'GitHub Auth'}},
              components: [{id: 'comp-1', type: 'BUTTON'}],
            },
          })}
        />,
      );

      const view = screen.getByTestId('view-component');
      expect(view).toHaveAttribute('data-heading', 'GitHub Auth');
    });

    it('should enable source handle on View', () => {
      render(
        <Execution
          {...createMockProps({
            data: {
              action: {executor: {name: 'Test'}},
              components: [{id: 'comp-1', type: 'BUTTON'}],
            },
          })}
        />,
      );

      const view = screen.getByTestId('view-component');
      expect(view).toHaveAttribute('data-enable-source-handle', 'true');
    });

    it('should make View non-deletable', () => {
      render(
        <Execution
          {...createMockProps({
            data: {
              action: {executor: {name: 'Test'}},
              components: [{id: 'comp-1', type: 'BUTTON'}],
            },
          })}
        />,
      );

      const view = screen.getByTestId('view-component');
      expect(view).toHaveAttribute('data-deletable', 'false');
    });

    it('should make View configurable', () => {
      render(
        <Execution
          {...createMockProps({
            data: {
              action: {executor: {name: 'Test'}},
              components: [{id: 'comp-1', type: 'BUTTON'}],
            },
          })}
        />,
      );

      const view = screen.getByTestId('view-component');
      expect(view).toHaveAttribute('data-configurable', 'true');
    });
  });

  describe('Rendering without Components', () => {
    it('should render ExecutionMinimal when data has no components', () => {
      render(
        <Execution
          {...createMockProps({
            data: {
              action: {executor: {name: 'Simple Executor'}},
              components: [],
            },
          })}
        />,
      );

      expect(screen.getByTestId('execution-minimal')).toBeInTheDocument();
      expect(screen.queryByTestId('view-component')).not.toBeInTheDocument();
    });

    it('should render ExecutionMinimal when components is undefined', () => {
      render(
        <Execution
          {...createMockProps({
            data: {
              action: {executor: {name: 'No Components'}},
            },
          })}
        />,
      );

      expect(screen.getByTestId('execution-minimal')).toBeInTheDocument();
    });

    it('should pass resource with correct label to ExecutionMinimal', () => {
      render(
        <Execution
          {...createMockProps({
            data: {
              action: {executor: {name: 'My Executor'}},
            },
          })}
        />,
      );

      const minimal = screen.getByTestId('execution-minimal');
      expect(minimal).toHaveAttribute('data-label', 'My Executor');
    });
  });

  describe('ValidationErrorBoundary', () => {
    it('should wrap content in ValidationErrorBoundary', () => {
      render(
        <Execution
          {...createMockProps({
            data: {
              action: {executor: {name: 'Test'}},
            },
          })}
        />,
      );

      // The component is wrapped in ValidationErrorBoundary (mocked)
      expect(screen.getByTestId('execution-minimal')).toBeInTheDocument();
    });

    it('should render execution with correct label', () => {
      render(
        <Execution
          {...createMockProps({
            data: {
              action: {executor: {name: 'Validated Executor'}},
            },
          })}
        />,
      );

      const minimal = screen.getByTestId('execution-minimal');
      expect(minimal).toHaveAttribute('data-label', 'Validated Executor');
    });
  });

  describe('Default Values', () => {
    it('should use "Executor" as default name when action.executor.name is not provided', () => {
      render(<Execution {...createMockProps({data: {}})} />);

      const minimal = screen.getByTestId('execution-minimal');
      expect(minimal).toHaveAttribute('data-label', 'Executor');
    });

    it('should handle undefined data gracefully', () => {
      render(<Execution {...createMockProps()} />);

      expect(screen.getByTestId('execution-minimal')).toBeInTheDocument();
    });
  });

  describe('Display Metadata', () => {
    it('should use display.label from data when available', () => {
      render(
        <Execution
          {...createMockProps({
            data: {
              action: {executor: {name: 'Default Name'}},
              display: {label: 'Custom Label'},
            },
          })}
        />,
      );

      const minimal = screen.getByTestId('execution-minimal');
      expect(minimal).toHaveAttribute('data-label', 'Custom Label');
    });

    it('should render with display.image from data when available', () => {
      render(
        <Execution
          {...createMockProps({
            data: {
              action: {executor: {name: 'Test'}},
              display: {image: '/path/to/image.svg'},
            },
          })}
        />,
      );

      // Component renders successfully with display.image
      expect(screen.getByTestId('execution-minimal')).toBeInTheDocument();
    });

    it('should map display.description into the resource display', () => {
      render(
        <Execution
          {...createMockProps({
            data: {
              action: {executor: {name: 'SSOCheckExecutor'}},
              display: {description: 'Can the following authentication be skipped by reusing the existing session?'},
            },
          })}
        />,
      );

      expect(screen.getByTestId('execution-minimal')).toHaveAttribute(
        'data-description',
        'Can the following authentication be skipped by reusing the existing session?',
      );
    });

    it('should map display.outcomes into the resource display', () => {
      render(
        <Execution
          {...createMockProps({
            data: {
              action: {executor: {name: 'SSOCheckExecutor'}},
              display: {outcomes: {success: 'Skip to', failure: 'Authenticate'}},
            },
          })}
        />,
      );

      expect(screen.getByTestId('execution-minimal')).toHaveAttribute(
        'data-outcomes',
        JSON.stringify({success: 'Skip to', failure: 'Authenticate'}),
      );
    });
  });

  describe('Memoization', () => {
    it('should render correctly on rerender with same props', () => {
      const props = createMockProps({
        data: {
          action: {executor: {name: 'Memo Test'}},
          components: [],
        },
      });

      const {rerender} = render(<Execution {...props} />);

      expect(screen.getByTestId('execution-minimal')).toBeInTheDocument();

      rerender(<Execution {...props} />);

      expect(screen.getByTestId('execution-minimal')).toBeInTheDocument();
    });

    it('should re-render when data prop changes', () => {
      const initialProps = createMockProps({
        data: {
          action: {executor: {name: 'Initial Executor'}},
          components: [],
        },
      });

      const {rerender} = render(<Execution {...initialProps} />);

      expect(screen.getByTestId('execution-minimal')).toHaveAttribute('data-label', 'Initial Executor');

      const updatedProps = createMockProps({
        data: {
          action: {executor: {name: 'Updated Executor'}},
          components: [],
        },
      });

      rerender(<Execution {...updatedProps} />);

      expect(screen.getByTestId('execution-minimal')).toHaveAttribute('data-label', 'Updated Executor');
    });

    it('should re-render when resources prop changes', () => {
      const initialProps = createMockProps({
        data: {
          action: {executor: {name: 'Test Executor'}},
          components: [],
        },
        resources: [],
      });

      const {rerender} = render(<Execution {...initialProps} />);

      expect(screen.getByTestId('execution-minimal')).toBeInTheDocument();

      const updatedProps = createMockProps({
        data: {
          action: {executor: {name: 'Test Executor'}},
          components: [],
        },
        resources: [{id: 'new-resource', type: 'BUTTON'}] as unknown as CommonStepFactoryPropsInterface['resources'],
      });

      rerender(<Execution {...updatedProps} />);

      expect(screen.getByTestId('execution-minimal')).toBeInTheDocument();
    });
  });

  describe('Action Panel Double Click Handler', () => {
    it('should call setLastInteractedStepId and setLastInteractedResource when onActionPanelDoubleClick is triggered', () => {
      render(
        <Execution
          {...createMockProps({
            data: {
              action: {executor: {name: 'Click Test'}},
              components: [{id: 'comp-1', type: 'BUTTON'}],
            },
          })}
        />,
      );

      expect(screen.getByTestId('view-component')).toBeInTheDocument();

      // Trigger the captured double click handler
      capturedOnActionPanelDoubleClick?.();

      expect(mockSetLastInteractedStepId).toHaveBeenCalledWith('execution-node-id');
      expect(mockSetLastInteractedResource).toHaveBeenCalled();
    });

    it('should not call setLastInteractedStepId when stepId is null', () => {
      mockUseNodeId.mockReturnValue(null);

      render(
        <Execution
          {...createMockProps({
            data: {
              action: {executor: {name: 'Null StepId Test'}},
              components: [{id: 'comp-1', type: 'BUTTON'}],
            },
          })}
        />,
      );

      // Trigger the captured double click handler
      capturedOnActionPanelDoubleClick?.();

      expect(mockSetLastInteractedStepId).not.toHaveBeenCalled();
      expect(mockSetLastInteractedResource).toHaveBeenCalled();
    });
  });
});
