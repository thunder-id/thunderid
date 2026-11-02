// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, fireEvent} from '@testing-library/react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import CompactStacksContext from '../../../../../context/CompactStacksContext';
import type {Step} from '../../../../../models/steps';
import ExecutionCompact from '../ExecutionCompact';

// Mock @xyflow/react
const mockUseNodeId = vi.fn((): string | null => 'execution-node-id');

vi.mock('@xyflow/react', () => ({
  useNodeId: () => mockUseNodeId(),
  Handle: ({
    type,
    position,
    id = '',
    'data-handle': dataHandle = '',
  }: {
    type: string;
    position: string;
    id?: string;
    'data-handle'?: string;
  }) => (
    <div
      data-testid={`handle-${type}${dataHandle ? `-${dataHandle}` : ''}`}
      data-position={position}
      data-id={id}
      data-handle={dataHandle}
    />
  ),
  Position: {
    Left: 'left',
    Right: 'right',
    Top: 'top',
    Bottom: 'bottom',
  },
}));

// Mock useInteractionState
const mockSetLastInteractedResource = vi.fn();
const mockSetLastInteractedStepId = vi.fn();

vi.mock('../../../../../hooks/useInteractionState', () => ({
  default: () => ({
    setLastInteractedResource: mockSetLastInteractedResource,
    setLastInteractedStepId: mockSetLastInteractedStepId,
  }),
}));

// Mock useUIPanelState
const mockSetIsOpenResourcePropertiesPanel = vi.fn();

vi.mock('../../../../../hooks/useUIPanelState', () => ({
  default: () => ({
    setIsOpenResourcePropertiesPanel: mockSetIsOpenResourcePropertiesPanel,
  }),
}));

// Mock react-i18next
vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (key: string, fallback?: string) => fallback ?? key,
  }),
}));

// Mock ResourceDisplayImage
vi.mock('../../../../ResourceDisplayImage', () => ({
  default: ({image, label}: {image?: string; label?: string}) => (
    <div data-testid="resource-display-image" data-image={image} data-label={label} />
  ),
}));

// Mock VisualFlowConstants
vi.mock('../../../../../constants/VisualFlowConstants', () => ({
  default: {
    FLOW_BUILDER_NEXT_HANDLE_SUFFIX: '-next',
    FLOW_BUILDER_INCOMPLETE_HANDLE_SUFFIX: '-incomplete',
  },
}));

// Create mock resource
const createMockResource = (overrides: Partial<Step> = {}): Step =>
  ({
    id: 'execution-1',
    type: 'TASK_EXECUTION',
    position: {x: 0, y: 0},
    size: {width: 48, height: 48},
    display: {
      label: 'Test Executor',
      image: 'test-image.svg',
      showOnResourcePanel: true,
    },
    data: {
      action: {
        executor: {
          name: 'TestExecutor',
        },
      },
      config: {
        testConfig: 'value',
      },
    },
    config: {},
    ...overrides,
  }) as Step;

