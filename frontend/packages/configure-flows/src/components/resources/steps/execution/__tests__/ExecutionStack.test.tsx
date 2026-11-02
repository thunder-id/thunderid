// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, fireEvent} from '@testing-library/react';
import type {NodeProps} from '@xyflow/react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import CompactStacksContext from '../../../../../context/CompactStacksContext';
import ExecutionStack from '../ExecutionStack';

// Mock @xyflow/react
const mockUseNodeId = vi.fn((): string | null => 'execution-stack_exec-a');

vi.mock('@xyflow/react', () => ({
  useNodeId: () => mockUseNodeId(),
  Handle: ({type, position, id = ''}: {type: string; position: string; id?: string}) => (
    <div data-testid={`handle-${type}`} data-position={position} data-id={id} />
  ),
  Position: {
    Left: 'left',
    Right: 'right',
    Top: 'top',
    Bottom: 'bottom',
  },
}));

// Mock react-i18next
vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (_key: string, fallback?: string) => fallback ?? _key,
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
    FLOW_BUILDER_COMPACT_EXECUTION_NODE_SIZE: 48,
    FLOW_BUILDER_NEXT_HANDLE_SUFFIX: '-next',
    FLOW_BUILDER_INCOMPLETE_HANDLE_SUFFIX: '-incomplete',
    FLOW_BUILDER_PREVIOUS_HANDLE_SUFFIX: '-previous',
  },
}));

// Mock useValidationStatus with a settable notification list
const mockNotifications: {hasResource: (id: string) => boolean; getType: () => string}[] = [];

vi.mock('../../../../../hooks/useValidationStatus', () => ({
  default: () => ({notifications: mockNotifications}),
}));

// Mock ValidationErrorBoundary so the resource it receives is observable
vi.mock('../../../../validation-panel/ValidationErrorBoundary', () => ({
  default: ({children, resource}: {children: React.ReactNode; resource: {id: string}}) => (
    <div data-testid="validation-error-boundary" data-resource-id={resource.id}>
      {children}
    </div>
  ),
}));

const mockExpandStack = vi.fn();
const mockCollapseStack = vi.fn();

const member = (id: string, label: string, extras: Record<string, unknown> = {}) => ({
  data: {
    action: {executor: {name: `${id}-executor`}},
    display: {label},
    ...extras,
  },
  id,
});

const renderStack = (members: ReturnType<typeof member>[]) =>
  render(
    <CompactStacksContext.Provider
      value={{
        collapseStack: mockCollapseStack,
        expandStack: mockExpandStack,
        expandedHeadIdToStackId: new Map<string, string>(),
      }}
    >
      <ExecutionStack {...({data: {memberIds: members.map((m) => m.id), members}} as unknown as NodeProps)} />
    </CompactStacksContext.Provider>,
  );

