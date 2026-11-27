// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import {act, fireEvent, render, screen, waitFor} from '@thunderid/test-utils';
import {FlowExecutionFailure} from '@thunderid/utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import {OWNER_VERIFICATION_POLL_LIMIT_MS, OWNER_VERIFICATION_POLL_MS} from '../../../../constants/agent-constants';
import type {Agent} from '../../../../models/agent';
import OwnerVerificationPanel from '../OwnerVerificationPanel';

const {mockMutate, mockReset, mockInvalidate, sendState} = vi.hoisted(() => ({
  mockMutate: vi.fn(),
  mockReset: vi.fn(),
  mockInvalidate: vi.fn(),
  sendState: {isPending: false, isError: false, error: null as Error | null},
}));

vi.mock('../../../../api/useSendOwnerVerification', () => ({
  default: () => ({mutate: mockMutate, reset: mockReset, ...sendState}),
}));

vi.mock('@tanstack/react-query', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@tanstack/react-query')>()),
  useQueryClient: () => ({invalidateQueries: mockInvalidate}),
}));

const emailOwner = {id: 'jane@example.com', idType: 'email', isLocal: false, verified: false} as const;

const agentWith = (owner: Agent['owner']): Agent => ({id: 'agent-1', ouId: 'ou-1', type: 'default', owner}) as Agent;

/** Makes the mocked send succeed with the masked destination the flow answers with. */
const sendSucceeds = (destination: string): void => {
  mockMutate.mockImplementation((_vars: unknown, options?: {onSuccess?: (data: unknown) => void}) =>
    options?.onSuccess?.({data: {additionalData: {maskedDestination: destination}}}),
  );
};

