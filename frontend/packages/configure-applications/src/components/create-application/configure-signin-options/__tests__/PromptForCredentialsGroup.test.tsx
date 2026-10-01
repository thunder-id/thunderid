// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, within} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {AuthenticatorTypes} from '@thunderid/configure-connections';
import {describe, it, expect, vi} from 'vitest';
import PromptForCredentialsGroup, {type PromptForCredentialsGroupProps} from '../PromptForCredentialsGroup';

vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (key: string, fallback?: string) => fallback ?? key,
  }),
}));

describe('PromptForCredentialsGroup', () => {
  const mockOnIntegrationToggle = vi.fn();

  const defaultProps: PromptForCredentialsGroupProps = {
    integrations: {[AuthenticatorTypes.CREDENTIALS_AUTH]: false},
    onIntegrationToggle: mockOnIntegrationToggle,
  };

  const renderComponent = (props: Partial<PromptForCredentialsGroupProps> = {}) =>
    render(<PromptForCredentialsGroup {...defaultProps} {...props} />);

  it('starts expanded and toggles the integration on click', async () => {
    const user = userEvent.setup();
    renderComponent();

    const row = screen.getByTestId(`auth-method-${AuthenticatorTypes.CREDENTIALS_AUTH}`);
    await user.click(within(row).getByRole('switch'));

    expect(mockOnIntegrationToggle).toHaveBeenCalledWith(AuthenticatorTypes.CREDENTIALS_AUTH);
  });

  it('starts collapsed, with the master checkbox and expand chevron disabled, when a pre-configured flow is selected', () => {
    renderComponent({disabled: true});

    expect(screen.queryByTestId(`auth-method-${AuthenticatorTypes.CREDENTIALS_AUTH}`)).not.toBeInTheDocument();
    expect(screen.getByRole('checkbox')).toBeDisabled();
    expect(screen.getByRole('button', {name: 'Expand'})).toBeDisabled();
  });
});
