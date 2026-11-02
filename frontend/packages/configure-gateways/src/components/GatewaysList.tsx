// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {QueryErrorNotice} from '@thunderid/components';
import {useDataGridLocaleText} from '@thunderid/hooks';
import {useLogger} from '@thunderid/logger/react';
import {Chip, DataGrid, IconButton, ListingTable, Stack, Tooltip, Typography} from '@wso2/oxygen-ui';
import {Network, Pencil, Trash2} from '@wso2/oxygen-ui-icons-react';
import {useCallback, useMemo, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import GatewayDeleteDialog from './GatewayDeleteDialog';
import useGetGateways from '../api/useGetGateways';
import useGatewayErrorTranslator from '../hooks/useGatewayErrorTranslator';
import useGatewayRoutes from '../hooks/useGatewayRoutes';
import type {Gateway} from '../models/gateway';
import formatTimestamp from '../utils/formatTimestamp';

export default function GatewaysList(): JSX.Element {
  const navigate = useNavigate();
  const routes = useGatewayRoutes();
  const {t} = useTranslation();
  const tForErrors = useGatewayErrorTranslator();
  const logger = useLogger('GatewaysList');
  const dataGridLocaleText = useDataGridLocaleText();
  const {data, isLoading, error, refetch} = useGetGateways();
  const [deleteTarget, setDeleteTarget] = useState<Gateway | null>(null);

  const openDetail = useCallback(
    (id: string): void => {
      (async (): Promise<void> => {
        await navigate(routes.detail(id));
      })().catch((err: unknown) => {
        logger.error('Failed to navigate to gateway detail', {error: err});
      });
    },
    [navigate, routes, logger],
  );

  const columns: DataGrid.GridColDef<Gateway>[] = useMemo(
    () => [
      {
        field: 'name',
        headerName: t('gateways:list.columns.name', 'Name'),
        flex: 1,
        minWidth: 180,
        renderCell: (params: DataGrid.GridRenderCellParams<Gateway>) => (
          <ListingTable.CellIcon
            icon={<Network size={20} />}
            primary={
              <Stack direction="row" spacing={1} alignItems="center">
                <span>{params.row.name}</span>
                {params.row.isDefault && (
                  <Chip size="small" variant="outlined" label={t('gateways:default', 'Default')} />
                )}
              </Stack>
            }
            sx={{width: '100%'}}
          />
        ),
      },
      {
        field: 'baseUrl',
        headerName: t('gateways:list.columns.baseUrl', 'Base URL'),
        flex: 1.5,
        minWidth: 240,
        renderCell: (params: DataGrid.GridRenderCellParams<Gateway>) => (
          <Typography variant="body2" color="text.secondary" sx={{fontFamily: 'monospace', fontSize: '0.8rem'}}>
            {params.row.baseUrl}
          </Typography>
        ),
      },
      {
        field: 'createdAt',
        headerName: t('gateways:list.columns.createdAt', 'Created'),
        width: 200,
        renderCell: (params: DataGrid.GridRenderCellParams<Gateway>) => (
          <Typography variant="body2" color="text.secondary">
            {formatTimestamp(params.row.createdAt)}
          </Typography>
        ),
      },
      {
        field: 'actions',
        headerName: t('gateways:list.columns.actions', 'Actions'),
        width: 120,
        align: 'center',
        headerAlign: 'center',
        sortable: false,
        filterable: false,
        hideable: false,
        renderCell: (params: DataGrid.GridRenderCellParams<Gateway>): JSX.Element => (
          <ListingTable.RowActions>
            <Tooltip title={t('common:actions.edit', 'Edit')}>
              <IconButton
                size="small"
                aria-label={t('common:actions.edit', 'Edit')}
                onClick={(e) => {
                  e.stopPropagation();
                  openDetail(params.row.id);
                }}
              >
                <Pencil size={16} />
              </IconButton>
            </Tooltip>
            <Tooltip title={t('common:actions.delete', 'Delete')}>
              <IconButton
                size="small"
                color="error"
                aria-label={t('common:actions.delete', 'Delete')}
                onClick={(e) => {
                  e.stopPropagation();
                  setDeleteTarget(params.row);
                }}
              >
                <Trash2 size={16} />
              </IconButton>
            </Tooltip>
          </ListingTable.RowActions>
        ),
      },
    ],
    [t, openDetail],
  );

  // A refetch that fails keeps the gateways already read, so the list stays and says so above it.
  if (error && !data) {
    return (
      <QueryErrorNotice
        error={error}
        t={tForErrors}
        variant="block"
        title={t('gateways:list.error', 'Failed to load gateways')}
        onRetry={() => void refetch()}
      />
    );
  }

  return (
    <>
      {error && <QueryErrorNotice error={error} t={tForErrors} variant="inline" onRetry={() => void refetch()} />}
      <ListingTable.Provider variant="data-grid" loading={isLoading}>
        <ListingTable.Container>
          <ListingTable.DataGrid
            rows={data ?? []}
            columns={columns}
            getRowId={(row) => (row as Gateway).id}
            onRowClick={(params) => openDetail((params.row as Gateway).id)}
            disableRowSelectionOnClick
            disableColumnFilter
            localeText={{
              ...dataGridLocaleText,
              noRowsLabel: t('gateways:list.empty', 'No gateways are registered yet.'),
            }}
            initialState={{pagination: {paginationModel: {pageSize: 10}}}}
            pageSizeOptions={[5, 10, 25]}
            autoHeight
            sx={{'& .MuiDataGrid-row': {cursor: 'pointer'}}}
          />
        </ListingTable.Container>
      </ListingTable.Provider>

      <GatewayDeleteDialog
        open={deleteTarget !== null}
        gateway={deleteTarget}
        onClose={() => setDeleteTarget(null)}
        onSuccess={() => setDeleteTarget(null)}
      />
    </>
  );
}