describe('OwnerVerificationPanel', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    Object.assign(sendState, {isPending: false, isError: false, error: null});
    mockInvalidate.mockResolvedValue(undefined);
  });

  it('says how the owner is reached and sends the link', async () => {
    const user = userEvent.setup();
    render(<OwnerVerificationPanel agent={agentWith(emailOwner)} ownerUnsaved={false} handle="owner-flow" />);

    expect(screen.getByText(/They get a link by email/)).toBeInTheDocument();
    expect(screen.getByTestId('agent-owner-not-verified')).toHaveTextContent('Not verified');
    await user.click(screen.getByRole('button', {name: 'Send verification link'}));

    expect(mockMutate).toHaveBeenCalledWith({agentId: 'agent-1', handle: 'owner-flow'}, expect.anything());
  });

  it('says a mobile owner gets a text message', () => {
    render(
      <OwnerVerificationPanel
        agent={agentWith({id: '+94771234567', idType: 'mobile', isLocal: false, verified: false})}
        ownerUnsaved={false}
        handle="owner-flow"
      />,
    );

    expect(screen.getByText(/They get a link by text message/)).toBeInTheDocument();
  });

  it('shows a spinner on the button while the link is sent', () => {
    sendState.isPending = true;
    render(<OwnerVerificationPanel agent={agentWith(emailOwner)} ownerUnsaved={false} handle="owner-flow" />);

    const button = screen.getByTestId('agent-owner-send-verification');
    expect(button).toBeDisabled();
    expect(button).toHaveTextContent('Sending...');
    expect(screen.getByRole('progressbar', {name: 'Sending the verification link'})).toBeInTheDocument();
  });

  it('waits for the owner once the link is sent, and checks on request', async () => {
    const user = userEvent.setup();
    sendSucceeds('j•••@example.com');
    render(<OwnerVerificationPanel agent={agentWith(emailOwner)} ownerUnsaved={false} handle="owner-flow" />);

    await user.click(screen.getByRole('button', {name: 'Send verification link'}));

    const pending = screen.getByTestId('agent-owner-verification-pending');
    expect(pending).toHaveTextContent('Link sent to j•••@example.com.');
    expect(pending).toHaveTextContent('This page updates when they do.');
    // The link is already sent, so nothing spins while the page waits for the owner.
    expect(screen.queryByRole('progressbar')).not.toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'Resend link'})).toBeEnabled();

    await user.click(screen.getByRole('button', {name: 'Check now'}));
    expect(mockInvalidate).toHaveBeenCalledWith({queryKey: ['agent', 'agent-1']});
  });

  it('reads the agent again while it waits, and stops once the owner has had long enough', () => {
    vi.useFakeTimers();
    try {
      sendSucceeds('j•••@example.com');
      render(<OwnerVerificationPanel agent={agentWith(emailOwner)} ownerUnsaved={false} handle="owner-flow" />);

      fireEvent.click(screen.getByRole('button', {name: 'Send verification link'}));
      expect(mockInvalidate).not.toHaveBeenCalled();

      act(() => {
        vi.advanceTimersByTime(OWNER_VERIFICATION_POLL_MS);
      });
      expect(mockInvalidate).toHaveBeenCalledTimes(1);
      expect(mockInvalidate).toHaveBeenCalledWith({queryKey: ['agent', 'agent-1']});

      act(() => {
        vi.advanceTimersByTime(OWNER_VERIFICATION_POLL_MS);
      });
      expect(mockInvalidate).toHaveBeenCalledTimes(2);

      act(() => {
        vi.advanceTimersByTime(OWNER_VERIFICATION_POLL_LIMIT_MS);
      });
      const pending = screen.getByTestId('agent-owner-verification-pending');
      expect(pending).toHaveTextContent('The owner has not confirmed yet. Check again later, or resend the link.');
      expect(pending).not.toHaveTextContent('This page updates when they do.');

      const reads = mockInvalidate.mock.calls.length;
      act(() => {
        vi.advanceTimersByTime(OWNER_VERIFICATION_POLL_MS * 3);
      });
      expect(mockInvalidate).toHaveBeenCalledTimes(reads);
    } finally {
      vi.useRealTimers();
    }
  });

  it('shows the owner verified once they confirm', async () => {
    const user = userEvent.setup();
    sendSucceeds('j•••@example.com');
    const {rerender} = render(
      <OwnerVerificationPanel agent={agentWith(emailOwner)} ownerUnsaved={false} handle="owner-flow" />,
    );
    await user.click(screen.getByRole('button', {name: 'Send verification link'}));

    rerender(
      <OwnerVerificationPanel
        agent={agentWith({...emailOwner, verified: true, verifiedAt: '2026-11-17T06:19:41Z'})}
        ownerUnsaved={false}
        handle="owner-flow"
      />,
    );

    await waitFor(() => expect(screen.getByText('The owner confirmed ownership of this agent')).toBeInTheDocument());
    expect(screen.queryByTestId('agent-owner-verification-pending')).not.toBeInTheDocument();
    expect(screen.getByTestId('agent-owner-verified')).toHaveTextContent(
      `Verified on ${new Date('2026-11-17T06:19:41Z').toLocaleString(undefined, {dateStyle: 'medium', timeStyle: 'short'})}`,
    );
    expect(screen.getByRole('button', {name: 'Send a new verification link'})).toBeEnabled();
  });

  it.each([
    ['SUSPENDED', 'Unsuspend the agent to send its owner a verification link.'],
    ['EXPIRED', "This agent's lifespan has ended, so its owner cannot be sent a verification link."],
  ] as const)('offers no link for a %s agent, which the flow refuses, and says why', (value, hint) => {
    render(
      <OwnerVerificationPanel
        agent={{...agentWith(emailOwner), status: {value}}}
        ownerUnsaved={false}
        handle="owner-flow"
      />,
    );

    expect(screen.getByText(hint)).toBeInTheDocument();
    expect(screen.getByTestId('agent-owner-send-verification')).toBeDisabled();
  });

  it('clears a failed send once the owner is edited', () => {
    Object.assign(sendState, {isError: true, error: new Error('Network down')});
    const {rerender} = render(
      <OwnerVerificationPanel agent={agentWith(emailOwner)} ownerUnsaved={false} handle="owner-flow" />,
    );
    expect(mockReset).not.toHaveBeenCalled();

    rerender(<OwnerVerificationPanel agent={agentWith(emailOwner)} ownerUnsaved handle="owner-flow" />);

    expect(mockReset).toHaveBeenCalled();
  });

  it('says what to do when the flow has no email sender', () => {
    Object.assign(sendState, {
      isError: true,
      error: new FlowExecutionFailure({code: 'FET-1038'}, 'Email provider is not configured'),
    });
    render(<OwnerVerificationPanel agent={agentWith(emailOwner)} ownerUnsaved={false} handle="owner-flow" />);

    expect(screen.getByRole('alert')).toHaveTextContent('The flow has no usable email provider on its Send Email step');
  });

  it('asks for a changed owner to be saved before a link is sent', () => {
    render(<OwnerVerificationPanel agent={agentWith(emailOwner)} ownerUnsaved handle="owner-flow" />);

    expect(screen.getByText('Save the new owner before you send a verification link.')).toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'Send verification link'})).toBeDisabled();
    expect(screen.queryByTestId('agent-owner-not-verified')).not.toBeInTheDocument();
  });
});
