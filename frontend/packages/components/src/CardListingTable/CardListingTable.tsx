// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useDataGridLocaleText} from '@thunderid/hooks';
import {ListingTable, Skeleton, TablePagination, Typography} from '@wso2/oxygen-ui';
import {useEffect, useMemo, useState, type JSX, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';

/**
 * A column of a card listing table.
 *
 * @public
 */
export interface CardListingTableColumn<T> {
  /** Unique key of the column. */
  field: string;

  /** Header content of the column. */
  header: ReactNode;

  /** Renders the cell of a row. */
  render: (row: T) => ReactNode;

  /** The value a row sorts on. Without it, the column is not sortable. */
  sortValue?: (row: T) => string | number | null | undefined;

  /** Width of the column. */
  width?: number | string;

  /** Minimum width of the column. */
  minWidth?: number | string;

  /** Horizontal alignment of the header and cells. */
  align?: 'left' | 'center' | 'right';
}

/**
 * The page a card listing table shows.
 *
 * @public
 */
export interface CardListingTablePaginationModel {
  page: number;
  pageSize: number;
}

/**
 * Props of {@link CardListingTable}.
 *
 * @public
 */
export interface CardListingTableProps<T> {
  /** Rows of the current page in server mode, or of every page in client mode. */
  rows: T[];

  columns: CardListingTableColumn<T>[];

  getRowId: (row: T) => string;

  /** Opens the row. Without it, rows are not clickable. */
  onRowClick?: (row: T) => void;

  /** Shows skeleton rows in place of the data. */
  loading?: boolean;

  /** Text shown when there are no rows. Defaults to "No rows". */
  emptyLabel?: string;

  pageSizeOptions?: number[];

  /** Hides the pagination controls, for short lists that always fit on one page. */
  hidePagination?: boolean;

  /** `client` pages the rows in the browser. `server` shows rows that are one page the server returned. */
  paginationMode?: 'client' | 'server';

  /** Rows per page at first, in client mode. */
  defaultPageSize?: number;

  /** Total number of rows across every page, in server mode. */
  rowCount?: number;

  /** The page to show, in server mode. */
  paginationModel?: CardListingTablePaginationModel;

  /** Called with the page to show next, in server mode. */
  onPaginationModelChange?: (model: CardListingTablePaginationModel) => void;
}

type SortDirection = 'asc' | 'desc';

const SKELETON_ROW_COUNT = 3;

function compareSortValues(a: string | number | null | undefined, b: string | number | null | undefined): number {
  if (a === b) return 0;
  if (a === null || a === undefined) return 1;
  if (b === null || b === undefined) return -1;
  if (typeof a === 'number' && typeof b === 'number') return a - b;

  return String(a).localeCompare(String(b));
}

/**
 * The card table of Oxygen UI's `ListingTable` for a page's main resource list, where each row opens its own page.
 * Adds what the HTML table leaves to the caller: client or server paging, sorting on the loaded rows, skeleton rows
 * while loading, and an empty row.
 *
 * @public
 */
export default function CardListingTable<T>({
  rows,
  columns,
  getRowId,
  onRowClick = undefined,
  loading = false,
  emptyLabel = undefined,
  pageSizeOptions = [5, 10, 25],
  hidePagination = false,
  paginationMode = 'client',
  defaultPageSize = 10,
  rowCount = undefined,
  paginationModel = undefined,
  onPaginationModelChange = undefined,
}: CardListingTableProps<T>): JSX.Element {
  const {t} = useTranslation();
  const localeText = useDataGridLocaleText();
  const [sortField, setSortField] = useState<string | null>(null);
  const [sortDirection, setSortDirection] = useState<SortDirection>('asc');
  const [clientPaginationModel, setClientPaginationModel] = useState<CardListingTablePaginationModel>({
    page: 0,
    pageSize: defaultPageSize,
  });

  const isServerMode = paginationMode === 'server';
  const totalRowCount = isServerMode ? (rowCount ?? 0) : rows.length;
  const currentPaginationModel = (isServerMode ? paginationModel : undefined) ?? clientPaginationModel;
  const lastPage = Math.max(0, Math.ceil(totalRowCount / currentPaginationModel.pageSize) - 1);
  const page = Math.min(currentPaginationModel.page, lastPage);

  // When the last page empties (e.g. its last row was deleted), move the server back to the last page that has rows.
  useEffect(() => {
    if (isServerMode && !loading && page !== currentPaginationModel.page) {
      onPaginationModelChange?.({...currentPaginationModel, page});
    }
  }, [currentPaginationModel, isServerMode, loading, onPaginationModelChange, page]);

  const sortedRows = useMemo(() => {
    const sortColumn = columns.find((column) => column.field === sortField);
    if (!sortColumn?.sortValue) return rows;
    const {sortValue} = sortColumn;
    const factor = sortDirection === 'asc' ? 1 : -1;

    return [...rows].sort((a, b) => factor * compareSortValues(sortValue(a), sortValue(b)));
  }, [columns, rows, sortDirection, sortField]);

  const visibleRows = isServerMode
    ? sortedRows
    : sortedRows.slice(page * currentPaginationModel.pageSize, (page + 1) * currentPaginationModel.pageSize);

  const handlePaginationModelChange = (model: CardListingTablePaginationModel): void => {
    if (isServerMode) {
      onPaginationModelChange?.(model);
    } else {
      setClientPaginationModel(model);
    }
  };

  const handleSortClick = (field: string): void => {
    if (sortField === field) {
      setSortDirection(sortDirection === 'asc' ? 'desc' : 'asc');
    } else {
      setSortField(field);
      setSortDirection('asc');
    }
  };

  const renderBody = (): ReactNode => {
    if (loading) {
      return Array.from({length: SKELETON_ROW_COUNT}, (_, index) => (
        <ListingTable.Row key={`skeleton-${index}`} variant="card">
          {columns.map((column) => (
            <ListingTable.Cell key={column.field} align={column.align}>
              <Skeleton variant="text" />
            </ListingTable.Cell>
          ))}
        </ListingTable.Row>
      ));
    }

    if (visibleRows.length === 0) {
      return (
        <ListingTable.Row variant="card">
          <ListingTable.Cell colSpan={columns.length} align="center">
            <Typography variant="body2" color="text.secondary">
              {emptyLabel ?? t('common:dataTable.noRowsLabel', 'No rows')}
            </Typography>
          </ListingTable.Cell>
        </ListingTable.Row>
      );
    }

    return visibleRows.map((row) => (
      <ListingTable.Row
        key={getRowId(row)}
        variant="card"
        hover
        clickable={Boolean(onRowClick)}
        onClick={onRowClick ? () => onRowClick(row) : undefined}
      >
        {columns.map((column) => (
          <ListingTable.Cell key={column.field} align={column.align}>
            {column.render(row)}
          </ListingTable.Cell>
        ))}
      </ListingTable.Row>
    ));
  };

  return (
    <ListingTable.Container disablePaper>
      <ListingTable variant="card" aria-busy={loading}>
        <ListingTable.Head>
          <ListingTable.Row>
            {columns.map((column) => {
              const isSorted = sortField === column.field;
              let ariaSort: 'ascending' | 'descending' | undefined;
              if (isSorted) {
                ariaSort = sortDirection === 'asc' ? 'ascending' : 'descending';
              }

              return (
                <ListingTable.Cell
                  key={column.field}
                  align={column.align}
                  aria-sort={ariaSort}
                  sx={{width: column.width, minWidth: column.minWidth}}
                >
                  {column.sortValue ? (
                    <ListingTable.SortLabel
                      active={isSorted}
                      direction={isSorted ? sortDirection : 'asc'}
                      onClick={() => handleSortClick(column.field)}
                    >
                      {column.header}
                    </ListingTable.SortLabel>
                  ) : (
                    column.header
                  )}
                </ListingTable.Cell>
              );
            })}
          </ListingTable.Row>
        </ListingTable.Head>
        <ListingTable.Body>{renderBody()}</ListingTable.Body>
      </ListingTable>
      {!hidePagination && (
        <TablePagination
          component="div"
          count={totalRowCount}
          page={page}
          rowsPerPage={currentPaginationModel.pageSize}
          rowsPerPageOptions={pageSizeOptions}
          onPageChange={(_event, newPage) => handlePaginationModelChange({...currentPaginationModel, page: newPage})}
          onRowsPerPageChange={(event) => handlePaginationModelChange({page: 0, pageSize: Number(event.target.value)})}
          labelRowsPerPage={localeText.paginationRowsPerPage}
          labelDisplayedRows={(args) => localeText.paginationDisplayedRows?.({...args, estimated: undefined})}
          getItemAriaLabel={(type) => {
            switch (type) {
              case 'first':
                return t('common:dataTable.paginationFirstPage', 'Go to first page');
              case 'last':
                return t('common:dataTable.paginationLastPage', 'Go to last page');
              case 'next':
                return t('common:dataTable.paginationNextPage', 'Go to next page');
              default:
                return t('common:dataTable.paginationPreviousPage', 'Go to previous page');
            }
          }}
        />
      )}
    </ListingTable.Container>
  );
}
