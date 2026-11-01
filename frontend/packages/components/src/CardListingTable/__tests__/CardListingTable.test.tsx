// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {screen, within} from '@testing-library/react';
import {render, userEvent} from '@thunderid/test-utils';
import {IconButton} from '@wso2/oxygen-ui';
import {useState, type JSX} from 'react';
import {describe, expect, it, vi} from 'vitest';
import CardListingTable, {type CardListingTableColumn, type CardListingTablePaginationModel} from '../CardListingTable';

// The package initializes no i18next instance, so the fallback is the rendered copy.
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, fallback?: string) => (typeof fallback === 'string' ? fallback : key),
  }),
}));

vi.mock('@thunderid/hooks', () => ({
  useDataGridLocaleText: () => ({
    paginationRowsPerPage: 'Rows per page:',
    paginationDisplayedRows: ({from, to, count}: {from: number; to: number; count: number}) =>
      `${from}–${to} of ${count}`,
  }),
}));

interface Item {
  id: string;
  name: string;
  size: number;
}

const items: Item[] = [
  {id: 'b', name: 'Beta', size: 2},
  {id: 'a', name: 'Alpha', size: 3},
  {id: 'c', name: 'Gamma', size: 1},
];

const columns: CardListingTableColumn<Item>[] = [
  {field: 'name', header: 'Name', render: (row) => row.name, sortValue: (row) => row.name},
  {field: 'size', header: 'Size', render: (row) => row.size, sortValue: (row) => row.size},
  {
    field: 'actions',
    header: 'Actions',
    align: 'right',
    width: 120,
    render: (row) => (
      <IconButton aria-label={`Delete ${row.name}`} onClick={(e) => e.stopPropagation()}>
        x
      </IconButton>
    ),
  },
];

const bodyRows = (): HTMLElement[] => screen.getAllByRole('row').slice(1);
const names = (): string[] => bodyRows().map((row) => within(row).getAllByRole('cell')[0].textContent ?? '');

