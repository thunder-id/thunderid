// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, fireEvent, waitFor} from '@thunderid/test-utils';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import OrganizationUnitSection from '../OrganizationUnitSection';

const ouQuery: {data?: {id: string; handle: string; name: string}} = {};

vi.mock('../../api/useConnectionOU', () => ({default: () => ouQuery}));
vi.mock('@thunderid/components', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/components')>()),
  SettingsCard: ({title, children}: {title: string; children: React.ReactNode}) => (
    <section aria-label={title}>{children}</section>
  ),
}));

describe('OrganizationUnitSection', () => {
  const writeText = vi.fn().mockResolvedValue(undefined);

  beforeEach(() => {
    vi.clearAllMocks();
    ouQuery.data = undefined;
    Object.defineProperty(navigator, 'clipboard', {value: {writeText}, configurable: true});
  });

  it('shows the id, and leaves the handle out until it is known', () => {
    render(<OrganizationUnitSection ouId="ou-1" />);

    expect(screen.getByRole('region', {name: 'Organization unit'})).toBeInTheDocument();
    expect(screen.getByDisplayValue('ou-1')).toBeInTheDocument();
    expect(screen.queryByTestId('connection-ou-handle-copy')).not.toBeInTheDocument();
  });

  it('shows the handle and the id once the organization unit is known', () => {
    ouQuery.data = {id: 'ou-1', handle: 'tenant-a', name: 'Tenant A'};
    render(<OrganizationUnitSection ouId="ou-1" />);

    expect(screen.getByDisplayValue('tenant-a')).toBeInTheDocument();
    expect(screen.getByDisplayValue('ou-1')).toBeInTheDocument();
  });

  it('copies the handle and the id', async () => {
    ouQuery.data = {id: 'ou-1', handle: 'tenant-a', name: 'Tenant A'};
    render(<OrganizationUnitSection ouId="ou-1" />);

    fireEvent.click(screen.getByTestId('connection-ou-handle-copy'));
    await waitFor(() => expect(writeText).toHaveBeenCalledWith('tenant-a'));

    fireEvent.click(screen.getByTestId('connection-ou-id-copy'));
    await waitFor(() => expect(writeText).toHaveBeenCalledWith('ou-1'));
  });
});
