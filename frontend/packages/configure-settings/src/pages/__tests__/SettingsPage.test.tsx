// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {screen, waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {renderWithProviders} from '@thunderid/test-utils';
import {beforeEach, describe, it, expect, vi} from 'vitest';

vi.mock('../../components/cors/CorsSection', () => ({
  default: () => (
    <div data-testid="cors-section">
      <input aria-label="CORS draft" />
    </div>
  ),
}));

vi.mock('../../components/accountSecurity/AccountSecuritySection', () => ({
  default: () => <div data-testid="account-security-section" />,
}));

const mockConfig = vi.hoisted(() => ({productName: ''}));

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: () => {
      const context = actual.useConfig();
      if (!mockConfig.productName) {
        return context;
      }
      return {
        ...context,
        config: {...context.config, brand: {...context.config.brand, product_name: mockConfig.productName}},
      };
    },
  };
});

const {default: SettingsPage} = await import('../SettingsPage');

describe('SettingsPage', () => {
  beforeEach(() => {
    mockConfig.productName = '';
  });

  it('renders the title, subtitle, the CORS tab, and the CORS panel', () => {
    renderWithProviders(<SettingsPage />);

    expect(screen.getByText('Settings')).toBeInTheDocument();
    expect(screen.getByText('Settings that apply across your entire ThunderID deployment.')).toBeInTheDocument();
    expect(screen.getByRole('tab', {name: 'CORS'})).toBeInTheDocument();
    expect(screen.getByTestId('cors-section')).toBeInTheDocument();
  });

  it('names the configured product in the subtitle', () => {
    mockConfig.productName = 'Acme IAM';
    renderWithProviders(<SettingsPage />);

    expect(screen.getByText('Settings that apply across your entire Acme IAM deployment.')).toBeInTheDocument();
  });

  it('sets the document title', async () => {
    renderWithProviders(<SettingsPage />);
    await waitFor(() => expect(document.title).toBe('Settings | ThunderID'));
  });

  it('keeps the CORS panel active when its tab is clicked', async () => {
    const user = userEvent.setup();
    renderWithProviders(<SettingsPage />);
    await user.click(screen.getByRole('tab', {name: 'CORS'}));
    expect(screen.getByTestId('cors-section')).toBeInTheDocument();
  });

  it('keeps an unsaved edit when switching tabs and back', async () => {
    const user = userEvent.setup();
    renderWithProviders(<SettingsPage />);

    await user.type(screen.getByRole('textbox', {name: 'CORS draft'}), 'https://example.com');
    await user.click(screen.getByRole('tab', {name: 'Account security'}));
    expect(screen.getByTestId('cors-section')).not.toBeVisible();
    expect(screen.getByTestId('account-security-section')).toBeVisible();

    await user.click(screen.getByRole('tab', {name: 'CORS'}));
    expect(screen.getByRole('textbox', {name: 'CORS draft'})).toHaveValue('https://example.com');
  });
});
