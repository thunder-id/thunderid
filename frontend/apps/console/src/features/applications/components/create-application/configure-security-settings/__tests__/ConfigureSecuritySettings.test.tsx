// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import {OrganizationUnitDefaultItem} from '@thunderid/configure-applications';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import ConfigureSecuritySettings from '../ConfigureSecuritySettings';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, fallback?: string) => fallback ?? key,
  }),
}));

let mockOuDefaults: Record<string, boolean>;
vi.mock('@/features/applications/hooks/useApplicationCreateContext', () => ({
  default: () => ({
    integrations: {},
    toggleIntegration: vi.fn(),
    ouDefaults: mockOuDefaults,
  }),
}));

// ConfigureMfaSettings (and the other sign-in categories) are now rendered internally by
// ConfigureSignInOptions itself (see ConfigureSignInOptions.tsx), so this stub only needs to
// stand in for the whole sign-in section; their own rendering is covered by
// ConfigureSignInOptions.test.tsx and ConfigureMfaSettings.test.tsx.
vi.mock('../../configure-signin-options/ConfigureSignInOptions', () => ({
  default: () => <div data-testid="sign-in-options" />,
}));

describe('ConfigureSecuritySettings', () => {
  beforeEach(() => {
    mockOuDefaults = {[OrganizationUnitDefaultItem.SIGN_IN]: false};
  });

  it('renders the sign-in section when sign-in is not snapshotted from the OU', () => {
    render(<ConfigureSecuritySettings />);

    expect(screen.getByTestId('sign-in-options')).toBeInTheDocument();
  });

  it('renders no sign-in section when sign-in is snapshotted from the organization unit default', () => {
    mockOuDefaults = {[OrganizationUnitDefaultItem.SIGN_IN]: true};
    render(<ConfigureSecuritySettings />);

    expect(screen.queryByTestId('sign-in-options')).not.toBeInTheDocument();
  });
});
