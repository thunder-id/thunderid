// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {QueryErrorNotice} from '@thunderid/components';
import {useDataGridLocaleText} from '@thunderid/hooks';
import {useLogger} from '@thunderid/logger/react';
import {Chip, IconButton, Tooltip, Typography, ListingTable, DataGrid} from '@wso2/oxygen-ui';
import {Eye, Pencil, Trash2} from '@wso2/oxygen-ui-icons-react';
import {useCallback, useMemo, useState} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import UserTypeDeleteDialog from './edit-user-type/UserTypeDeleteDialog';
import useGetUserTypes from '../api/useGetUserTypes';
import useUserTypeRoutes from '../hooks/useUserTypeRoutes';
import type {UserTypeListItem} from '../types/user-types';

type GridColDef<R extends DataGrid.GridValidRowModel = DataGrid.GridValidRowModel> = DataGrid.GridColDef<R>;
type GridRenderCellParams<R extends DataGrid.GridValidRowModel = DataGrid.GridValidRowModel> =
  DataGrid.GridRenderCellParams<R>;

export default function UserTypesList() {
  const navigate = useNavigate();
  const {t} = useTranslation();
  const logger = useLogger('UserTypesList');
  const routes = useUserTypeRoutes();
  const dataGridLocaleText = useDataGridLocaleText();

  const {data: userTypesData, isLoading, error: userTypesRequestError, refetch} = useGetUserTypes();

  const [selectedUserTypeId, setSelectedUserTypeId] = useState<string | null>(null);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);

  const handleDeleteClick = useCallback((userTypeId: string): void => {
    setSelectedUserTypeId(userTypeId);
    setDeleteDialogOpen(true);
  }, []);

  const handleViewClick = useCallback(
    (userTypeId: string): void => {
      (async (): Promise<void> => {
        await navigate(routes.detail(userTypeId));
      })().catch((_error: unknown) => {
        logger.error('Failed to navigate to user type', {error: _error, userTypeId});
      });
    },
    [logger, navigate, routes],
  );

  const handleDeleteDialogClose = (): void => {
    setDeleteDialogOpen(false);
    setSelectedUserTypeId(null);
  };

  const columns: GridColDef<UserTypeListItem>[] = useMemo(
    () => [
      {
        field: 'name',
        headerName: t('userTypes:listing.columns.name', 'Name'),
        flex: 1.5,
        minWidth: 220,
        renderCell: (params: DataGrid.GridRenderCellParams<UserTypeListItem>) => (
          <Typography variant="body2">{params.row.name}</Typography>
        ),
      },
      {
        field: 'id',
        headerName: t('userTypes:listing.columns.id', 'User Type ID'),
        width: 350,
        renderCell: (params: DataGrid.GridRenderCellParams<UserTypeListItem>) => (
          <Typography variant="body2" sx={{fontFamily: 'monospace', fontSize: '0.875rem'}}>
            {params.row.id}
          </Typography>
        ),
      },
      {
        field: 'ouHandle',
        headerName: t('userTypes:listing.columns.organizationUnit', 'Organization Unit'),
        flex: 1,
        minWidth: 220,
        renderCell: (params: DataGrid.GridRenderCellParams<UserTypeListItem>) => (
          <Typography variant="body2" sx={{fontFamily: 'monospace', fontSize: '0.875rem'}}>
            {params.row.ouHandle ?? params.row.ouId ?? t('common:messages.noData')}
          </Typography>
        ),
      },
      {
        field: 'allowSelfRegistration',
        headerName: t('userTypes:listing.columns.allowSelfRegistration', 'Self Registration'),
        width: 200,
        renderCell: (params: GridRenderCellParams<UserTypeListItem>) => (
          <Chip
            label={params.row.allowSelfRegistration ? t('common:status.enabled') : t('common:status.disabled')}
            color={params.row.allowSelfRegistration ? 'success' : 'default'}
            size="small"
          />
        ),
      },
      {
        field: 'actions',
        headerName: t('userTypes:listing.columns.actions', 'Actions'),
        width: 150,
        align: 'center',
        headerAlign: 'center',
        sortable: false,
        filterable: false,
        hideable: false,
        renderCell: (params: GridRenderCellParams<UserTypeListItem>) => (
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
                      handleViewClick(params.row.id);
                    }}
                  >
                    <Pencil size={16} />
                  </IconButton>
                </Tooltip>
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
              </>
            )}
          </ListingTable.RowActions>
        ),
      },
    ],
    [t, handleDeleteClick, handleViewClick],
  );

  if (userTypesRequestError) {
    return (
      <QueryErrorNotice
        error={userTypesRequestError}
        t={(key, options) => t(key.includes(':') ? key : `userTypes:${key}`, options)}
        variant="block"
        title={t('userTypes:listing.error', 'Failed to load user types')}
        onRetry={() => void refetch()}
      />
    );
  }

  return (
    <>
      <ListingTable.Provider variant="data-grid-card" loading={isLoading}>
        <ListingTable.Container disablePaper>
          <ListingTable.DataGrid
            rows={userTypesData?.types ?? []}
            columns={columns}
            getRowId={(row) => (row as UserTypeListItem).id}
            onRowClick={(params) => {
              handleViewClick((params.row as UserTypeListItem).id);
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
            autoHeight
            sx={{
              '& .MuiDataGrid-row': {
                cursor: 'pointer',
              },
            }}
          />
        </ListingTable.Container>
      </ListingTable.Provider>

      {/* Delete Confirmation Dialog */}
      <UserTypeDeleteDialog open={deleteDialogOpen} userTypeId={selectedUserTypeId} onClose={handleDeleteDialogClose} />
    </>
  );
}
