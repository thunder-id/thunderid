// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import {render, screen, waitFor} from '@thunderid/test-utils';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import CredentialResetSection from '../CredentialResetSection';

const mockMutate = vi.fn();
const mockReset = vi.fn();
const mockMutation = {mutate: mockMutate, reset: mockReset, isPending: false, error: null as Error | null};
const mockCan = vi.fn().mockReturnValue(true);

vi.mock('../../../../api/useUpdateAgentCredentials', () => ({default: () => mockMutation}));

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {...actual, useCapabilitiesContext: () => ({can: mockCan})};
});

const fields = [{fieldName: 'password', label: 'Password'}];

describe('CredentialResetSection', () => {
  beforeEach(() => {
    mockMutate.mockReset();
    mockReset.mockReset();
    mockCan.mockReset().mockReturnValue(true);
    mockMutation.error = null;
    mockMutation.isPending = false;
  });

  it('renders nothing when the agent type declares no credential', () => {
    const {container} = render(<CredentialResetSection agentId="agent-1" fields={[]} />);

    expect(container).toBeEmptyDOMElement();
  });

  it('offers a reset button for each credential', () => {
    render(<CredentialResetSection agentId="agent-1" fields={[...fields, {fieldName: 'pin', label: 'PIN'}]} />);

    expect(screen.getByRole('button', {name: 'Reset Password'})).toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'Reset PIN'})).toBeInTheDocument();
  });

  it('disables the reset for a read-only agent', () => {
    render(<CredentialResetSection agentId="agent-1" fields={fields} disabled />);

    expect(screen.getByRole('button', {name: 'Reset Password'})).toBeDisabled();
  });

  it('disables the reset without permission to update agents', () => {
    mockCan.mockReturnValue(false);

    render(<CredentialResetSection agentId="agent-1" fields={fields} />);

    expect(screen.getByRole('button', {name: 'Reset Password'})).toBeDisabled();
  });

  it('keeps the confirm button off until a value is entered', async () => {
    const user = userEvent.setup();
    render(<CredentialResetSection agentId="agent-1" fields={fields} />);

    await user.click(screen.getByRole('button', {name: 'Reset Password'}));

    expect(await screen.findByText('Reset Password?')).toBeInTheDocument();
    expect(screen.getAllByRole('button', {name: 'Reset Password'}).at(-1)).toBeDisabled();
  });

  it('refuses mismatched values without calling the API', async () => {
    const user = userEvent.setup();
    render(<CredentialResetSection agentId="agent-1" fields={fields} />);

    await user.click(screen.getByRole('button', {name: 'Reset Password'}));
    await user.type(await screen.findByPlaceholderText('Enter new password'), 'one');
    await user.type(screen.getByPlaceholderText('Confirm new password'), 'two');
    await user.click(screen.getAllByRole('button', {name: 'Reset Password'}).at(-1)!);

    expect(screen.getByText('Values do not match.')).toBeInTheDocument();
    expect(mockMutate).not.toHaveBeenCalled();
  });

  it('posts the new value for the agent and closes on success', async () => {
    const user = userEvent.setup();
    mockMutate.mockImplementation((_vars: unknown, options?: {onSuccess?: () => void}) => options?.onSuccess?.());
    render(<CredentialResetSection agentId="agent-1" fields={fields} />);

    await user.click(screen.getByRole('button', {name: 'Reset Password'}));
    await user.type(await screen.findByPlaceholderText('Enter new password'), 'N3w-pass!');
    await user.type(screen.getByPlaceholderText('Confirm new password'), 'N3w-pass!');
    await user.click(screen.getAllByRole('button', {name: 'Reset Password'}).at(-1)!);

    expect(mockMutate).toHaveBeenCalledWith(
      {agentId: 'agent-1', data: {credentials: {password: 'N3w-pass!'}}},
      expect.objectContaining({onSuccess: expect.any(Function) as () => void}),
    );
    await waitFor(() => {
      expect(screen.queryByText('Reset Password?')).not.toBeInTheDocument();
    });
  });

  it('shows the error when the reset fails', async () => {
    const user = userEvent.setup();
    mockMutation.error = new Error('boom');
    render(<CredentialResetSection agentId="agent-1" fields={fields} />);

    await user.click(screen.getByRole('button', {name: 'Reset Password'}));

    expect(await screen.findByText('Failed to update credentials. Please try again.')).toBeInTheDocument();
  });
});
