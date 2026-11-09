// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import {render, screen, within, waitFor} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {Agent} from '../../../../models/agent';
import OwnerSection from '../OwnerSection';

const {mockUseGetUsers} = vi.hoisted(() => ({
  mockUseGetUsers: vi.fn(),
}));

vi.mock('@thunderid/configure-users', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/configure-users')>()),
  useGetUsers: (...args: unknown[]): unknown => mockUseGetUsers(...args) as unknown,
}));

describe('OwnerSection', () => {
  const mockOnFieldChange = vi.fn();

  const mockAgent: Agent = {
    id: 'agent-1',
    ouId: 'ou-1',
    type: 'default',
    display: 'Test Agent',
    owner: {id: 'user-1', idType: 'uuid', isLocal: true},
  };

  beforeEach(() => {
    vi.clearAllMocks();
    mockUseGetUsers.mockReturnValue({
      data: {
        users: [
          {id: 'user-1', display: 'Alice'},
          {id: 'user-2', display: 'Bob'},
        ],
      },
      isLoading: false,
    });
  });

  it('renders the resolved owner label', () => {
    render(<OwnerSection agent={mockAgent} editedAgent={{}} onFieldChange={mockOnFieldChange} />);

    expect(screen.getByText('Alice')).toBeInTheDocument();
  });

  it('shows a placeholder when no owner is assigned', () => {
    render(
      <OwnerSection agent={{...mockAgent, owner: undefined}} editedAgent={{}} onFieldChange={mockOnFieldChange} />,
    );

    expect(screen.getByText('No owner assigned')).toBeInTheDocument();
  });

  it('prefers the edited owner over the saved one', () => {
    render(
      <OwnerSection
        agent={mockAgent}
        editedAgent={{owner: {id: 'user-2', idType: 'uuid', isLocal: true}}}
        onFieldChange={mockOnFieldChange}
      />,
    );

    expect(screen.getByText('Bob')).toBeInTheDocument();
  });

  it('opens a picker and calls onFieldChange when a new owner is selected', async () => {
    const user = userEvent.setup();
    render(<OwnerSection agent={mockAgent} editedAgent={{}} onFieldChange={mockOnFieldChange} />);

    await user.click(screen.getByRole('combobox'));

    const listbox = await screen.findByRole('listbox');
    await user.click(within(listbox).getByText('Bob'));

    await waitFor(() => {
      expect(mockOnFieldChange).toHaveBeenCalledWith('owner', {
        id: 'user-2',
        idType: 'uuid',
        isLocal: true,
      });
    });
  });

  it('disables the picker for read-only agents', () => {
    render(
      <OwnerSection agent={{...mockAgent, isReadOnly: true}} editedAgent={{}} onFieldChange={mockOnFieldChange} />,
    );

    expect(screen.getByRole('combobox')).toHaveAttribute('aria-disabled', 'true');
  });

  it('opens an external owner on External with its type and ID, ready to change', () => {
    render(
      <OwnerSection
        agent={{...mockAgent, owner: {id: 'alice@example.com', idType: 'email', isLocal: false}}}
        editedAgent={{}}
        onFieldChange={mockOnFieldChange}
      />,
    );

    expect(screen.getByRole('radio', {name: 'External'})).toBeChecked();
    expect(screen.getByRole('textbox')).toHaveValue('alice@example.com');
    expect(screen.getByRole('textbox')).toBeEnabled();
    expect(screen.getByRole('combobox')).toHaveTextContent('Email');
    expect(mockOnFieldChange).not.toHaveBeenCalled();
  });

  it('sets an external owner from the type and ID entered', async () => {
    const user = userEvent.setup();
    render(<OwnerSection agent={mockAgent} editedAgent={{}} onFieldChange={mockOnFieldChange} />);

    await user.click(screen.getByRole('radio', {name: 'External'}));
    await user.type(screen.getByRole('textbox'), 'jane@example.com');

    expect(mockOnFieldChange).toHaveBeenLastCalledWith('owner', {
      id: 'jane@example.com',
      idType: 'email',
      isLocal: false,
    });
  });

  it('sets a mobile number owner and says when the number is not in international format', async () => {
    const user = userEvent.setup();
    render(<OwnerSection agent={mockAgent} editedAgent={{}} onFieldChange={mockOnFieldChange} />);

    await user.click(screen.getByRole('radio', {name: 'External'}));
    await user.click(screen.getByRole('combobox'));
    await user.click(within(await screen.findByRole('listbox')).getByText('Mobile number'));
    await user.type(screen.getByRole('textbox'), '0771234567');

    expect(screen.getByText(/international format/)).toBeInTheDocument();
    expect(mockOnFieldChange).toHaveBeenLastCalledWith('owner', {id: '0771234567', idType: 'mobile', isLocal: false});
  });

  it('goes back to the saved owner when Internal is chosen again', async () => {
    const user = userEvent.setup();
    render(<OwnerSection agent={mockAgent} editedAgent={{}} onFieldChange={mockOnFieldChange} />);

    await user.click(screen.getByRole('radio', {name: 'External'}));
    await user.click(screen.getByRole('radio', {name: 'Internal'}));

    expect(mockOnFieldChange).toHaveBeenLastCalledWith('owner', undefined);
    expect(screen.getByText('Alice')).toBeInTheDocument();
  });

  it('replaces an external owner with a user of this deployment', async () => {
    const user = userEvent.setup();
    render(
      <OwnerSection
        agent={{...mockAgent, owner: {id: 'alice@example.com', idType: 'email', isLocal: false}}}
        editedAgent={{}}
        onFieldChange={mockOnFieldChange}
      />,
    );

    await user.click(screen.getByRole('radio', {name: 'Internal'}));
    expect(screen.getByText('No owner assigned')).toBeInTheDocument();
    await user.click(screen.getByRole('combobox'));
    await user.click(within(await screen.findByRole('listbox')).getByText('Bob'));

    expect(mockOnFieldChange).toHaveBeenLastCalledWith('owner', {id: 'user-2', idType: 'uuid', isLocal: true});
  });

  it('does not let the owner kind change on a read-only agent', () => {
    render(
      <OwnerSection agent={{...mockAgent, isReadOnly: true}} editedAgent={{}} onFieldChange={mockOnFieldChange} />,
    );

    expect(screen.getByRole('radio', {name: 'Internal'})).toBeDisabled();
    expect(screen.getByRole('radio', {name: 'External'})).toBeDisabled();
  });

  it('falls back to the username attribute when no display name is set', () => {
    mockUseGetUsers.mockReturnValue({
      data: {users: [{id: 'user-1', attributes: {username: 'alice.doe'}}]},
      isLoading: false,
    });

    render(<OwnerSection agent={mockAgent} editedAgent={{}} onFieldChange={mockOnFieldChange} />);

    expect(screen.getByText('alice.doe')).toBeInTheDocument();
  });

  it('falls back to the email attribute when there is no display name or username', () => {
    mockUseGetUsers.mockReturnValue({
      data: {users: [{id: 'user-1', attributes: {email: 'alice@example.com'}}]},
      isLoading: false,
    });

    render(<OwnerSection agent={mockAgent} editedAgent={{}} onFieldChange={mockOnFieldChange} />);

    expect(screen.getByText('alice@example.com')).toBeInTheDocument();
  });

  it('falls back to the user id when no display name, username, or email is available', () => {
    mockUseGetUsers.mockReturnValue({
      data: {users: [{id: 'user-1', attributes: {}}]},
      isLoading: false,
    });

    render(<OwnerSection agent={mockAgent} editedAgent={{}} onFieldChange={mockOnFieldChange} />);

    expect(screen.getByText('user-1')).toBeInTheDocument();
  });
});
