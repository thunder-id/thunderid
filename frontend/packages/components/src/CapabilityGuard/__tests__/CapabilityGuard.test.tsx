// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {Capabilities} from '@thunderid/contexts';
import {render, screen} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import CapabilityGuard from '../CapabilityGuard';

let mockCapabilities: Capabilities = {can: () => true, canManageRoot: true, isLoading: false};
vi.mock('@thunderid/contexts', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/contexts')>()),
  useCapabilitiesContext: () => mockCapabilities,
}));

describe('CapabilityGuard', () => {
  beforeEach(() => {
    mockCapabilities = {can: () => true, canManageRoot: true, isLoading: false};
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

  it('checks the root permission for a root page', () => {
    mockCapabilities = {...mockCapabilities, canManageRoot: false};
    render(
      <CapabilityGuard root>
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

    expect(screen.queryByText('page')).not.toBeInTheDocument();
    expect(screen.queryByText('You do not have permission to view this page.')).not.toBeInTheDocument();
  });
});