describe('ExecutionStack', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseNodeId.mockReturnValue('execution-stack_exec-a');
    mockNotifications.length = 0;
  });

  describe('Validation', () => {
    it('should surface a stacked member that has a notification', () => {
      mockNotifications.push({getType: () => 'ERROR', hasResource: (id: string) => id === 'exec-b'});

      renderStack([member('exec-a', 'One'), member('exec-b', 'Two')]);

      // The hidden member's error is raised onto the collapsed stack.
      expect(screen.getByTestId('validation-error-boundary')).toHaveAttribute('data-resource-id', 'exec-b');
    });

    it('should fall back to the stack id when no member has a notification', () => {
      renderStack([member('exec-a', 'One'), member('exec-b', 'Two')]);

      expect(screen.getByTestId('validation-error-boundary')).toHaveAttribute(
        'data-resource-id',
        'execution-stack_exec-a',
      );
    });
  });

  it('should render one chip with the member labels as accessible name', () => {
    renderStack([member('exec-a', 'Attribute Collector'), member('exec-b', 'Send OTP')]);

    expect(screen.getByRole('button', {name: 'Attribute Collector, Send OTP'})).toBeInTheDocument();
  });

  it('should render only the first member icon on the main chip', () => {
    const {container} = renderStack([
      member('exec-a', 'Attribute Collector', {display: {label: 'Attribute Collector', image: 'collector.svg'}}),
      member('exec-b', 'Send OTP', {display: {label: 'Send OTP', image: 'otp.svg'}}),
    ]);

    const chipImages = Array.from(
      container.querySelectorAll('[data-stack-chip] [data-testid="resource-display-image"]'),
    );
    expect(chipImages).toHaveLength(1);
    expect(chipImages[0]).toHaveAttribute('data-image', 'collector.svg');
  });

  it('should show a +N badge counting the stacked members after the first', () => {
    renderStack([member('a', 'One'), member('b', 'Two'), member('c', 'Three')]);

    expect(screen.getByTestId('execution-stack-badge')).toHaveTextContent('+2');
  });

  it('should not show a badge for a single member', () => {
    renderStack([member('a', 'One')]);

    expect(screen.queryByTestId('execution-stack-badge')).not.toBeInTheDocument();
  });

  it('should cap the deck layers while the badge keeps the full count', () => {
    const {container} = renderStack([
      member('a', 'One'),
      member('b', 'Two'),
      member('c', 'Three'),
      member('d', 'Four'),
      member('e', 'Five'),
    ]);

    expect(container.querySelectorAll('[data-stack-layer]')).toHaveLength(2);
    expect(screen.getByTestId('execution-stack-badge')).toHaveTextContent('+4');
  });

  it('should render a hover fan chip for every stacked member after the first', () => {
    const {container} = renderStack([member('a', 'One'), member('b', 'Two'), member('c', 'Three')]);

    expect(container.querySelectorAll('[data-stack-fan-chip]')).toHaveLength(2);
  });

  it('should expand the stack on click', () => {
    renderStack([member('exec-a', 'Attribute Collector'), member('exec-b', 'Send OTP')]);

    fireEvent.click(screen.getByRole('button', {name: 'Attribute Collector, Send OTP'}));

    expect(mockExpandStack).toHaveBeenCalledWith('execution-stack_exec-a', ['exec-a', 'exec-b']);
  });

  it('should expand on Enter and Space', () => {
    renderStack([member('exec-a', 'Attribute Collector'), member('exec-b', 'Send OTP')]);
    const chip = screen.getByRole('button', {name: 'Attribute Collector, Send OTP'});

    // The chip is a div with role="button", so it has no native activation.
    fireEvent.keyDown(chip, {key: 'Enter'});
    expect(mockExpandStack).toHaveBeenCalledTimes(1);

    fireEvent.keyDown(chip, {key: ' '});
    expect(mockExpandStack).toHaveBeenCalledTimes(2);
  });

  it('should ignore other keys', () => {
    renderStack([member('exec-a', 'Attribute Collector'), member('exec-b', 'Send OTP')]);

    fireEvent.keyDown(screen.getByRole('button', {name: 'Attribute Collector, Send OTP'}), {key: 'a'});

    expect(mockExpandStack).not.toHaveBeenCalled();
  });

  it('should not expand when the node id is unavailable', () => {
    mockUseNodeId.mockReturnValue(null);
    renderStack([member('exec-a', 'Attribute Collector'), member('exec-b', 'Send OTP')]);

    fireEvent.click(screen.getByRole('button', {name: 'Attribute Collector, Send OTP'}));

    expect(mockExpandStack).not.toHaveBeenCalled();
  });

  it('should render target and next-suffixed source handles', () => {
    renderStack([member('exec-a', 'One'), member('exec-b', 'Two')]);

    expect(screen.getByTestId('handle-target')).toHaveAttribute('data-position', 'left');
    const sourceHandle = screen.getByTestId('handle-source');
    expect(sourceHandle).toHaveAttribute('data-position', 'right');
    expect(sourceHandle).toHaveAttribute('data-id', 'execution-stack_exec-a-next');
  });
});
