// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {EditOrViewAction, QueryErrorNotice} from '@thunderid/components';
import {useCapabilitiesContext, useProjectScope} from '@thunderid/contexts';
import {useDataGridLocaleText} from '@thunderid/hooks';
import {useLogger} from '@thunderid/logger/react';
import {Box, DataGrid, IconButton, ListingTable, Stack, Tooltip, Typography} from '@wso2/oxygen-ui';
import {Eye, Trash2} from '@wso2/oxygen-ui-icons-react';
import {useCallback, useEffect, useMemo, useRef, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import AuthorizationDetailTypeDeleteDialog from './AuthorizationDetailTypeDeleteDialog';
import useGetAuthorizationDetailTypes from '../../api/useGetAuthorizationDetailTypes';
import useAuthorizationDetailTypeRoutes from '../../hooks/useAuthorizationDetailTypeRoutes';
import type {AuthorizationDetailType} from '../../models/authorization-detail-type';

/**
 * Lists the RFC 9396 authorization details types, opens them for editing, and deletes them. Types
 * defined in declarative configuration can only be viewed. Inside a project only the project's types are listed.
 */
export default function AuthorizationDetailTypesList(): JSX.Element {
  const {t} = useTranslation();
  const navigate = useNavigate();
  const routes = useAuthorizationDetailTypeRoutes();
  const logger = useLogger('AuthorizationDetailTypesList');
  const dataGridLocaleText = useDataGridLocaleText();
  const {can} = useCapabilitiesContext();
  const {projectId} = useProjectScope();
  const [paginationModel, setPaginationModel] = useState<DataGrid.GridPaginationModel>({pageSize: 10, page: 0});
  const {data, isLoading, isPlaceholderData, error, refetch} = useGetAuthorizationDetailTypes({
    limit: paginationModel.pageSize,
    offset: paginationModel.page * paginationModel.pageSize,
    projectId,
  });
  // A page left empty, as when its last type is deleted, gives way to the page before it.
  if (!isPlaceholderData && data?.types.length === 0 && paginationModel.page > 0) {
    setPaginationModel({...paginationModel, page: paginationModel.page - 1});
  }

  const [deleteTarget, setDeleteTarget] = useState<AuthorizationDetailType | null>(null);
  // The row a deleted type was on is gone once the list loads again, so focus moves to the list rather
  // than to the page.
  const [focusAfterDelete, setFocusAfterDelete] = useState<string | null>(null);
  const focusedAfterDelete = useRef<string | null>(null);
  const listRef = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (
      !focusAfterDelete ||
      focusAfterDelete === focusedAfterDelete.current ||
      data?.types.some((detailType) => detailType.id === focusAfterDelete)
    )
      return;
    focusedAfterDelete.current = focusAfterDelete;
    const list = listRef.current;
    // The header of the first column, as the cell that had focus may still be the deleted row's.
    (list?.querySelector<HTMLElement>('[role="columnheader"]') ?? list)?.focus();
  }, [focusAfterDelete, data]);

  // Resolves an error through the `authorizationDetailTypes` catalog. `t` defaults to the `common`
  // namespace, so this forwards explicit `ns:` prefixes unchanged and prefixes bare keys with
  // `authorizationDetailTypes:`, per getErrorMessage's namespace-resolution contract.
  const tForErrors = useCallback(
    (key: string, options?: Record<string, unknown>): string =>
      t(key.includes(':') ? key : `authorizationDetailTypes:${key}`, options),
    [t],
  );

  const openEditor = useCallback(
    (detailType: AuthorizationDetailType): void => {
      (async (): Promise<void> => {
        await navigate(routes.detail(detailType.id));
      })().catch((err: unknown) => {
        logger.error('Failed to navigate to intent', {error: err});
      });
    },
    [navigate, routes, logger],
  );

  const columns: DataGrid.GridColDef<AuthorizationDetailType>[] = useMemo(
    () => [
      {
        field: 'name',
        headerName: t('authorizationDetailTypes:listing.columns.name', 'Name'),
        flex: 1,
        minWidth: 200,
        renderCell: (params: DataGrid.GridRenderCellParams<AuthorizationDetailType>) => (
          <Typography variant="body2" fontWeight={500}>
            {params.row.name}
          </Typography>
        ),
      },
      {
        field: 'description',
        headerName: t('authorizationDetailTypes:listing.columns.description', 'Description'),
        flex: 1.5,
        minWidth: 250,
        valueGetter: (_value, row: AuthorizationDetailType): string => row.description ?? '-',
      },
      {
        field: 'type',
        headerName: t('authorizationDetailTypes:listing.columns.type', 'Type identifier'),
        flex: 1.2,
        minWidth: 200,
        renderCell: (params: DataGrid.GridRenderCellParams<AuthorizationDetailType>) => (
          // A long identifier ends in an ellipsis, and shows in full on hover.
          <Box sx={{display: 'flex', alignItems: 'center', height: '100%', overflow: 'hidden'}}>
            <Typography
              variant="body2"
              color="text.secondary"
              noWrap
              title={params.row.type}
              sx={{fontFamily: 'monospace', fontSize: '0.8rem'}}
            >
              {params.row.type}
            </Typography>
          </Box>
        ),
      },
      {
        field: 'version',
        headerName: t('authorizationDetailTypes:listing.columns.version', 'Version'),
        width: 100,
      },
      {
        field: 'actions',
        headerName: t('authorizationDetailTypes:listing.columns.actions', 'Actions'),
        width: 150,
        align: 'center',
        headerAlign: 'center',
        sortable: false,
        filterable: false,
        hideable: false,
        renderCell: (params: DataGrid.GridRenderCellParams<AuthorizationDetailType>): JSX.Element => (
          <ListingTable.RowActions>
            {params.row.isReadOnly ? (
              <Tooltip title={t('common:status.readOnly', 'Read only')}>
                <IconButton
                  size="small"
                  disableRipple
                  aria-label={t('common:status.readOnly', 'Read only')}
                  sx={{cursor: 'default'}}
                >
                  <Eye size={16} />
                </IconButton>
              </Tooltip>
            ) : (
              <>
                <EditOrViewAction canEdit={can('authzdetail', 'update')} onClick={() => openEditor(params.row)} />
                {can('authzdetail', 'delete') && (
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
                )}
              </>
            )}
          </ListingTable.RowActions>
        ),
      },
    ],
    [can, t, openEditor],
  );

  return (
    <Stack spacing={2} ref={listRef} tabIndex={-1} sx={{outline: 'none'}}>
      {error ? (
        <QueryErrorNotice
          error={error}
          t={tForErrors}
          variant="block"
          title={t('authorizationDetailTypes:listing.error', 'Failed to load intents')}
          onRetry={() => void refetch()}
        />
      ) : (
        <ListingTable.Provider variant="data-grid-card" loading={isLoading}>
          <ListingTable.Container disablePaper>
            <ListingTable.DataGrid
              rows={data?.types ?? []}
              columns={columns}
              getRowId={(row) => (row as AuthorizationDetailType).id}
              onRowClick={(params) => openEditor(params.row as AuthorizationDetailType)}
              rowCount={data?.totalResults ?? 0}
              paginationMode="server"
              paginationModel={paginationModel}
              onPaginationModelChange={setPaginationModel}
              pageSizeOptions={[5, 10, 25]}
              disableRowSelectionOnClick
              // Filtering is not wired end to end, so the column filter panel stays hidden.
              disableColumnFilter
              localeText={dataGridLocaleText}
              autoHeight
              sx={{'& .MuiDataGrid-row': {cursor: 'pointer'}}}
            />
          </ListingTable.Container>
        </ListingTable.Provider>
      )}

      <AuthorizationDetailTypeDeleteDialog
        detailType={deleteTarget}
        onClose={() => setDeleteTarget(null)}
        onSuccess={() => setFocusAfterDelete(deleteTarget?.id ?? null)}
      />
    </Stack>
  );
}
