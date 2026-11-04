// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {screen, within} from '@testing-library/react';
import {render, userEvent} from '@thunderid/test-utils';
import {ListingTable} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {describe, expect, it, vi} from 'vitest';
import useFlowListColumns, {type FlowListRow} from '../useFlowListColumns';

// The package initializes no i18next instance, so the fallback is the rendered copy.
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, fallback?: string) => (typeof fallback === 'string' ? fallback : key),
  }),
}));

const flows: FlowListRow[] = [
  {id: 'flow-1', name: 'Suspend users', flowType: 'ADMINISTRATION', updatedAt: '2025-01-01T10:00:00Z'},
  {
    id: 'flow-2',
    name: 'Default sign-in',
    flowType: 'AUTHENTICATION',
    updatedAt: '2025-01-02T15:30:00Z',
    isReadOnly: true,
  },
];

function FlowTable({
  onOpen,
  onDelete = undefined,
}: {
  onOpen: (flow: FlowListRow) => void;
  onDelete?: (flow: FlowListRow) => void;
}): JSX.Element {
  const columns = useFlowListColumns<FlowListRow>({onOpen, onDelete});

  return (
    <ListingTable.Provider variant="data-grid-card">
      <ListingTable.Container disablePaper>
        <ListingTable.DataGrid rows={flows} columns={columns} hideFooter autoHeight />
      </ListingTable.Container>
    </ListingTable.Provider>
  );
}

const rowOf = (name: string): HTMLElement => screen.getByRole('row', {name: new RegExp(name)});

describe('useFlowListColumns', () => {
  it('lists the name, type and last update of each flow', () => {
    render(<FlowTable onOpen={vi.fn()} />);

    ['Name', 'Type', 'Last updated', 'Actions'].forEach((header) => {
      expect(screen.getByRole('columnheader', {name: header})).toBeInTheDocument();
    });
    const row = rowOf('Suspend users');
    expect(within(row).getByText('ADMINISTRATION')).toBeInTheDocument();
    expect(within(row).getByText(/Jan.*2025|2025.*Jan/)).toBeInTheDocument();
  });

  it('opens and deletes an editable flow through its own actions', async () => {
    const onOpen = vi.fn();
    const onDelete = vi.fn();
    render(<FlowTable onOpen={onOpen} onDelete={onDelete} />);
    const row = rowOf('Suspend users');

    await userEvent.click(within(row).getByRole('button', {name: 'Edit'}));
    expect(onOpen).toHaveBeenCalledWith(flows[0]);

    await userEvent.click(within(row).getByRole('button', {name: 'Delete'}));
    expect(onDelete).toHaveBeenCalledWith(flows[0]);
  });

  it('offers no delete action without a delete handler', () => {
    render(<FlowTable onOpen={vi.fn()} />);
    const row = rowOf('Suspend users');

    expect(within(row).getByRole('button', {name: 'Edit'})).toBeInTheDocument();
    expect(within(row).queryByRole('button', {name: 'Delete'})).not.toBeInTheDocument();
  });

  it('offers only View on a read-only flow, even with a delete handler', async () => {
    const onOpen = vi.fn();
    const onDelete = vi.fn();
    render(<FlowTable onOpen={onOpen} onDelete={onDelete} />);
    const row = rowOf('Default sign-in');

    expect(within(row).queryByRole('button', {name: 'Edit'})).not.toBeInTheDocument();
    expect(within(row).queryByRole('button', {name: 'Delete'})).not.toBeInTheDocument();
    await userEvent.click(within(row).getByRole('button', {name: 'View'}));
    expect(onOpen).toHaveBeenCalledWith(flows[1]);
  });
});
