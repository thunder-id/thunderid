// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {QueryErrorNotice} from '@thunderid/components';
import {useDataGridLocaleText} from '@thunderid/hooks';
import {useLogger} from '@thunderid/logger/react';
import {Box, DataGrid, IconButton, ListingTable, Tooltip, Typography} from '@wso2/oxygen-ui';
import {Pencil, Trash2} from '@wso2/oxygen-ui-icons-react';
import {useCallback, useMemo, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import NotificationTemplateDeleteDialog from './NotificationTemplateDeleteDialog';
import useGetNotificationTemplates from '../api/useGetNotificationTemplates';
import useNotificationTemplateRoutes from '../hooks/useNotificationTemplateRoutes';
import type {NotificationChannel, TemplateSummary} from '../models/notification-template';

/**
 * Props for the {@link NotificationTemplatesList} component.
 *
 * @public
 */
export interface NotificationTemplatesListProps {
  /**
   * The channel whose templates are listed.
   */
  channel: NotificationChannel;
}

/**
 * Table of notification templates for a single channel, with a Name column and row actions.
 * Edit opens the template editor; Delete is a placeholder wired later.
 *
 * @public
 */
export default function NotificationTemplatesList({channel}: NotificationTemplatesListProps): JSX.Element {
  const {t} = useTranslation('notificationTemplates');
  const navigate = useNavigate();
  const logger = useLogger('NotificationTemplatesList');
  const routes = useNotificationTemplateRoutes();
  const dataGridLocaleText = useDataGridLocaleText();

  const [paginationModel, setPaginationModel] = useState<DataGrid.GridPaginationModel>({pageSize: 10, page: 0});
  const {data, isLoading, error, refetch} = useGetNotificationTemplates(channel, {
    limit: paginationModel.pageSize,
    offset: paginationModel.page * paginationModel.pageSize,
  });

  const rows = useMemo(() => data?.templates ?? [], [data?.templates]);

  const [deleteTarget, setDeleteTarget] = useState<TemplateSummary | null>(null);

  const handleDeleteClick = useCallback((template: TemplateSummary): void => {
    setDeleteTarget(template);
  }, []);

  const handleEditClick = useCallback(
    (id: string): void => {
      (async (): Promise<void> => {
        await navigate(routes.notificationTemplates.detail(channel, id));
      })().catch((_error: unknown) => {
        logger.error('Failed to navigate to notification template', {error: _error, channel, id});
      });
    },
    [channel, logger, navigate, routes],
  );

  const columns: DataGrid.GridColDef<TemplateSummary>[] = useMemo(
    () => [
      {
        field: 'displayName',
        headerName: t('listing.columns.name', 'Name'),
        flex: 1,
        minWidth: 280,
        renderCell: (params: DataGrid.GridRenderCellParams<TemplateSummary>): JSX.Element => (
          <Typography variant="body2">{params.row.displayName}</Typography>
        ),
      },
      {
        field: 'actions',
        headerName: t('listing.columns.actions', 'Actions'),
        width: 120,
        align: 'right',
        headerAlign: 'right',
        sortable: false,
        filterable: false,
        hideable: false,
        renderCell: (params: DataGrid.GridRenderCellParams<TemplateSummary>): JSX.Element => (
          <ListingTable.RowActions>
            <Tooltip title={t('common:actions.edit')}>
              <IconButton
                size="small"
                aria-label={t('common:actions.edit')}
                onClick={(e) => {
                  e.stopPropagation();
                  handleEditClick(params.row.id);
                }}
              >
                <Pencil size={16} />
              </IconButton>
            </Tooltip>
            <Tooltip title={t('common:actions.delete')}>
              <IconButton
                size="small"
                color="error"
                aria-label={t('common:actions.delete')}
                onClick={(e) => {
                  e.stopPropagation();
                  handleDeleteClick(params.row);
                }}
              >
                <Trash2 size={16} />
              </IconButton>
            </Tooltip>
          </ListingTable.RowActions>
        ),
      },
    ],
    [handleDeleteClick, handleEditClick, t],
  );

  if (error) {
    return (
      <QueryErrorNotice
        error={error}
        t={t}
        title={t('listing.error', 'Failed to load templates')}
        onRetry={() => void refetch()}
      />
    );
  }

  return (
    <Box data-testid="notification-templates-list">
      <ListingTable.Provider variant="data-grid-card" loading={isLoading}>
        <ListingTable.Container disablePaper>
          <ListingTable.DataGrid
            rows={rows}
            columns={columns}
            getRowId={(row): string => (row as TemplateSummary).id}
            onRowClick={(params) => {
              handleEditClick((params.row as TemplateSummary).id);
            }}
            paginationMode="server"
            rowCount={data?.totalResults ?? 0}
            paginationModel={paginationModel}
            onPaginationModelChange={setPaginationModel}
            pageSizeOptions={[5, 10, 25, 50]}
            rowHeight={56}
            disableRowSelectionOnClick
            // Filtering is not wired end to end, so the column filter panel stays hidden.
            disableColumnFilter
            localeText={dataGridLocaleText}
            autoHeight
            sx={{
              '& .MuiDataGrid-row': {
                cursor: 'pointer',
              },
            }}
          />
        </ListingTable.Container>
      </ListingTable.Provider>

      <NotificationTemplateDeleteDialog
        open={deleteTarget !== null}
        channel={channel}
        templateId={deleteTarget?.id ?? null}
        templateName={deleteTarget?.displayName}
        onClose={() => setDeleteTarget(null)}
      />
    </Box>
  );
}
