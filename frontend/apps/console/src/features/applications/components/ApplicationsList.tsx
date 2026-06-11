/**
 * Copyright (c) 2025-2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {ResourceAvatar} from '@thunderid/components';
import {useConfig} from '@thunderid/contexts';
import {useDataGridLocaleText} from '@thunderid/hooks';
import {useLogger} from '@thunderid/logger/react';
import {Box, Chip, IconButton, Tooltip, Typography, ListingTable, DataGrid} from '@wso2/oxygen-ui';
import {Eye, Pencil, Trash2} from '@wso2/oxygen-ui-icons-react';
import {useMemo, useCallback, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import ApplicationDeleteDialog from './ApplicationDeleteDialog';
import useGetApplications from '../api/useGetApplications';
import type {BasicApplication} from '../models/application';
import getTemplateMetadata from '../utils/getTemplateMetadata';

export default function ApplicationsList(): JSX.Element {
  const navigate = useNavigate();
  const {config} = useConfig();
  const {t} = useTranslation();
  const logger = useLogger('ApplicationsList');
  const dataGridLocaleText = useDataGridLocaleText();
  const {data, isLoading, error} = useGetApplications();
  const systemConsoleClientId = (config?.client?.client_id ?? 'CONSOLE').toUpperCase();

  const [selectedAppId, setSelectedAppId] = useState<string | null>(null);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState<boolean>(false);

  const handleDeleteClick = useCallback((appId: string): void => {
    setSelectedAppId(appId);
    setDeleteDialogOpen(true);
  }, []);

  const handleEditClick = useCallback(
    (appId: string): void => {
      (async (): Promise<void> => {
        await navigate(`/applications/${appId}`);
      })().catch((_error: unknown) => {
        logger.error('Failed to navigate to application', {error: _error, applicationId: appId});
      });
    },
    [logger, navigate],
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
            icon={<ResourceAvatar value={params.row.logoUrl} size={30} fallback="emoji:🖥️" />}
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
            <Chip
              icon={
                <Box sx={{display: 'flex', alignItems: 'center', '& > *': {width: 16, height: 16}}}>
                  {templateMetadata.icon}
                </Box>
              }
              label={templateMetadata.displayName}
              size="small"
              variant="outlined"
              sx={{
                px: 0.5,
                fontSize: '0.75rem',
              }}
            />
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
              <Tooltip title={t('common:status.readOnly', 'Read Only')}>
                <IconButton size="small" disableRipple sx={{cursor: 'default'}}>
                  <Eye size={16} />
                </IconButton>
              </Tooltip>
            ) : (
              <>
                <Tooltip title={t('common:actions.edit')}>
                  <IconButton
                    size="small"
                    onClick={(e) => {
                      e.stopPropagation();
                      handleEditClick(params.row.id);
                    }}
                  >
                    <Pencil size={16} />
                  </IconButton>
                </Tooltip>
                {params.row.clientId?.toUpperCase() !== systemConsoleClientId && (
                  <Tooltip title={t('common:actions.delete')}>
                    <IconButton
                      size="small"
                      color="error"
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
    [handleDeleteClick, handleEditClick, systemConsoleClientId, t],
  );

  if (error) {
    return (
      <Box sx={{textAlign: 'center', py: 8}}>
        <Typography variant="h6" color="error" gutterBottom>
          Failed to load applications
        </Typography>
        <Typography variant="body2" color="text.secondary">
          {error.message ?? 'Unknown error'}
        </Typography>
      </Box>
    );
  }

  return (
    <Box data-testid="applications-list">
      <ListingTable.Provider variant="data-grid-card" loading={isLoading}>
        <ListingTable.Container disablePaper>
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

      <ApplicationDeleteDialog
        open={deleteDialogOpen}
        applicationId={selectedAppId}
        onClose={handleDeleteDialogClose}
      />
    </Box>
  );
}
