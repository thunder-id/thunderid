// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {AccessStatusIndicator, QueryErrorNotice, ResourceAvatar} from '@thunderid/components';
import {useConfig} from '@thunderid/contexts';
import {useDataGridLocaleText} from '@thunderid/hooks';
import {useLogger} from '@thunderid/logger/react';
import {Box, Chip, IconButton, Tooltip, Typography, ListingTable, DataGrid} from '@wso2/oxygen-ui';
import {Eye, Pencil, Trash2} from '@wso2/oxygen-ui-icons-react';
import {useCallback, useMemo, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import ApplicationDeleteDialog from './ApplicationDeleteDialog';
import useGetApplications from '../api/useGetApplications';
import ApplicationConstants from '../constants/application-constants';
import useApplicationRoutes from '../hooks/useApplicationRoutes';
import type {BasicApplication} from '../models/application';
import getApplicationErrorMessage from '../utils/getApplicationErrorMessage';
import getTemplateMetadata from '../utils/getTemplateMetadata';
import isSystemConsoleClient from '../utils/isSystemConsoleClient';

export default function ApplicationsList(): JSX.Element {
  const routes = useApplicationRoutes();
  const navigate = useNavigate();
  const {config} = useConfig();
  const {t} = useTranslation();
  const logger = useLogger('ApplicationsList');
  const dataGridLocaleText = useDataGridLocaleText();
  const {data, isLoading, error, refetch} = useGetApplications();
  const consoleClientId = config?.client?.client_id;

  // Resolves an error through the `applications` catalog. `t` defaults to the `common` namespace,
  // so this forwards explicit `ns:` prefixes unchanged and prefixes bare keys with `applications:`,
  // per getErrorMessage's namespace-resolution contract.
  const tForErrors = useCallback(
    (key: string, options?: Record<string, unknown>): string =>
      t(key.includes(':') ? key : `applications:${key}`, options),
    [t],
  );

  const [selectedAppId, setSelectedAppId] = useState<string | null>(null);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState<boolean>(false);

  const handleDeleteClick = useCallback((appId: string): void => {
    setSelectedAppId(appId);
    setDeleteDialogOpen(true);
  }, []);

  const handleEditClick = useCallback(
    (appId: string): void => {
      (async (): Promise<void> => {
        await navigate(routes.applications.detail(appId));
      })().catch((_error: unknown) => {
        logger.error('Failed to navigate to application', {error: _error, applicationId: appId});
      });
    },
    [logger, navigate, routes],
  );

  const handleDeleteDialogClose = (): void => {
    setDeleteDialogOpen(false);
    setSelectedAppId(null);
  };

  const columns: DataGrid.GridColDef<BasicApplication>[] = useMemo(
    () => [
      {
        field: 'name',
        headerName: t('applications:listing.columns.name'),
        flex: 2,
        minWidth: 260,
        renderCell: (params: DataGrid.GridRenderCellParams<BasicApplication>): JSX.Element => (
          <ListingTable.CellIcon
            sx={{width: '100%'}}
            icon={
              <ResourceAvatar
                variant="rounded"
                value={params.row.logoUrl}
                size={30}
                fallback={ApplicationConstants.DEFAULT_AVATAR}
              />
            }
            primary={params.row.name}
            secondary={params.row.description}
          />
        ),
      },
      {
        field: 'template',
        headerName: t('applications:listing.columns.template'),
        flex: 0.8,
        minWidth: 120,
        renderCell: (params: DataGrid.GridRenderCellParams<BasicApplication>): JSX.Element => {
          const templateMetadata = getTemplateMetadata(params.row.template);
          return templateMetadata ? (
            <Chip label={templateMetadata.displayName} size="small" variant="outlined" />
          ) : (
            <>-</>
          );
        },
      },
      {
        field: 'clientId',
        headerName: t('applications:listing.columns.clientId'),
        flex: 1,
        minWidth: 200,
        renderCell: (params: DataGrid.GridRenderCellParams<BasicApplication>): JSX.Element =>
          params.row.clientId ? (
            <Typography variant="body2" sx={{fontFamily: 'monospace', fontSize: '0.875rem'}}>
              {params.row.clientId}
            </Typography>
          ) : (
            <>-</>
          ),
      },
      {
        field: 'status',
        headerName: t('applications:listing.columns.status', 'Status'),
        flex: 0.4,
        minWidth: 110,
        sortable: false,
        renderCell: (params: DataGrid.GridRenderCellParams<BasicApplication>): JSX.Element => (
          <AccessStatusIndicator status={params.row.status} category="application" tabIndex={params.tabIndex} />
        ),
      },
      {
        field: 'actions',
        headerName: t('applications:listing.columns.actions'),
        width: 150,
        align: 'center',
        headerAlign: 'center',
        sortable: false,
        filterable: false,
        hideable: false,
        renderCell: (params: DataGrid.GridRenderCellParams<BasicApplication>): JSX.Element => (
          <ListingTable.RowActions>
            {params.row.isReadOnly ? (
              <Tooltip title={t('common:status.readOnly', 'Read only')}>
                <IconButton
                  size="small"
                  disableRipple
                  sx={{cursor: 'default'}}
                  aria-label={t('common:status.readOnly', 'Read only')}
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
                      handleEditClick(params.row.id);
                    }}
                  >
                    <Pencil size={16} />
                  </IconButton>
                </Tooltip>
                {!isSystemConsoleClient(params.row.clientId, consoleClientId) && (
                  <Tooltip title={t('common:actions.delete', 'Delete')}>
                    <IconButton
                      size="small"
                      color="error"
                      aria-label={t('common:actions.delete', 'Delete')}
                      onClick={(e) => {
                        e.stopPropagation();
                        handleDeleteClick(params.row.id);
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
    [consoleClientId, handleDeleteClick, handleEditClick, t],
  );

  if (error) {
    return (
      <QueryErrorNotice
        error={error}
        t={tForErrors}
        title={t('applications:listing.error', 'Failed to load applications')}
        resolveErrorMessage={getApplicationErrorMessage}
        onRetry={() => void refetch()}
      />
    );
  }

  return (
    <Box data-testid="applications-list">
      <ListingTable.Provider variant="data-grid" loading={isLoading}>
        <ListingTable.Container>
          <ListingTable.DataGrid
            rows={data?.applications ?? []}
            columns={columns}
            getRowId={(row): string => (row as BasicApplication).id}
            onRowClick={(params) => {
              handleEditClick((params.row as BasicApplication).id);
            }}
            initialState={{
              pagination: {
                paginationModel: {pageSize: 10},
              },
            }}
            pageSizeOptions={[5, 10, 25, 50]}
            disableRowSelectionOnClick
            // Filtering is not wired end to end, so the column filter panel stays hidden.
            disableColumnFilter
            localeText={dataGridLocaleText}
            rowHeight={64}
            autoHeight
            sx={{
              '& .MuiDataGrid-row': {
                cursor: 'pointer',
              },
            }}
          />
        </ListingTable.Container>
      </ListingTable.Provider>

      <ApplicationDeleteDialog
        open={deleteDialogOpen}
        applicationId={selectedAppId}
        onClose={handleDeleteDialogClose}
      />
    </Box>
  );
}
