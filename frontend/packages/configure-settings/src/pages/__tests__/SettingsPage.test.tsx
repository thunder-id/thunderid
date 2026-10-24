// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {screen, waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {renderWithProviders} from '@thunderid/test-utils';
import {describe, it, expect, vi} from 'vitest';

vi.mock('../../components/cors/CorsSection', () => ({
  default: () => <div data-testid="cors-section" />,
}));

const {default: SettingsPage} = await import('../SettingsPage');

describe('SettingsPage', () => {
  it('renders the title, subtitle, the CORS tab, and the CORS panel', () => {
    renderWithProviders(<SettingsPage />);

    expect(screen.getByText('Settings')).toBeInTheDocument();
    expect(screen.getByText('Settings that apply across your entire ThunderID deployment.')).toBeInTheDocument();
    expect(screen.getByRole('tab', {name: 'CORS'})).toBeInTheDocument();
    expect(screen.getByTestId('cors-section')).toBeInTheDocument();
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
});
