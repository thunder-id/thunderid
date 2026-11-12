// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {AdministrationModes} from '@thunderid/contexts';
import {render, screen, userEvent} from '@thunderid/test-utils';
import {FlowExecutionFailure} from '@thunderid/utils';
import {describe, expect, it, vi, beforeEach} from 'vitest';
import SendVerificationLinkButton from '../SendVerificationLinkButton';

const mockMutate = vi.fn();
const mockShowToast = vi.fn();
let isPending = false;
let administrationOperation: unknown = AdministrationModes.FLOW;
let canUpdate = true;
let configuredHandle: string | undefined = 'custom-verification-flow';
const mockUseGetAttributeVerificationFlow = vi.fn();

vi.mock('@/api/useSendUserAttributeVerification', () => ({
  default: () => ({mutate: mockMutate, isPending}),
}));

vi.mock('@/api/useGetAttributeVerificationFlow', () => ({
  default: (enabled: boolean) => {
    mockUseGetAttributeVerificationFlow(enabled);
    return {data: enabled ? configuredHandle : undefined};
  },
}));

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useAdministrationOperation: () => administrationOperation,
    useToast: () => ({showToast: mockShowToast}),
    useCapabilitiesContext: () => ({
      can: (resource: string, action?: string) => canUpdate && resource === 'user' && action === 'update',
    }),
  };
});

const SEND = {name: 'Send verification link for Email'};

