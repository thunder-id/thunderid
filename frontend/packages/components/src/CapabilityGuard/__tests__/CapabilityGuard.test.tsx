// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {Capabilities} from '@thunderid/contexts';
import {render, screen, userEvent} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import CapabilityGuard from '../CapabilityGuard';

let mockCapabilities: Capabilities = {can: () => true, isLoading: false};
vi.mock('@thunderid/contexts', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/contexts')>()),
  useCapabilitiesContext: () => mockCapabilities,
}));

describe('CapabilityGuard', () => {
  beforeEach(() => {
    mockCapabilities = {can: () => true, isLoading: false};
  });

  it('renders the page when the action is allowed', () => {
    const can = vi.fn(() => true);
    mockCapabilities = {...mockCapabilities, can};
    render(
      <CapabilityGuard resource="app" action="create">
        <span>page</span>
      </CapabilityGuard>,
    );

    expect(screen.getByText('page')).toBeInTheDocument();
    expect(can).toHaveBeenCalledWith('app', 'create');
  });

  it('shows a notice instead of the page when the action is not allowed', () => {
    mockCapabilities = {...mockCapabilities, can: () => false};
    render(
      <CapabilityGuard resource="app" action="create">
        <span>page</span>
      </CapabilityGuard>,
    );

    expect(screen.queryByText('page')).not.toBeInTheDocument();
    expect(screen.getByText('You do not have permission to view this page.')).toBeInTheDocument();
  });

  it('renders a page that offers several actions when any one of them is allowed', () => {
    mockCapabilities = {...mockCapabilities, can: (resource, action) => resource === 'export' && action === 'create'};
    render(
      <CapabilityGuard
        anyOf={[
          {resource: 'import', action: 'create'},
          {resource: 'export', action: 'create'},
        ]}
      >
        <span>page</span>
      </CapabilityGuard>,
    );

    expect(screen.getByText('page')).toBeInTheDocument();
  });

  it('shows a notice instead of a page that offers several actions when none is allowed', () => {
    mockCapabilities = {...mockCapabilities, can: () => false};
    render(
      <CapabilityGuard
        anyOf={[
          {resource: 'import', action: 'create'},
          {resource: 'export', action: 'create'},
        ]}
      >
        <span>page</span>
      </CapabilityGuard>,
    );

    expect(screen.queryByText('page')).not.toBeInTheDocument();
    expect(screen.getByText('You do not have permission to view this page.')).toBeInTheDocument();
  });

  it('renders neither while the capabilities load', () => {
    mockCapabilities = {...mockCapabilities, can: () => false, isLoading: true};
    render(
      <CapabilityGuard resource="app">
        <span>page</span>
      </CapabilityGuard>,
    );

    expect(screen.getByRole('status')).toBeInTheDocument();
    expect(screen.queryByText('page')).not.toBeInTheDocument();
    expect(screen.queryByText('You do not have permission to view this page.')).not.toBeInTheDocument();
  });

  it('offers a retry instead of a denial when the capabilities could not be read', async () => {
    const retry = vi.fn();
    mockCapabilities = {can: () => false, isLoading: false, error: new Error('unavailable'), retry};
    render(
      <CapabilityGuard resource="app">
        <span>page</span>
      </CapabilityGuard>,
    );

    expect(screen.queryByText('page')).not.toBeInTheDocument();
    expect(screen.queryByText('You do not have permission to view this page.')).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', {name: 'Refresh'}));
    expect(retry).toHaveBeenCalled();
  });
});
