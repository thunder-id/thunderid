// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Chip, IconButton, ListingTable, Tooltip} from '@wso2/oxygen-ui';
import {Eye, Pencil, Trash2, Workflow} from '@wso2/oxygen-ui-icons-react';
import {useMemo, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import type {CardListingTableColumn} from '../CardListingTable/CardListingTable';

/**
 * The fields of a flow a flow list shows.
 *
 * @public
 */
export interface FlowListRow {
  id: string;
  handle: string;
  name: string;
  flowType: string;
  updatedAt: string;
  isReadOnly?: boolean;
}

/**
 * Row actions of a flow list.
 *
 * @public
 */
export interface FlowListColumnsOptions<T extends FlowListRow> {
  /** Opens the flow in the flow editor. */
  onOpen: (flow: T) => void;

  /** Deletes the flow. Without it, rows offer no delete action. */
  onDelete?: (flow: T) => void;
}

function formatUpdatedAt(row: FlowListRow): string {
  const date = new Date(row.updatedAt);
  return date.toLocaleDateString(undefined, {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}

/**
 * Columns of a flow list: name, type, last update and the row actions. A read-only flow offers View;
 * any other flow offers Edit, and Delete when `onDelete` is given.
 *
 * @public
 */
export default function useFlowListColumns<T extends FlowListRow>({
  onOpen,
  onDelete = undefined,
}: FlowListColumnsOptions<T>): CardListingTableColumn<T>[] {
  const {t} = useTranslation();

  return useMemo(
    (): CardListingTableColumn<T>[] => [
      {
        field: 'name',
        header: t('flows:listing.columns.name', 'Name'),
        sortValue: (row) => row.name,
        minWidth: 220,
        render: (row: T): JSX.Element => (
          <ListingTable.CellIcon
            icon={<Workflow size={20} />}
            primary={row.name}
            secondary={row.handle}
            sx={{width: '100%'}}
          />
        ),
      },
      {
        field: 'flowType',
        header: t('flows:listing.columns.flowType', 'Type'),
        sortValue: (row) => row.flowType,
        width: 180,
        render: (row: T): JSX.Element => <Chip label={row.flowType} size="small" variant="outlined" />,
      },
      {
        field: 'updatedAt',
        header: t('flows:listing.columns.updatedAt', 'Last updated'),
        width: 180,
        sortValue: (row) => {
          const time = new Date(row.updatedAt).getTime();
          return Number.isNaN(time) ? null : time;
        },
        render: formatUpdatedAt,
      },
      {
        field: 'actions',
        header: t('flows:listing.columns.actions', 'Actions'),
        width: 150,
        align: 'center',
        render: (row: T): JSX.Element => (
          <ListingTable.RowActions>
            {row.isReadOnly ? (
              <Tooltip title={t('common:actions.view', 'View')}>
                <IconButton
                  size="small"
                  aria-label={t('common:actions.view', 'View')}
                  onClick={(e) => {
                    e.stopPropagation();
                    onOpen(row);
                  }}
                >
                  <Eye size={16} />
                </IconButton>
              </Tooltip>
            ) : (
              <>
                <Tooltip title={t('common:actions.edit', 'Edit')}>
                  <IconButton
                    size="small"
                    aria-label={t('common:actions.edit', 'Edit')}
                    onClick={(e) => {
                      e.stopPropagation();
                      onOpen(row);
                    }}
                  >
                    <Pencil size={16} />
                  </IconButton>
                </Tooltip>
                {onDelete && (
                  <Tooltip title={t('common:actions.delete', 'Delete')}>
                    <IconButton
                      size="small"
                      color="error"
                      aria-label={t('common:actions.delete', 'Delete')}
                      onClick={(e) => {
                        e.stopPropagation();
                        onDelete(row);
                      }}
                    >
                      <Trash2 size={16} />
                    </IconButton>
                  </Tooltip>
                )}
              </>
            )}
          </ListingTable.RowActions>
        ),
      },
    ],
    [onDelete, onOpen, t],
  );
}
