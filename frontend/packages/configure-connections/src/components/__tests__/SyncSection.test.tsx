// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {fireEvent, render, screen, within} from '@testing-library/react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import SyncSection from '../SyncSection';

vi.mock('@thunderid/components', () => ({
  SettingsCard: ({title, children}: {title: string; children: React.ReactNode}) => (
    <section aria-label={title}>{children}</section>
  ),
}));

describe('SyncSection', () => {
  const onChange = vi.fn();
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('shows the server defaults when no mode is set', () => {
    render(<SyncSection onChange={onChange} />);
    expect(screen.getByRole('combobox', {name: /Attributes sync/})).toHaveTextContent('Override received');
    expect(screen.getByRole('combobox', {name: /Groups and roles sync/})).toHaveTextContent('No sync');
  });

  it('reports a changed mode and keeps the other one', () => {
    render(<SyncSection sync={{attributes: 'no_sync'}} onChange={onChange} />);
    fireEvent.mouseDown(screen.getByRole('combobox', {name: /Groups and roles sync/}));
    fireEvent.click(within(screen.getByRole('listbox')).getByText('Session only'));
    expect(onChange).toHaveBeenCalledWith({attributes: 'no_sync', groupsAndRoles: 'session_only'});
  });

  it('does not offer override all for attributes', () => {
    render(<SyncSection onChange={onChange} />);
    fireEvent.mouseDown(screen.getByRole('combobox', {name: /Attributes sync/}));
    expect(within(screen.getByRole('listbox')).queryByText('Override all')).not.toBeInTheDocument();
  });

  it('does not offer override received for groups and roles', () => {
    render(<SyncSection onChange={onChange} />);
    fireEvent.mouseDown(screen.getByRole('combobox', {name: /Groups and roles sync/}));
    expect(within(screen.getByRole('listbox')).queryByText('Override received')).not.toBeInTheDocument();
  });
});
