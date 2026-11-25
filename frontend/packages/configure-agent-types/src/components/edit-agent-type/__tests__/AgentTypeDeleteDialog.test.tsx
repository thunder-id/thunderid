// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/* eslint-disable @typescript-eslint/no-unsafe-assignment, @typescript-eslint/no-unsafe-return, @typescript-eslint/no-explicit-any */
import {render, screen, waitFor, userEvent} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import AgentTypeDeleteDialog from '../AgentTypeDeleteDialog';

const mockMutate = vi.fn();
const mockUseDeleteAgentType = vi.fn<() => any>();

vi.mock('../../../api/useDeleteAgentType', () => ({
  default: () => mockUseDeleteAgentType(),
}));

const apiError = (code: string): Error => Object.assign(new Error('Request failed'), {response: {data: {code}}});

describe('AgentTypeDeleteDialog', () => {
  const defaultProps = {
    open: true,
    agentTypeId: 'agent-type-123',
    onClose: vi.fn(),
    onSuccess: vi.fn(),
  };

  beforeEach(() => {
    vi.clearAllMocks();
    mockUseDeleteAgentType.mockReturnValue({
      mutate: mockMutate,
      isPending: false,
      error: null,
      reset: vi.fn(),
    });
  });

  it('should render the confirmation and the warning that it cannot be undone', () => {
    render(<AgentTypeDeleteDialog {...defaultProps} />);

    expect(screen.getByRole('dialog', {name: 'Delete agent type'})).toBeInTheDocument();
    expect(screen.getByText(/are you sure you want to delete this agent type/i)).toBeInTheDocument();
    expect(screen.getByText(/all associated schema definitions will be permanently removed/i)).toBeInTheDocument();
  });

  it('should call onClose when cancel is clicked', async () => {
    const user = userEvent.setup();
    render(<AgentTypeDeleteDialog {...defaultProps} />);

    await user.click(screen.getByRole('button', {name: 'Cancel'}));

    expect(defaultProps.onClose).toHaveBeenCalled();
    expect(mockMutate).not.toHaveBeenCalled();
  });

  it('should disable delete when no agent type is selected', () => {
    render(<AgentTypeDeleteDialog {...defaultProps} agentTypeId={null} />);

    expect(screen.getByRole('button', {name: 'Delete'})).toBeDisabled();
  });

  it('should delete the selected agent type on confirm', async () => {
    const user = userEvent.setup();
    render(<AgentTypeDeleteDialog {...defaultProps} />);

    await user.click(screen.getByRole('button', {name: 'Delete'}));

    expect(mockMutate).toHaveBeenCalledWith(
      'agent-type-123',
      expect.objectContaining({onSuccess: expect.any(Function), onError: expect.any(Function)}),
    );
  });

  it('should close and call onSuccess once the delete succeeds', async () => {
    const user = userEvent.setup();
    mockMutate.mockImplementation((_id: string, options: {onSuccess: () => void}) => {
      options.onSuccess();
    });

    render(<AgentTypeDeleteDialog {...defaultProps} />);

    await user.click(screen.getByRole('button', {name: 'Delete'}));

    await waitFor(() => {
      expect(defaultProps.onClose).toHaveBeenCalled();
      expect(defaultProps.onSuccess).toHaveBeenCalled();
    });
  });

  it('should keep the dialog open and explain why when the type is still in use', async () => {
    const user = userEvent.setup();
    mockMutate.mockImplementation((_id: string, options: {onError: (err: Error) => void}) => {
      options.onError(apiError('USRS-1022'));
    });

    render(<AgentTypeDeleteDialog {...defaultProps} />);

    await user.click(screen.getByRole('button', {name: 'Delete'}));

    expect(
      await screen.findByText(
        'This agent type is in use. Delete or change the agents of this type, and remove it from the allowed agent types of applications, agents, and flows, before deleting it.',
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole('dialog')).toBeInTheDocument();
    expect(defaultProps.onClose).not.toHaveBeenCalled();
    expect(defaultProps.onSuccess).not.toHaveBeenCalled();
  });

  it('should explain a declarative agent type cannot be deleted', async () => {
    const user = userEvent.setup();
    mockMutate.mockImplementation((_id: string, options: {onError: (err: Error) => void}) => {
      options.onError(apiError('USRS-1008'));
    });

    render(<AgentTypeDeleteDialog {...defaultProps} />);

    await user.click(screen.getByRole('button', {name: 'Delete'}));

    expect(
      await screen.findByText('This agent type is managed by the system and cannot be modified or deleted.'),
    ).toBeInTheDocument();
  });

  it('should fall back to a generic message for an unknown failure', async () => {
    const user = userEvent.setup();
    mockMutate.mockImplementation((_id: string, options: {onError: (err: Error) => void}) => {
      options.onError(new Error('Delete failed'));
    });

    render(<AgentTypeDeleteDialog {...defaultProps} />);

    await user.click(screen.getByRole('button', {name: 'Delete'}));

    expect(await screen.findByText('Failed to delete agent type. Please try again.')).toBeInTheDocument();
    expect(screen.queryByText('Delete failed')).not.toBeInTheDocument();
  });

  it('should clear the error when the dialog is cancelled', async () => {
    const user = userEvent.setup();
    mockMutate.mockImplementation((_id: string, options: {onError: (err: Error) => void}) => {
      options.onError(apiError('USRS-1022'));
    });

    const {rerender} = render(<AgentTypeDeleteDialog {...defaultProps} />);

    await user.click(screen.getByRole('button', {name: 'Delete'}));
    expect(await screen.findByText(/this agent type is in use/i)).toBeInTheDocument();
    await user.click(screen.getByRole('button', {name: 'Cancel'}));
    rerender(<AgentTypeDeleteDialog {...defaultProps} />);

    expect(screen.queryByText(/this agent type is in use/i)).not.toBeInTheDocument();
  });

  it('should show the deleting state and block closing while the delete runs', () => {
    mockUseDeleteAgentType.mockReturnValue({
      mutate: mockMutate,
      isPending: true,
      error: null,
      reset: vi.fn(),
    });

    render(<AgentTypeDeleteDialog {...defaultProps} />);

    expect(screen.getByRole('button', {name: 'Deleting...'})).toBeDisabled();
    expect(screen.getByRole('button', {name: 'Cancel'})).toBeDisabled();
  });
});
