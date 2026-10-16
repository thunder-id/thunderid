// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {screen, fireEvent, renderWithProviders, within} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import OrganizationUnitSelectDialog from '../OrganizationUnitSelectDialog';

const levels: Record<string, {id: string; handle: string; name: string}[]> = {
  root: [
    {id: 'acme', handle: 'acme', name: 'Acme Corp'},
    {id: 'northwind', handle: 'northwind', name: 'Northwind Partners'},
  ],
  acme: [
    {id: 'retail', handle: 'retail', name: 'Acme Retail'},
    {id: 'trials', handle: 'trials', name: 'Acme Trials'},
  ],
  retail: [{id: 'retail-eu', handle: 'retail-eu', name: 'Retail EU'}],
};

vi.mock('@thunderid/configure-organization-units', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/configure-organization-units')>()),
  usePagedOrganizationUnits: (parentId: string | null) => ({
    data: {pages: [{totalResults: 0, startIndex: 1, count: 0, organizationUnits: levels[parentId ?? 'root'] ?? []}]},
    error: null,
    isLoading: false,
    refetch: vi.fn(),
    hasNextPage: false,
    fetchNextPage: vi.fn(),
    isFetchingNextPage: false,
  }),
}));

const baseProps = {
  open: true,
  title: 'Pick units',
  description: 'Choose some',
  confirmLabel: 'Done',
  onClose: vi.fn(),
  onConfirm: vi.fn(),
};

describe('OrganizationUnitSelectDialog', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('returns the picked children with the reach chosen for each', () => {
    renderWithProviders(
      <OrganizationUnitSelectDialog {...baseProps} source={{type: 'children', parentId: 'acme'}} withReach />,
    );

    fireEvent.click(screen.getByRole('checkbox', {name: /Acme Retail/}));
    fireEvent.click(screen.getByRole('button', {name: 'This unit only'}));
    fireEvent.click(screen.getByRole('button', {name: 'Done'}));

    expect(baseProps.onConfirm).toHaveBeenCalledWith([{ouId: 'retail', reach: 'unit'}]);
  });

  it('keeps confirm disabled until something is picked', () => {
    renderWithProviders(<OrganizationUnitSelectDialog {...baseProps} source={{type: 'roots'}} />);

    expect(screen.getByRole('button', {name: 'Done'})).toBeDisabled();
  });

  it('locks already chosen units and hides excluded ones', () => {
    renderWithProviders(
      <OrganizationUnitSelectDialog
        {...baseProps}
        source={{type: 'roots'}}
        hiddenIds={['acme']}
        lockedIds={['northwind']}
        lockedLabel="Already granted"
      />,
    );

    expect(screen.queryByText('Acme Corp')).not.toBeInTheDocument();
    expect(screen.getByRole('checkbox', {name: /Northwind Partners/})).toBeDisabled();
    expect(screen.getByText('Already granted')).toBeInTheDocument();
  });

  it('filters the listed units by name or handle', () => {
    renderWithProviders(<OrganizationUnitSelectDialog {...baseProps} source={{type: 'roots'}} />);

    fireEvent.change(screen.getByRole('textbox', {name: 'Search organization units'}), {target: {value: 'north'}});

    expect(screen.queryByText('Acme Corp')).not.toBeInTheDocument();
    expect(screen.getByText('Northwind Partners')).toBeInTheDocument();
  });

  it('marks the units below a picked unit as included with it when browsing every tree', () => {
    renderWithProviders(<OrganizationUnitSelectDialog {...baseProps} source={{type: 'allTrees'}} tone="exclude" />);

    fireEvent.click(screen.getByRole('button', {name: 'Show units under Acme Corp'}));
    fireEvent.click(screen.getByRole('checkbox', {name: /Acme Retail/}));
    fireEvent.click(screen.getByRole('button', {name: 'Show units under Acme Retail'}));

    const child = screen.getByRole('checkbox', {name: /Retail EU/});
    expect(child).toBeChecked();
    expect(child).toBeDisabled();
    expect(screen.getByText('Included with Acme Retail')).toBeInTheDocument();
  });

  it('lets a picked unit be removed from the summary', () => {
    renderWithProviders(<OrganizationUnitSelectDialog {...baseProps} source={{type: 'roots'}} />);

    fireEvent.click(screen.getByRole('checkbox', {name: /Northwind Partners/}));
    const summary = screen.getByText('To grant').parentElement!;
    fireEvent.click(within(summary).getByRole('button', {name: 'Remove Northwind Partners'}));

    expect(screen.getByRole('button', {name: 'Done'})).toBeDisabled();
  });
});
