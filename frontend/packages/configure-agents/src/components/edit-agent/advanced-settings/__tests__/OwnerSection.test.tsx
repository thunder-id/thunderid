// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import {AdministrationModes, AdministrationProvider} from '@thunderid/contexts';
import {render, screen, within, waitFor} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {Agent} from '../../../../models/agent';
import OwnerSection from '../OwnerSection';

const {mockUseGetUsers, mockSendVerification, mockUseGetHandle} = vi.hoisted(() => ({
  mockUseGetUsers: vi.fn(),
  mockSendVerification: vi.fn(),
  mockUseGetHandle: vi.fn(),
}));

vi.mock('../../../../api/useSendOwnerVerification', () => ({
  default: () => ({mutate: mockSendVerification, isPending: false, isError: false, reset: vi.fn()}),
}));

vi.mock('@thunderid/configure-users', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/configure-users')>()),
  useGetUsers: (...args: unknown[]): unknown => mockUseGetUsers(...args) as unknown,
  useGetAdministrationFlowHandle: (...args: unknown[]): unknown => mockUseGetHandle(...args) as unknown,
}));

const OWNER_FLOW = 'default-agent-owner-verification-flow';

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
    mockUseGetHandle.mockReturnValue({data: OWNER_FLOW});
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
    render(<OwnerSection agent={mockAgent} editedAgent={{}} onFieldChange={mockOnFieldChange} readOnly />);

    expect(screen.getByRole('combobox')).toHaveAttribute('aria-disabled', 'true');
  });

  it('locks the owner of a personal agent and says why', () => {
    render(
      <OwnerSection
        agent={{...mockAgent, kind: 'personal-assistant'}}
        editedAgent={{}}
        onFieldChange={mockOnFieldChange}
      />,
    );

    expect(screen.getByRole('combobox')).toHaveAttribute('aria-disabled', 'true');
    expect(screen.getByRole('radio', {name: 'Internal'})).toBeDisabled();
    expect(screen.getByRole('radio', {name: 'External'})).toBeDisabled();
    expect(
      screen.getByText('The user this personal agent acts for. It keeps the owner it was registered for.'),
    ).toBeInTheDocument();
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
    render(<OwnerSection agent={mockAgent} editedAgent={{}} onFieldChange={mockOnFieldChange} readOnly />);

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

  describe('owner verification', () => {
    const emailOwner = {id: 'jane@example.com', idType: 'email', isLocal: false, verified: false} as const;

    it('shows an unverified external owner and sends it a verification link', async () => {
      const user = userEvent.setup();
      render(
        <OwnerSection agent={{...mockAgent, owner: emailOwner}} editedAgent={{}} onFieldChange={mockOnFieldChange} />,
      );

      expect(screen.getByTestId('agent-owner-not-verified')).toHaveTextContent('Not verified');
      await user.click(screen.getByRole('button', {name: 'Send verification link'}));

      expect(mockSendVerification).toHaveBeenCalledWith({agentId: 'agent-1', handle: OWNER_FLOW}, expect.anything());
    });

    it('shows when the owner was verified and offers a new link', () => {
      render(
        <OwnerSection
          agent={{...mockAgent, owner: {...emailOwner, verifiedAt: '2026-11-09T10:12:00Z'}}}
          editedAgent={{}}
          onFieldChange={mockOnFieldChange}
        />,
      );

      expect(screen.getByTestId('agent-owner-verified')).toHaveTextContent('Verified on');
      expect(screen.getByRole('button', {name: 'Send a new verification link'})).toBeEnabled();
    });

    it('asks for a changed owner to be saved before a link is sent', () => {
      render(
        <OwnerSection
          agent={{...mockAgent, owner: emailOwner}}
          editedAgent={{owner: {...emailOwner, id: 'bob@example.com'}}}
          onFieldChange={mockOnFieldChange}
        />,
      );

      expect(screen.getByRole('button', {name: 'Send verification link'})).toBeDisabled();
      expect(screen.getByText('Save the new owner before you send a verification link.')).toBeInTheDocument();
      expect(screen.queryByTestId('agent-owner-not-verified')).not.toBeInTheDocument();
    });

    it('sends a link for an agent a control plane deployed, which is read only here', async () => {
      const user = userEvent.setup();
      render(
        <OwnerSection
          agent={{...mockAgent, owner: emailOwner, isReadOnly: true}}
          editedAgent={{}}
          onFieldChange={mockOnFieldChange}
        />,
      );

      await user.click(screen.getByRole('button', {name: 'Send verification link'}));

      expect(mockSendVerification).toHaveBeenCalledWith({agentId: 'agent-1', handle: OWNER_FLOW}, expect.anything());
    });

    it('sends a link to a mobile owner', async () => {
      const user = userEvent.setup();
      const mobileOwner = {id: '+94771234567', idType: 'mobile', isLocal: false, verified: false} as const;
      render(
        <OwnerSection agent={{...mockAgent, owner: mobileOwner}} editedAgent={{}} onFieldChange={mockOnFieldChange} />,
      );

      expect(screen.getByTestId('agent-owner-not-verified')).toHaveTextContent('Not verified');
      await user.click(screen.getByRole('button', {name: 'Send verification link'}));

      expect(mockSendVerification).toHaveBeenCalledWith({agentId: 'agent-1', handle: OWNER_FLOW}, expect.anything());
    });

    it.each([
      ['a local owner', {id: 'user-1', idType: 'uuid', isLocal: true}],
      ['an external subject', {id: 'sub-1', idType: 'uuid', isLocal: false}],
    ] as const)('offers no verification link for %s', (_name, owner) => {
      render(<OwnerSection agent={{...mockAgent, owner}} editedAgent={{}} onFieldChange={mockOnFieldChange} />);

      expect(screen.queryByTestId('agent-owner-send-verification')).not.toBeInTheDocument();
    });

    it('reads the flow the server configures for agent owner verification', () => {
      render(
        <OwnerSection agent={{...mockAgent, owner: emailOwner}} editedAgent={{}} onFieldChange={mockOnFieldChange} />,
      );

      expect(mockUseGetHandle).toHaveBeenCalledWith('adminAgentOwnerVerificationFlow', true);
    });

    it('offers no verification where agent administration does not run through flows', () => {
      render(
        <AdministrationProvider administration={{mode: AdministrationModes.NATIVE}}>
          <OwnerSection agent={{...mockAgent, owner: emailOwner}} editedAgent={{}} onFieldChange={mockOnFieldChange} />
        </AdministrationProvider>,
      );

      expect(mockUseGetHandle).toHaveBeenCalledWith('adminAgentOwnerVerificationFlow', false);
      expect(screen.queryByTestId('agent-owner-send-verification')).not.toBeInTheDocument();
      expect(screen.queryByTestId('agent-owner-not-verified')).not.toBeInTheDocument();
    });

    it('shows only the status when the server configures no flow for it', () => {
      mockUseGetHandle.mockReturnValue({data: ''});
      render(
        <OwnerSection agent={{...mockAgent, owner: emailOwner}} editedAgent={{}} onFieldChange={mockOnFieldChange} />,
      );

      expect(screen.queryByTestId('agent-owner-send-verification')).not.toBeInTheDocument();
      expect(screen.getByTestId('agent-owner-not-verified')).toBeInTheDocument();
      expect(screen.queryByText(/server configuration could not be read/)).not.toBeInTheDocument();
    });

    it('says so when the setting naming the flow cannot be read', () => {
      mockUseGetHandle.mockReturnValue({data: undefined, error: new Error('Network down')});
      render(
        <OwnerSection agent={{...mockAgent, owner: emailOwner}} editedAgent={{}} onFieldChange={mockOnFieldChange} />,
      );

      expect(screen.queryByTestId('agent-owner-send-verification')).not.toBeInTheDocument();
      expect(screen.getByTestId('agent-owner-not-verified')).toBeInTheDocument();
      expect(
        screen.getByText(
          'A verification link cannot be sent from here, because the server configuration could not be read.',
        ),
      ).toBeInTheDocument();
    });

    it('shows no verification for a declarative agent, for which the server states none', () => {
      render(
        <OwnerSection
          agent={{...mockAgent, owner: {id: 'jane@example.com', idType: 'email', isLocal: false}, isReadOnly: true}}
          editedAgent={{}}
          onFieldChange={mockOnFieldChange}
        />,
      );

      expect(mockUseGetHandle).toHaveBeenCalledWith('adminAgentOwnerVerificationFlow', false);
      expect(screen.queryByTestId('agent-owner-send-verification')).not.toBeInTheDocument();
      expect(screen.queryByTestId('agent-owner-not-verified')).not.toBeInTheDocument();
    });

    it('shows only the status, quietly, to an operator who may not read the server configuration', () => {
      mockUseGetHandle.mockReturnValue({data: undefined, error: {response: {status: 403}}});
      render(
        <OwnerSection agent={{...mockAgent, owner: emailOwner}} editedAgent={{}} onFieldChange={mockOnFieldChange} />,
      );

      expect(screen.queryByTestId('agent-owner-send-verification')).not.toBeInTheDocument();
      expect(screen.getByTestId('agent-owner-not-verified')).toBeInTheDocument();
      expect(screen.queryByText(/server configuration could not be read/)).not.toBeInTheDocument();
    });

    it('offers no link for an agent shared into this organization unit', () => {
      render(
        <OwnerSection
          agent={{...mockAgent, owner: emailOwner, isOwnedByOU: false, isReadOnly: true}}
          editedAgent={{}}
          onFieldChange={mockOnFieldChange}
        />,
      );

      expect(mockUseGetHandle).toHaveBeenCalledWith('adminAgentOwnerVerificationFlow', false);
      expect(screen.queryByTestId('agent-owner-send-verification')).not.toBeInTheDocument();
      expect(screen.getByTestId('agent-owner-not-verified')).toBeInTheDocument();
    });
  });
});