describe('SendVerificationLinkButton', () => {
  beforeEach(() => {
    mockMutate.mockReset();
    mockShowToast.mockReset();
    isPending = false;
    administrationOperation = AdministrationModes.FLOW;
    canUpdate = true;
    configuredHandle = 'custom-verification-flow';
    mockUseGetAttributeVerificationFlow.mockReset();
  });

  it('starts the flow the server configures when clicked', async () => {
    const user = userEvent.setup();
    render(<SendVerificationLinkButton userId="user-1" attribute="workEmail" label="Email" />);

    await user.click(screen.getByRole('button', SEND));

    expect(mockMutate).toHaveBeenCalledWith(
      {userId: 'user-1', attribute: 'workEmail', handle: 'custom-verification-flow'},
      expect.anything(),
    );
  });

  it.each([
    {name: 'is unset', handle: ''},
    {name: 'is still loading', handle: undefined},
  ])('is not offered while the configured flow $name', ({handle}) => {
    configuredHandle = handle;
    render(<SendVerificationLinkButton userId="user-1" attribute="email" label="Email" />);

    expect(screen.queryByRole('button', SEND)).not.toBeInTheDocument();
    expect(screen.queryByText('Save to verify')).not.toBeInTheDocument();
  });

  it('reads the configured flow only where the action can be offered', () => {
    canUpdate = false;
    render(<SendVerificationLinkButton userId="user-1" attribute="email" label="Email" />);

    expect(mockUseGetAttributeVerificationFlow).toHaveBeenCalledWith(false);
  });

  it('shows one error toast when sending fails', async () => {
    mockMutate.mockImplementation((_vars, options: {onError: (error: Error) => void}) =>
      options.onError(new Error('Network down')),
    );
    const user = userEvent.setup();
    render(<SendVerificationLinkButton userId="user-1" attribute="email" label="Email" />);

    await user.click(screen.getByRole('button', SEND));

    expect(mockShowToast).toHaveBeenCalledTimes(1);
    expect(mockShowToast).toHaveBeenCalledWith('Could not send the verification link. Please try again.', 'error');
  });

  it('shows the suspended message, not the server text, when the flow refuses the user', async () => {
    mockMutate.mockImplementation((_vars, options: {onError: (error: Error) => void}) =>
      options.onError(new FlowExecutionFailure({code: 'FET-1114', message: {defaultValue: 'Account suspended'}}, 'x')),
    );
    const user = userEvent.setup();
    render(<SendVerificationLinkButton userId="user-1" attribute="email" label="Email" />);

    await user.click(screen.getByRole('button', SEND));

    expect(mockShowToast).toHaveBeenCalledWith('This user is suspended. Unsuspend the user and try again.', 'error');
  });

  it('says the email provider is not configured, not to retry, when the flow has none', async () => {
    mockMutate.mockImplementation((_vars, options: {onError: (error: Error) => void}) =>
      options.onError(new FlowExecutionFailure({code: 'FET-1038', message: {defaultValue: 'Not configured'}}, 'x')),
    );
    const user = userEvent.setup();
    render(<SendVerificationLinkButton userId="user-1" attribute="email" label="Email" />);

    await user.click(screen.getByRole('button', SEND));

    expect(mockShowToast).toHaveBeenCalledWith('Email provider is not configured.', 'error');
  });

  it('is not offered where administration runs natively, with no flow to start', () => {
    administrationOperation = AdministrationModes.NATIVE;
    render(<SendVerificationLinkButton userId="user-1" attribute="email" label="Email" />);

    expect(screen.queryByRole('button', SEND)).not.toBeInTheDocument();
  });

  it('is not offered to an operator who cannot update users', () => {
    canUpdate = false;
    render(<SendVerificationLinkButton userId="user-1" attribute="email" label="Email" />);

    expect(screen.queryByRole('button', SEND)).not.toBeInTheDocument();
  });

  it('is disabled while the link is being sent', () => {
    isPending = true;
    render(<SendVerificationLinkButton userId="user-1" attribute="email" label="Email" />);

    expect(screen.getByRole('button', {name: 'Sending...'})).toBeDisabled();
  });

  it.each([
    {accessStatus: 'SUSPENDED', hint: 'Unsuspend the user to send a verification link.'},
    {accessStatus: 'EXPIRED', hint: "This user's account has expired, so a verification link cannot be sent."},
  ] as const)('is disabled for a $accessStatus user, with a hint saying why', async ({accessStatus, hint}) => {
    const user = userEvent.setup();
    render(<SendVerificationLinkButton userId="user-1" attribute="email" label="Email" accessStatus={accessStatus} />);

    const button = screen.getByRole('button', SEND);
    expect(button).toBeDisabled();
    await user.hover(button.parentElement!);
    expect(await screen.findByText(hint)).toBeInTheDocument();
    expect(mockMutate).not.toHaveBeenCalled();
  });

  it.each([
    {name: 'administration runs natively', mode: AdministrationModes.NATIVE, accessStatus: undefined},
    {name: 'the user is suspended', mode: AdministrationModes.FLOW, accessStatus: 'SUSPENDED'},
    {name: 'the user has expired', mode: AdministrationModes.FLOW, accessStatus: 'EXPIRED'},
  ] as const)('shows no save hint for an edited value when $name', ({mode, accessStatus}) => {
    administrationOperation = mode;
    render(
      <SendVerificationLinkButton userId="user-1" attribute="email" label="Email" accessStatus={accessStatus} edited />,
    );

    expect(screen.queryByText('Save to verify')).not.toBeInTheDocument();
  });

  it('asks to save an edited value instead of sending', () => {
    render(<SendVerificationLinkButton userId="user-1" attribute="email" label="Email" edited />);

    expect(screen.getByText('Save to verify')).toBeInTheDocument();
    expect(screen.queryByRole('button', SEND)).not.toBeInTheDocument();
  });

  it('names the attribute it sends to, so two email fields give two distinct buttons', () => {
    render(<SendVerificationLinkButton userId="user-1" attribute="workEmail" label="Work Email" />);

    expect(screen.getByRole('button', {name: 'Send verification link for Work Email'})).toHaveTextContent('Verify');
  });

  it('stays enabled for a locked user, who can still verify', () => {
    render(<SendVerificationLinkButton userId="user-1" attribute="email" label="Email" accessStatus="LOCKED" />);

    expect(screen.getByRole('button', SEND)).toBeEnabled();
  });
});