describe('CardListingTable', () => {
  it('renders the card variant of the listing table with a header row and a row per item', () => {
    render(<CardListingTable rows={items} columns={columns} getRowId={(row) => row.id} />);

    expect(screen.getByRole('table')).toBeInTheDocument();
    expect(screen.getByRole('columnheader', {name: 'Name'})).toBeInTheDocument();
    expect(screen.getByRole('columnheader', {name: 'Actions'})).toBeInTheDocument();
    expect(names()).toEqual(['Beta', 'Alpha', 'Gamma']);
  });

  it('opens a row on click, but not when a row action is clicked', async () => {
    const onRowClick = vi.fn();
    render(<CardListingTable rows={items} columns={columns} getRowId={(row) => row.id} onRowClick={onRowClick} />);

    await userEvent.click(screen.getByRole('button', {name: 'Delete Alpha'}));
    expect(onRowClick).not.toHaveBeenCalled();

    await userEvent.click(screen.getByText('Alpha'));
    expect(onRowClick).toHaveBeenCalledWith(items[1]);
  });

  it('sorts by a sortable column ascending, then descending, and marks the header', async () => {
    render(<CardListingTable rows={items} columns={columns} getRowId={(row) => row.id} />);
    const header = screen.getByRole('columnheader', {name: 'Name'});
    expect(header).not.toHaveAttribute('aria-sort');

    await userEvent.click(within(header).getByRole('button', {name: 'Name'}));
    expect(names()).toEqual(['Alpha', 'Beta', 'Gamma']);
    expect(header).toHaveAttribute('aria-sort', 'ascending');

    await userEvent.click(within(header).getByRole('button', {name: 'Name'}));
    expect(names()).toEqual(['Gamma', 'Beta', 'Alpha']);
    expect(header).toHaveAttribute('aria-sort', 'descending');
  });

  it('sorts numbers numerically and resets the direction when another column is sorted', async () => {
    render(<CardListingTable rows={items} columns={columns} getRowId={(row) => row.id} />);

    await userEvent.click(screen.getByRole('button', {name: 'Name'}));
    await userEvent.click(screen.getByRole('button', {name: 'Name'}));
    await userEvent.click(screen.getByRole('button', {name: 'Size'}));

    expect(names()).toEqual(['Gamma', 'Beta', 'Alpha']);
    expect(screen.getByRole('columnheader', {name: 'Size'})).toHaveAttribute('aria-sort', 'ascending');
    expect(screen.getByRole('columnheader', {name: 'Name'})).not.toHaveAttribute('aria-sort');
  });

  it('places rows without a sort value last', async () => {
    const rows = [...items, {id: 'd', name: '', size: 0}];
    const sparseColumns: CardListingTableColumn<Item>[] = [
      {field: 'name', header: 'Name', render: (row) => row.name || '-', sortValue: (row) => row.name || null},
    ];
    render(<CardListingTable rows={rows} columns={sparseColumns} getRowId={(row) => row.id} />);

    await userEvent.click(screen.getByRole('button', {name: 'Name'}));

    expect(names()).toEqual(['Alpha', 'Beta', 'Gamma', '-']);
  });

  it('offers no sort control on a column without a sort value', () => {
    render(<CardListingTable rows={items} columns={columns} getRowId={(row) => row.id} />);

    expect(within(screen.getByRole('columnheader', {name: 'Actions'})).queryByRole('button')).not.toBeInTheDocument();
  });

  it('pages the rows in the browser', async () => {
    render(
      <CardListingTable
        rows={items}
        columns={columns}
        getRowId={(row) => row.id}
        defaultPageSize={2}
        pageSizeOptions={[2, 5]}
      />,
    );

    expect(names()).toEqual(['Beta', 'Alpha']);
    expect(screen.getByText('1–2 of 3')).toBeInTheDocument();

    await userEvent.click(screen.getByRole('button', {name: 'Go to next page'}));
    expect(names()).toEqual(['Gamma']);
    expect(screen.getByText('3–3 of 3')).toBeInTheDocument();
  });

  it('returns to the first page when the page size changes', async () => {
    render(
      <CardListingTable
        rows={items}
        columns={columns}
        getRowId={(row) => row.id}
        defaultPageSize={2}
        pageSizeOptions={[2, 5]}
      />,
    );

    await userEvent.click(screen.getByRole('button', {name: 'Go to next page'}));
    await userEvent.click(screen.getByRole('combobox'));
    await userEvent.click(screen.getByRole('option', {name: '5'}));

    expect(names()).toEqual(['Beta', 'Alpha', 'Gamma']);
  });

  it('stays on the last page that has rows when rows are removed', async () => {
    function Shrinking(): JSX.Element {
      const [rows, setRows] = useState(items);

      return (
        <>
          <button type="button" onClick={() => setRows(items.slice(0, 1))}>
            Shrink
          </button>
          <CardListingTable
            rows={rows}
            columns={columns}
            getRowId={(row) => row.id}
            defaultPageSize={2}
            pageSizeOptions={[2]}
          />
        </>
      );
    }
    render(<Shrinking />);

    await userEvent.click(screen.getByRole('button', {name: 'Go to next page'}));
    await userEvent.click(screen.getByRole('button', {name: 'Shrink'}));

    expect(names()).toEqual(['Beta']);
  });

  it('shows the rows it is given in server mode and reports page changes', async () => {
    const onPaginationModelChange = vi.fn<(model: CardListingTablePaginationModel) => void>();
    render(
      <CardListingTable
        rows={items.slice(0, 2)}
        columns={columns}
        getRowId={(row) => row.id}
        paginationMode="server"
        rowCount={12}
        paginationModel={{page: 0, pageSize: 2}}
        onPaginationModelChange={onPaginationModelChange}
        pageSizeOptions={[2, 5]}
      />,
    );

    expect(names()).toEqual(['Beta', 'Alpha']);
    expect(screen.getByText('1–2 of 12')).toBeInTheDocument();

    await userEvent.click(screen.getByRole('button', {name: 'Go to next page'}));
    expect(onPaginationModelChange).toHaveBeenCalledWith({page: 1, pageSize: 2});

    await userEvent.click(screen.getByRole('combobox'));
    await userEvent.click(screen.getByRole('option', {name: '5'}));
    expect(onPaginationModelChange).toHaveBeenCalledWith({page: 0, pageSize: 5});
  });

  it('moves the server back to the last page that has rows once that page empties', () => {
    const onPaginationModelChange = vi.fn<(model: CardListingTablePaginationModel) => void>();
    const props = {
      rows: [],
      columns,
      getRowId: (row: Item) => row.id,
      paginationMode: 'server' as const,
      rowCount: 4,
      paginationModel: {page: 2, pageSize: 2},
      onPaginationModelChange,
    };
    const {rerender} = render(<CardListingTable {...props} loading />);
    expect(onPaginationModelChange).not.toHaveBeenCalled();

    rerender(<CardListingTable {...props} />);
    expect(onPaginationModelChange).toHaveBeenCalledWith({page: 1, pageSize: 2});
  });

  it('names the page buttons', () => {
    render(<CardListingTable rows={items} columns={columns} getRowId={(row) => row.id} />);

    expect(screen.getByRole('button', {name: 'Go to previous page'})).toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'Go to next page'})).toBeInTheDocument();
    expect(screen.getByText('Rows per page:')).toBeInTheDocument();
  });

  it('can hide the pagination controls', () => {
    render(<CardListingTable rows={items} columns={columns} getRowId={(row) => row.id} hidePagination />);

    expect(screen.queryByRole('button', {name: 'Go to next page'})).not.toBeInTheDocument();
  });

  it('shows skeleton rows and marks the table busy while loading', () => {
    render(<CardListingTable rows={items} columns={columns} getRowId={(row) => row.id} loading />);

    expect(screen.getByRole('table')).toHaveAttribute('aria-busy', 'true');
    expect(screen.queryByText('Alpha')).not.toBeInTheDocument();
    expect(bodyRows()).toHaveLength(3);
  });

  it('shows "No rows" when there are no rows', () => {
    render(<CardListingTable rows={[]} columns={columns} getRowId={(row) => row.id} />);

    expect(screen.getByText('No rows')).toBeInTheDocument();
    expect(screen.getByText('No rows').closest('td')).toHaveAttribute('colspan', '3');
  });

  it('shows a custom empty label', () => {
    render(<CardListingTable rows={[]} columns={columns} getRowId={(row) => row.id} emptyLabel="Nothing here yet." />);

    expect(screen.getByText('Nothing here yet.')).toBeInTheDocument();
  });
});
