// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {fireEvent, render, screen} from '@thunderid/test-utils';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import SetDefaultEmailSenderDialog from '../SetDefaultEmailSenderDialog';

const mockShowToast = vi.fn();
const mockMutate = vi.fn();

vi.mock('@thunderid/contexts', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/contexts')>()),
  useToast: () => ({showToast: mockShowToast}),
}));
vi.mock('../../api/useSetDefaultEmailSender', () => ({
  default: () => ({mutate: mockMutate, isPending: false}),
}));

describe('SetDefaultEmailSenderDialog', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders the title and the sender name', () => {
    render(
      <SetDefaultEmailSenderDialog
        open
        senderId="sm1"
        senderName="Corp SMTP"
        replacesDefault={false}
        onClose={vi.fn()}
      />,
    );

    expect(screen.getByText('Set default email sender')).toBeInTheDocument();
    expect(screen.getByText('Corp SMTP').tagName).toBe('STRONG');
    expect(screen.getByText(/will be used as the default sender/)).toHaveTextContent(
      'Corp SMTP will be used as the default sender for system-generated emails, such as account lock notifications.',
    );
    expect(screen.queryByText('This will replace the current default email sender.')).not.toBeInTheDocument();
  });

  it('says the current default is replaced when one exists', () => {
    render(
      <SetDefaultEmailSenderDialog open senderId="sm1" senderName="Corp SMTP" replacesDefault onClose={vi.fn()} />,
    );

    expect(screen.getByText('This will replace the current default email sender.')).toBeInTheDocument();
  });

  it('does not render when closed', () => {
    render(
      <SetDefaultEmailSenderDialog
        open={false}
        senderId="sm1"
        senderName="Corp SMTP"
        replacesDefault={false}
        onClose={vi.fn()}
      />,
    );

    expect(screen.queryByText('Set default email sender')).not.toBeInTheDocument();
  });

  it('mutates with the sender id when confirmed', () => {
    render(
      <SetDefaultEmailSenderDialog
        open
        senderId="sm1"
        senderName="Corp SMTP"
        replacesDefault={false}
        onClose={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByRole('button', {name: 'Set as default'}));

    expect(mockMutate).toHaveBeenCalledWith({defaultEmailSenderId: 'sm1'}, expect.any(Object));
  });

  it('shows a success toast and closes on success', () => {
    mockMutate.mockImplementation((_vars, opts: {onSuccess: () => void}) => opts.onSuccess());
    const onClose = vi.fn();

    render(
      <SetDefaultEmailSenderDialog
        open
        senderId="sm1"
        senderName="Corp SMTP"
        replacesDefault={false}
        onClose={onClose}
      />,
    );
    fireEvent.click(screen.getByRole('button', {name: 'Set as default'}));

    expect(mockShowToast).toHaveBeenCalledWith('Corp SMTP is now the default email sender.', 'success');
    expect(onClose).toHaveBeenCalled();
  });

  it('shows the catalog message inline, never the raw server text, and stays open on failure', () => {
    const rawServerMessage = 'raw backend set-default failure detail';
    mockMutate.mockImplementation((_vars, opts: {onError: (err: Error) => void}) =>
      opts.onError(new Error(rawServerMessage)),
    );
    const onClose = vi.fn();

    render(
      <SetDefaultEmailSenderDialog
        open
        senderId="sm1"
        senderName="Corp SMTP"
        replacesDefault={false}
        onClose={onClose}
      />,
    );
    fireEvent.click(screen.getByRole('button', {name: 'Set as default'}));

    expect(screen.getByText('Failed to set the default email sender.')).toBeInTheDocument();
    expect(screen.queryByText(rawServerMessage)).not.toBeInTheDocument();
    expect(mockShowToast).not.toHaveBeenCalled();
    expect(onClose).not.toHaveBeenCalled();
  });

  it('clears the previous error when the user retries', () => {
    mockMutate
      .mockImplementationOnce((_vars, opts: {onError: (err: Error) => void}) => opts.onError(new Error('nope')))
      .mockImplementationOnce(() => undefined);

    render(
      <SetDefaultEmailSenderDialog
        open
        senderId="sm1"
        senderName="Corp SMTP"
        replacesDefault={false}
        onClose={vi.fn()}
      />,
    );
    fireEvent.click(screen.getByRole('button', {name: 'Set as default'}));
    expect(screen.getByText('Failed to set the default email sender.')).toBeInTheDocument();

    // The retry stays in flight, so only the start of the request can clear the error.
    fireEvent.click(screen.getByRole('button', {name: 'Set as default'}));
    expect(mockMutate).toHaveBeenCalledTimes(2);
    expect(screen.queryByText('Failed to set the default email sender.')).not.toBeInTheDocument();
  });

  it('closes when Cancel is clicked after a failure', () => {
    mockMutate.mockImplementation((_vars, opts: {onError: (err: Error) => void}) => opts.onError(new Error('nope')));
    const onClose = vi.fn();

    render(
      <SetDefaultEmailSenderDialog
        open
        senderId="sm1"
        senderName="Corp SMTP"
        replacesDefault={false}
        onClose={onClose}
      />,
    );
    fireEvent.click(screen.getByRole('button', {name: 'Set as default'}));
    fireEvent.click(screen.getByRole('button', {name: 'Cancel'}));

    expect(onClose).toHaveBeenCalled();
  });
});
