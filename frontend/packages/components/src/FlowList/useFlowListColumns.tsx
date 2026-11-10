// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useCapabilitiesContext} from '@thunderid/contexts';
import {Chip, DataGrid, IconButton, ListingTable, Tooltip} from '@wso2/oxygen-ui';
import {Eye, Trash2} from '@wso2/oxygen-ui-icons-react';
import {useMemo, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import EditOrViewAction from '../EditOrViewAction/EditOrViewAction';
import {useDeployStatusSource} from '../ResourceDeployStatus/DeployStatusContext';
import ResourceDeployStatus from '../ResourceDeployStatus/ResourceDeployStatus';

/**
 * The fields of a flow a flow list shows.
 *
 * @public
 */
export interface FlowListRow {
  id: string;
  name: string;
  flowType: string;
  updatedAt: string;
  isReadOnly?: boolean;
  isOwnedByOU?: boolean;
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

/**
 * Columns of a flow list: name, type, last update and the row actions. A read-only flow offers View;
 * any other flow offers Edit (View when the admin cannot update flows), and Delete when `onDelete` is given.
 *
 * @public
 */
export default function useFlowListColumns<T extends FlowListRow>({
  onOpen,
  onDelete = undefined,
}: FlowListColumnsOptions<T>): DataGrid.GridColDef<T>[] {
  const {t} = useTranslation();
  const {can} = useCapabilitiesContext();
  const canUpdate = can('flow', 'update');
  // Where flows are along the deploy path is shown only where the app says so.
  const deploys = useDeployStatusSource() !== null;

  return useMemo(
    (): DataGrid.GridColDef<T>[] => [
      {
        field: 'name',
        headerName: t('flows:listing.columns.name', 'Name'),
        flex: 1,
        minWidth: 220,
      },
      {
        field: 'flowType',
        headerName: t('flows:listing.columns.flowType', 'Type'),
        width: 180,
        renderCell: (params: DataGrid.GridRenderCellParams<T>): JSX.Element => (
          <Chip
            label={params.row.flowType}
            size="small"
            color="primary"
            variant="outlined"
            sx={{
              fontSize: '0.7rem',
            }}
          />
        ),
      },
      {
        field: 'updatedAt',
        headerName: t('flows:listing.columns.updatedAt', 'Last updated'),
        width: 180,
        valueGetter: (_value, row): string => {
          const date = new Date(row.updatedAt);
          return date.toLocaleDateString(undefined, {
            year: 'numeric',
            month: 'short',
            day: 'numeric',
            hour: '2-digit',
            minute: '2-digit',
          });
        },
      },
      ...(deploys
        ? [
            {
              field: 'deployment',
              headerName: t('flows:listing.columns.deployment', 'Deployment'),
              width: 170,
              sortable: false,
              renderCell: (params: DataGrid.GridRenderCellParams<T>): JSX.Element => (
                <ResourceDeployStatus resourceType="flow" id={params.row.id} tabIndex={params.tabIndex} />
              ),
            } satisfies DataGrid.GridColDef<T>,
          ]
        : []),
      {
        field: 'actions',
        headerName: t('flows:listing.columns.actions', 'Actions'),
        width: 150,
        align: 'center',
        headerAlign: 'center',
        sortable: false,
        filterable: false,
        hideable: false,
        renderCell: (params: DataGrid.GridRenderCellParams<T>): JSX.Element => (
          <ListingTable.RowActions>
            {params.row.isReadOnly || params.row.isOwnedByOU === false ? (
              <Tooltip title={t('common:actions.view', 'View')}>
                <IconButton
                  aria-label={t('common:actions.view', 'View')}
                  size="small"
                  onClick={(e) => {
                    e.stopPropagation();
                    onOpen(params.row);
                  }}
                >
                  <Eye size={16} />
                </IconButton>
              </Tooltip>
            ) : (
              <>
                <EditOrViewAction canEdit={canUpdate} onClick={() => onOpen(params.row)} />
                {onDelete && (
                  <Tooltip title={t('common:actions.delete', 'Delete')}>
                    <IconButton
                      aria-label={t('common:actions.delete', 'Delete')}
                      size="small"
                      color="error"
                      onClick={(e) => {
                        e.stopPropagation();
                        onDelete(params.row);
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
    [canUpdate, deploys, onDelete, onOpen, t],
  );
}
