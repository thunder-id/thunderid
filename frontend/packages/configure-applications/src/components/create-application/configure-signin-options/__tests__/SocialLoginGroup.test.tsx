// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {fireEvent, render, screen} from '@testing-library/react';
import {IdentityProviderTypes, type IdentityProvider} from '@thunderid/configure-connections';
import {describe, it, expect, vi} from 'vitest';
import SocialLoginGroup, {type SocialLoginGroupProps} from '../SocialLoginGroup';

vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (key: string, fallback?: string) => {
      if (key === 'applications:onboarding.configure.SignInOptions.notConfigured') {
        return 'Not configured';
      }
      return fallback ?? key;
    },
  }),
}));

describe('SocialLoginGroup', () => {
  const mockOnIntegrationToggle = vi.fn();

  const availableIntegrations: IdentityProvider[] = [
    {id: 'google-idp', name: 'Google', type: IdentityProviderTypes.GOOGLE, description: 'Sign in with Google'},
  ];

  const defaultProps: SocialLoginGroupProps = {
    integrations: {'google-idp': false},
    availableIntegrations,
    onIntegrationToggle: mockOnIntegrationToggle,
  };

  it('starts collapsed, same as always, when no flow is selected', () => {
    render(<SocialLoginGroup {...defaultProps} />);

    expect(screen.queryByTestId('auth-method-google-idp')).not.toBeInTheDocument();
    expect(screen.getByRole('checkbox')).not.toBeDisabled();
    expect(screen.getByRole('button', {name: 'Expand'})).not.toBeDisabled();
  });

  it('collapses an already-expanded list and disables the checkbox/chevron once a pre-configured flow is selected', () => {
    const {rerender} = render(<SocialLoginGroup {...defaultProps} />);
    const chevron = screen.getByRole('button', {name: 'Expand'});
    fireEvent.click(chevron);
    expect(screen.getByTestId('auth-method-google-idp')).toBeInTheDocument();
    expect(chevron).toHaveAttribute('aria-expanded', 'true');

    rerender(<SocialLoginGroup {...defaultProps} disabled />);

    // The Collapse's own exit transition (not run by jsdom) governs when the row actually leaves
    // the DOM under `unmountOnExit` — asserted here via the chevron/checkbox's immediate state
    // instead, which the disabling effect updates synchronously regardless of that animation.
    expect(chevron).toHaveAttribute('aria-expanded', 'false');
    expect(screen.getByRole('checkbox')).toBeDisabled();
    expect(chevron).toBeDisabled();
  });

  it('keeps the master checkbox disabled when no provider is configured, independent of any flow selection', () => {
    render(<SocialLoginGroup {...defaultProps} availableIntegrations={[]} />);

    expect(screen.getByRole('checkbox')).toBeDisabled();
  });
});
