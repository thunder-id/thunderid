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

import {ResourceAvatar, getInitials} from '@thunderid/components';
import {useDataGridLocaleText} from '@thunderid/hooks';
import {useLogger} from '@thunderid/logger/react';
import {IconButton, Tooltip, Typography, Snackbar, Alert, ListingTable, DataGrid} from '@wso2/oxygen-ui';
import {Eye, Pencil, Trash2} from '@wso2/oxygen-ui-icons-react';
import {useMemo, useState, useCallback} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import UserDeleteDialog from './UserDeleteDialog';
import useGetUsers from '../api/useGetUsers';
import UserConstants from '../constants/user-constants';
import useUserRoutes from '../hooks/useUserRoutes';
import type {UserWithDetails} from '../models/users';

export default function UsersList() {
  const navigate = useNavigate();
  const {t} = useTranslation();
  const logger = useLogger('UsersList');
  const dataGridLocaleText = useDataGridLocaleText();
  const routes = useUserRoutes();

  const {data: userData, isLoading, error: usersRequestError} = useGetUsers();

  const error = usersRequestError;

  const [snackbarOpen, setSnackbarOpen] = useState(false);
  const [selectedUserId, setSelectedUserId] = useState<string | null>(null);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);

  // Show snackbar when error occurs
  const [prevError, setPrevError] = useState<typeof error>(null);
  if (prevError !== error) {
    setPrevError(error);
    if (error) {
      setSnackbarOpen(true);
    }
  }

  const handleCloseSnackbar = () => {
    setSnackbarOpen(false);
  };

  const handleDeleteClick = useCallback((userId: string): void => {
    setSelectedUserId(userId);
    setDeleteDialogOpen(true);
  }, []);

  const handleEditClick = useCallback(
    (userId: string): void => {
      (async (): Promise<void> => {
        await navigate(routes.detail(userId));
      })().catch((_error: unknown) => {
        logger.error('Failed to navigate to user details', {error: _error, userId});
      });
    },
    [logger, navigate, routes],
  );

  const handleDeleteCancel = () => {
    setDeleteDialogOpen(false);
    setSelectedUserId(null);
  };

  const columns: DataGrid.GridColDef<UserWithDetails>[] = useMemo(
    () => [
      {
        field: 'name',
        headerName: t('users:listing.columns.name', 'Name'),
        flex: 1,
        minWidth: 200,
        renderCell: (params: DataGrid.GridRenderCellParams<UserWithDetails>) => {
          const displayVal = params.row.display ?? params.row.id;
          const rawPicture = params.row.attributes?.['picture'];
          const picture = typeof rawPicture === 'string' ? rawPicture : undefined;

          return (
            <ListingTable.CellIcon
              sx={{width: '100%'}}
              icon={
                <ResourceAvatar
                  value={picture}
                  size={30}
                  fallback={`${UserConstants.DEFAULT_AVATAR_PREFIX}${getInitials(displayVal)}`}
                />
              }
              primary={displayVal}
            />
          );
        },
      },
      {
        field: 'id',
        headerName: t('users:listing.columns.userId', 'User ID'),
        flex: 1,
        minWidth: 200,
        renderCell: (params: DataGrid.GridRenderCellParams<UserWithDetails>) => (
          <Typography variant="body2" sx={{fontFamily: 'monospace', fontSize: '0.875rem'}}>
            {params.row.id}
          </Typography>
        ),
      },
      {
        field: 'ouHandle',
        headerName: t('users:listing.columns.organizationUnit', 'Organization Unit'),
        flex: 0.5,
        minWidth: 150,
        renderCell: (params: DataGrid.GridRenderCellParams<UserWithDetails>) => (
          <Typography variant="body2" sx={{fontFamily: 'monospace', fontSize: '0.875rem'}}>
            {params.row.ouHandle ?? params.row.ouId ?? '-'}
          </Typography>
        ),
      },
      {
        field: 'actions',
        headerName: t('users:listing.columns.actions', 'Actions'),
        width: 150,
        align: 'center',
        headerAlign: 'center',
        sortable: false,
        filterable: false,
        hideable: false,
        renderCell: (params: DataGrid.GridRenderCellParams<UserWithDetails>) => (
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
    [handleDeleteClick, handleEditClick, t],
  );

  return (
    <>
      <ListingTable.Provider variant="data-grid-card" loading={isLoading}>
        <ListingTable.Container disablePaper>
          <ListingTable.DataGrid
            rows={userData?.users ?? []}
            columns={columns}
            getRowId={(row) => (row as UserWithDetails).id}
            onRowClick={(params) => {
              handleEditClick((params.row as UserWithDetails).id);
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

      {/* Delete Confirmation Dialog */}
      <UserDeleteDialog open={deleteDialogOpen} userId={selectedUserId} onClose={handleDeleteCancel} />

      <Snackbar
        open={snackbarOpen}
        autoHideDuration={6000}
        onClose={handleCloseSnackbar}
        anchorOrigin={{vertical: 'top', horizontal: 'right'}}
      >
        <Alert onClose={handleCloseSnackbar} severity="error" sx={{width: '100%'}}>
          {error?.message ?? t('common:messages.saveError')}
        </Alert>
      </Snackbar>
    </>
  );
}