describe('ExecutionCompact', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseNodeId.mockReturnValue('execution-node-id');
  });

  describe('Rendering', () => {
    it('should render the chip with the display label as accessible name', () => {
      const resource = createMockResource();
      render(<ExecutionCompact resource={resource} />);

      expect(screen.getByRole('button', {name: 'Test Executor'})).toBeInTheDocument();
    });

    it('should render the executor icon when display.image is set', () => {
      const resource = createMockResource();
      render(<ExecutionCompact resource={resource} />);

      const image = screen.getByTestId('resource-display-image');
      expect(image).toHaveAttribute('data-image', 'test-image.svg');
    });

    it('should render the first letter of the label when there is no display image', () => {
      const resource = createMockResource({
        display: {label: 'Passkey Challenge', image: '', showOnResourcePanel: true},
      });
      render(<ExecutionCompact resource={resource} />);

      expect(screen.queryByTestId('resource-display-image')).not.toBeInTheDocument();
      expect(screen.getByText('P')).toBeInTheDocument();
    });

    it('should fallback to executor name when display.label is not provided', () => {
      const resource = createMockResource({
        display: undefined,
        data: {action: {executor: {name: 'FallbackExecutor'}}},
      });
      render(<ExecutionCompact resource={resource} />);

      expect(screen.getByRole('button', {name: 'FallbackExecutor'})).toBeInTheDocument();
    });

    it('should fallback to "Executor" when both display.label and executor name are missing', () => {
      const resource = createMockResource({display: undefined, data: {}});
      render(<ExecutionCompact resource={resource} />);

      expect(screen.getByRole('button', {name: 'Executor'})).toBeInTheDocument();
    });

    it('should reveal label and description in a tooltip on hover', async () => {
      const resource = createMockResource({
        display: {
          label: 'Check SSO Session',
          description: 'Can the following authentication be skipped?',
          image: '',
          showOnResourcePanel: true,
        },
      });
      render(<ExecutionCompact resource={resource} />);

      fireEvent.mouseOver(screen.getByRole('button', {name: 'Check SSO Session'}));

      expect(await screen.findByText('Check SSO Session')).toBeInTheDocument();
      expect(await screen.findByText('Can the following authentication be skipped?')).toBeInTheDocument();
    });
  });

  describe('Handles', () => {
    it('should render target handle on the left without an id', () => {
      const resource = createMockResource();
      render(<ExecutionCompact resource={resource} />);

      const targetHandle = screen.getByTestId('handle-target');
      expect(targetHandle).toHaveAttribute('data-position', 'left');
      expect(targetHandle).toHaveAttribute('data-id', '');
    });

    it('should render source handle on the right with the next-suffixed id', () => {
      const resource = createMockResource({id: 'test-execution'});
      render(<ExecutionCompact resource={resource} />);

      const sourceHandle = screen.getByTestId('handle-source');
      expect(sourceHandle).toHaveAttribute('data-position', 'right');
      expect(sourceHandle).toHaveAttribute('data-id', 'test-execution-next');
    });

    it('should render success and failure handles when onFailure property exists', () => {
      const resource = createMockResource({
        data: {
          action: {executor: {name: 'TestExecutor'}, onSuccess: '', onFailure: ''},
        },
      });
      render(<ExecutionCompact resource={resource} />);

      expect(screen.getByTestId('handle-source-execution-handle-success')).toBeInTheDocument();
      const failureHandle = screen.getByTestId('handle-source-execution-handle-failure');
      expect(failureHandle).toHaveAttribute('data-position', 'bottom');
      expect(failureHandle).toHaveAttribute('data-id', 'failure');
    });

    it('should not render failure handle when onFailure is not present', () => {
      const resource = createMockResource({
        data: {action: {executor: {name: 'TestExecutor'}, onSuccess: ''}},
      });
      render(<ExecutionCompact resource={resource} />);

      expect(screen.queryByTestId('handle-source-execution-handle-failure')).not.toBeInTheDocument();
    });

    it('should render incomplete handle on top with the incomplete-suffixed id when onIncomplete exists', () => {
      const resource = createMockResource({
        id: 'test-execution',
        data: {action: {executor: {name: 'TestExecutor'}, onIncomplete: ''}},
      });
      render(<ExecutionCompact resource={resource} />);

      const incompleteHandle = screen.getByTestId('handle-source-execution-handle-incomplete');
      expect(incompleteHandle).toHaveAttribute('data-position', 'top');
      expect(incompleteHandle).toHaveAttribute('data-id', 'test-execution-incomplete');
    });

    it('should not render incomplete handle when onIncomplete is missing', () => {
      const resource = createMockResource({
        data: {action: {executor: {name: 'TestExecutor'}, onSuccess: ''}},
      });
      render(<ExecutionCompact resource={resource} />);

      expect(screen.queryByTestId('handle-source-execution-handle-incomplete')).not.toBeInTheDocument();
    });

    it('should use custom outcome labels from display.outcomes', () => {
      const resource = createMockResource({
        display: {
          label: 'SSO Check',
          image: '',
          showOnResourcePanel: true,
          outcomes: {success: 'Available', failure: 'Unavailable'},
        },
        data: {
          action: {executor: {name: 'SSOCheckExecutor'}, onSuccess: '', onFailure: ''},
        },
      });
      render(<ExecutionCompact resource={resource} />);

      expect(screen.getByLabelText('Available')).toBeInTheDocument();
      expect(screen.getByLabelText('Unavailable')).toBeInTheDocument();
    });
  });

  describe('Chip Click', () => {
    it('should set last interacted step id when the chip is clicked', () => {
      const resource = createMockResource();
      render(<ExecutionCompact resource={resource} />);

      fireEvent.click(screen.getByRole('button', {name: 'Test Executor'}));

      expect(mockSetLastInteractedStepId).toHaveBeenCalledWith('execution-node-id');
    });

    it('should set last interacted resource with merged config when the chip is clicked', () => {
      const resource = createMockResource({
        config: {field: {name: 'test', type: 'TEXT'}, styles: {}} as unknown as Step['config'],
        data: {
          action: {executor: {name: 'Test'}},
          config: {dataConfig: 'dataValue'},
        },
      });
      render(<ExecutionCompact resource={resource} />);

      fireEvent.click(screen.getByRole('button', {name: 'Test Executor'}));

      expect(mockSetLastInteractedResource).toHaveBeenCalledWith(
        expect.objectContaining({
          config: expect.objectContaining({
            dataConfig: 'dataValue',
            field: {name: 'test', type: 'TEXT'},
          }) as Record<string, unknown>,
        }),
      );
    });

    it('should merge only the resource-level config when the node data has no config object', () => {
      const resource = createMockResource({
        config: {field: {name: 'test', type: 'TEXT'}, styles: {}} as unknown as Step['config'],
        data: {
          action: {executor: {name: 'Test'}},
        },
      });
      render(<ExecutionCompact resource={resource} />);

      fireEvent.click(screen.getByRole('button', {name: 'Test Executor'}));

      expect(mockSetLastInteractedResource).toHaveBeenCalledWith(
        expect.objectContaining({
          config: {field: {name: 'test', type: 'TEXT'}, styles: {}},
        }),
      );
    });

    it('should open the resource properties panel when the chip is clicked', () => {
      const resource = createMockResource();
      render(<ExecutionCompact resource={resource} />);

      fireEvent.click(screen.getByRole('button', {name: 'Test Executor'}));

      expect(mockSetIsOpenResourcePropertiesPanel).toHaveBeenCalledWith(true);
    });

    it('should open the properties panel on Enter and Space', () => {
      const resource = createMockResource();
      render(<ExecutionCompact resource={resource} />);
      const chip = screen.getByRole('button', {name: 'Test Executor'});

      // The chip is a div with role="button", so it has no native activation.
      fireEvent.keyDown(chip, {key: 'Enter'});
      expect(mockSetIsOpenResourcePropertiesPanel).toHaveBeenCalledTimes(1);

      fireEvent.keyDown(chip, {key: ' '});
      expect(mockSetIsOpenResourcePropertiesPanel).toHaveBeenCalledTimes(2);
    });

    it('should ignore other keys', () => {
      const resource = createMockResource();
      render(<ExecutionCompact resource={resource} />);

      fireEvent.keyDown(screen.getByRole('button', {name: 'Test Executor'}), {key: 'a'});

      expect(mockSetIsOpenResourcePropertiesPanel).not.toHaveBeenCalled();
    });

    it('should not set step id when useNodeId returns null', () => {
      mockUseNodeId.mockReturnValue(null);
      const resource = createMockResource();
      render(<ExecutionCompact resource={resource} />);

      fireEvent.click(screen.getByRole('button', {name: 'Test Executor'}));

      expect(mockSetLastInteractedStepId).not.toHaveBeenCalled();
      expect(mockSetIsOpenResourcePropertiesPanel).toHaveBeenCalledWith(true);
    });
  });

  describe('Restack Affordance', () => {
    const mockCollapseStack = vi.fn();

    const renderWithExpandedGroup = (headId: string) =>
      render(
        <CompactStacksContext.Provider
          value={{
            collapseStack: mockCollapseStack,
            expandStack: vi.fn(),
            expandedHeadIdToStackId: new Map([[headId, `execution-stack_${headId}`]]),
          }}
        >
          <ExecutionCompact resource={createMockResource()} />
        </CompactStacksContext.Provider>,
      );

    it('should not show the restack button outside an expanded group', () => {
      render(<ExecutionCompact resource={createMockResource()} />);

      expect(screen.queryByRole('button', {name: 'Restack executors'})).not.toBeInTheDocument();
    });

    it('should show the restack button on the head chip of an expanded group', () => {
      renderWithExpandedGroup('execution-node-id');

      expect(screen.getByRole('button', {name: 'Restack executors'})).toBeInTheDocument();
    });

    it('should collapse the stack without opening the properties panel when restack is clicked', () => {
      renderWithExpandedGroup('execution-node-id');

      fireEvent.click(screen.getByRole('button', {name: 'Restack executors'}));

      expect(mockCollapseStack).toHaveBeenCalledWith('execution-stack_execution-node-id');
      expect(mockSetIsOpenResourcePropertiesPanel).not.toHaveBeenCalled();
    });

    it('should not show the restack button on non-head chips', () => {
      renderWithExpandedGroup('some-other-node');

      expect(screen.queryByRole('button', {name: 'Restack executors'})).not.toBeInTheDocument();
    });
  });
});
