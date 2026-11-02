// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {fireEvent, render, screen, within} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type {ConnectionInstance} from '@thunderid/configure-connections';
import type {BasicFlowDefinition} from '@thunderid/configure-flows';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import ConfigureMfaSettings from '../ConfigureMfaSettings';

vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    // AuthenticationMethodItem's own "Not configured" text has no inline fallback, so it needs a
    // real lookup here (mirrors AuthenticationMethodItem.test.tsx's mock).
    t: (key: string, fallback?: string) => {
      if (key === 'applications:onboarding.configure.SignInOptions.notConfigured') {
        return 'Not configured';
      }
      return fallback ?? key;
    },
  }),
}));

interface MockSmsProvidersResponse {
  data: ConnectionInstance[] | undefined;
  isLoading: boolean;
}
const mockUseSMSProviders = vi.fn<() => MockSmsProvidersResponse>();
vi.mock('@thunderid/configure-connections', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/configure-connections')>()),
  useSMSProviders: () => mockUseSMSProviders(),
}));

// Real @thunderid/configure-connections transitively resolves @thunderid/configure-organization-units'
// dist build, which fails to resolve its framer-motion import under vitest's transform. Stubbed here to
// avoid that (same workaround ApplicationCreatePage.test.tsx already applies); this component never
// renders anything from that package.
vi.mock('@thunderid/configure-organization-units', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/configure-organization-units')>()),
  useHasMultipleOUs: () => ({hasMultipleOUs: false, isLoading: false, ouList: []}),
  useGetOrganizationUnit: () => ({data: undefined}),
  OrganizationUnitPickerScreen: () => null,
}));

const mockSetIsEmailOtpMfaEnabled = vi.fn();
const mockSetIsSmsOtpMfaEnabled = vi.fn();
const mockSetSmsOtpSenderId = vi.fn();
let mockContextState: {
  integrations: Record<string, boolean>;
  isEmailOtpMfaEnabled: boolean;
  isSmsOtpMfaEnabled: boolean;
  smsOtpSenderId: string;
  selectedAuthFlow: BasicFlowDefinition | null;
};

vi.mock('@/hooks/useApplicationCreateContext', () => ({
  default: () => ({
    ...mockContextState,
    setIsEmailOtpMfaEnabled: mockSetIsEmailOtpMfaEnabled,
    setIsSmsOtpMfaEnabled: mockSetIsSmsOtpMfaEnabled,
    setSmsOtpSenderId: mockSetSmsOtpSenderId,
  }),
}));

describe('ConfigureMfaSettings', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockContextState = {
      integrations: {},
      isEmailOtpMfaEnabled: false,
      isSmsOtpMfaEnabled: false,
      smsOtpSenderId: '',
      selectedAuthFlow: null,
    };
    mockUseSMSProviders.mockReturnValue({data: [], isLoading: false});
  });

  // The group's method list is collapsed by default, expanded via its own chevron (independent of
  // the master checkbox); expand it so tests can reach the rows inside.
  const renderExpanded = () => {
    const result = render(<ConfigureMfaSettings />);
    fireEvent.click(screen.getByRole('button', {name: 'Expand'}));
    return result;
  };

  it('renders Email OTP as always available', () => {
    renderExpanded();

    expect(screen.getByText('Email OTP')).toBeInTheDocument();
    const emailRow = screen.getByTestId('auth-method-email-otp');
    expect(within(emailRow).getByRole('switch')).not.toBeDisabled();
  });

  it('shows SMS OTP as not configured when no SMS provider connections exist', () => {
    mockUseSMSProviders.mockReturnValue({data: [], isLoading: false});
    renderExpanded();

    expect(screen.getAllByText('Not configured').length).toBeGreaterThan(0);
  });

  it('enables the SMS OTP toggle when at least one SMS provider connection exists', () => {
    mockUseSMSProviders.mockReturnValue({
      data: [{id: 'sender-1', name: 'Twilio', type: 'twilio'} as unknown as ConnectionInstance],
      isLoading: false,
    });
    renderExpanded();

    const smsRow = screen.getByTestId('auth-method-sms-otp');
    expect(within(smsRow).getByRole('switch')).not.toBeDisabled();
  });

  it('toggles Email OTP MFA on click', async () => {
    const user = userEvent.setup();
    renderExpanded();

    const emailRow = screen.getByTestId('auth-method-email-otp');
    const toggle = within(emailRow).getByRole('switch');
    await user.click(toggle);

    expect(mockSetIsEmailOtpMfaEnabled).toHaveBeenCalledWith(true);
  });

  it('auto-selects the first SMS provider as the sender when SMS OTP is turned on', async () => {
    mockUseSMSProviders.mockReturnValue({
      data: [{id: 'sender-1', name: 'Twilio', type: 'twilio'} as unknown as ConnectionInstance],
      isLoading: false,
    });
    const user = userEvent.setup();
    renderExpanded();

    const smsRow = screen.getByTestId('auth-method-sms-otp');
    const toggle = within(smsRow).getByRole('switch');
    await user.click(toggle);

    expect(mockSetIsSmsOtpMfaEnabled).toHaveBeenCalledWith(true);
    expect(mockSetSmsOtpSenderId).toHaveBeenCalledWith('sender-1');
  });

  it('shows a disabled note when a pre-configured flow is selected', () => {
    mockContextState.selectedAuthFlow = {
      id: 'flow-1',
      name: 'Existing Flow',
      handle: 'existing-flow',
    } as BasicFlowDefinition;
    render(<ConfigureMfaSettings />);

    expect(
      screen.getByText(
        "MFA isn't available for the selected pre-configured flow. Clear it, or choose individual sign-in methods above instead.",
      ),
    ).toBeInTheDocument();
  });

  it('does not show the disabled note when a flow auto-matched from active toggles is selected', () => {
    mockContextState.integrations = {'credentials-auth': true};
    mockContextState.selectedAuthFlow = {
      id: 'flow-1',
      name: 'Existing Flow',
      handle: 'existing-flow',
    } as BasicFlowDefinition;
    renderExpanded();

    expect(
      screen.queryByText(
        "MFA isn't available for the selected pre-configured flow. Clear it, or choose individual sign-in methods above instead.",
      ),
    ).not.toBeInTheDocument();
    const emailRow = screen.getByTestId('auth-method-email-otp');
    expect(within(emailRow).getByRole('switch')).not.toBeDisabled();
  });
});
