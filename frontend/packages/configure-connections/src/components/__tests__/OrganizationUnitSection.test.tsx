// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, fireEvent, waitFor} from '@thunderid/test-utils';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import OrganizationUnitSection from '../OrganizationUnitSection';

const ouQuery: {data?: {id: string; handle: string; name: string}} = {};
const useGetOrganizationUnitMock = vi.fn<(id?: string) => typeof ouQuery>(() => ouQuery);

vi.mock('@thunderid/configure-organization-units', () => ({
  useGetOrganizationUnit: (id?: string) => useGetOrganizationUnitMock(id),
}));
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

  it('looks the organization unit up through the shared organization unit hook', () => {
    render(<OrganizationUnitSection ouId="ou-1" />);

    expect(useGetOrganizationUnitMock).toHaveBeenCalledWith('ou-1');
  });

  it('shows the id, and leaves the handle out until it is known', () => {
    render(<OrganizationUnitSection ouId="ou-1" />);

    expect(screen.getByRole('region', {name: 'Organization unit'})).toBeInTheDocument();
    expect(screen.getByText('ID')).toBeInTheDocument();
    expect(screen.getByText('ou-1')).toBeInTheDocument();
    expect(screen.queryByText('Handle')).not.toBeInTheDocument();
  });

  it('shows the handle and the id once the organization unit is known', () => {
    ouQuery.data = {id: 'ou-1', handle: 'tenant-a', name: 'Tenant A'};
    render(<OrganizationUnitSection ouId="ou-1" />);

    expect(screen.getByText('Handle')).toBeInTheDocument();
    expect(screen.getByText('tenant-a')).toBeInTheDocument();
    expect(screen.getByText('ou-1')).toBeInTheDocument();
  });

  it('copies the handle and the id', async () => {
    ouQuery.data = {id: 'ou-1', handle: 'tenant-a', name: 'Tenant A'};
    render(<OrganizationUnitSection ouId="ou-1" />);

    fireEvent.click(screen.getByRole('button', {name: 'Copy handle'}));
    await waitFor(() => expect(writeText).toHaveBeenCalledWith('tenant-a'));

    fireEvent.click(screen.getByRole('button', {name: 'Copy organization unit ID'}));
    await waitFor(() => expect(writeText).toHaveBeenCalledWith('ou-1'));
  });
});
