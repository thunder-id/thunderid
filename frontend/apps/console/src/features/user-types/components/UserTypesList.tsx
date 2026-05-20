/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

import {useDataGridLocaleText} from '@thunderid/hooks';
import {useLogger} from '@thunderid/logger/react';
import {
  Chip,
  IconButton,
  Tooltip,
  Typography,
  Snackbar,
  Alert,
  ListingTable,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogContentText,
  DialogActions,
  Button,
  DataGrid,
} from '@wso2/oxygen-ui';
import {Pencil, Trash2} from '@wso2/oxygen-ui-icons-react';
import {useCallback, useMemo, useState} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import useDeleteUserType from '../api/useDeleteUserType';
import useGetUserTypes from '../api/useGetUserTypes';
import type {UserTypeListItem} from '../types/user-types';

type GridColDef<R extends DataGrid.GridValidRowModel = DataGrid.GridValidRowModel> = DataGrid.GridColDef<R>;
type GridRenderCellParams<R extends DataGrid.GridValidRowModel = DataGrid.GridValidRowModel> =
  DataGrid.GridRenderCellParams<R>;

export default function UserTypesList() {
  const navigate = useNavigate();
  const {t} = useTranslation();
  const logger = useLogger('UserTypesList');
  const dataGridLocaleText = useDataGridLocaleText();

  const {data: userTypesData, isLoading, error: userTypesRequestError} = useGetUserTypes();
  const deleteUserTypeMutation = useDeleteUserType();

  const error = userTypesRequestError;

  const [snackbarOpen, setSnackbarOpen] = useState(false);
  const [selectedUserTypeId, setSelectedUserTypeId] = useState<string | null>(null);
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

  const handleDeleteClick = useCallback((userTypeId: string): void => {
    setSelectedUserTypeId(userTypeId);
    setDeleteDialogOpen(true);
  }, []);

  const handleViewClick = useCallback(
    (userTypeId: string): void => {
      (async (): Promise<void> => {
        await navigate(`/user-types/${userTypeId}`);
      })().catch((_error: unknown) => {
        logger.error('Failed to navigate to user type', {error: _error, userTypeId});
      });
    },
    [logger, navigate],
  );

  const handleDeleteCancel = () => {
    setDeleteDialogOpen(false);
    setSelectedUserTypeId(null);
    deleteUserTypeMutation.reset();
  };

  const handleDeleteConfirm = async () => {
    if (!selectedUserTypeId) return;

    try {
      await deleteUserTypeMutation.mutateAsync(selectedUserTypeId);
      setDeleteDialogOpen(false);
      setSelectedUserTypeId(null);
    } catch {
      // Keep dialog open so inline error is visible and user can retry
    }
  };

  const columns: GridColDef<UserTypeListItem>[] = useMemo(
    () => [
      {
        field: 'name',
        headerName: t('userTypes:listing.columns.name', 'Name'),
        flex: 1.5,
        minWidth: 220,
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
          </ListingTable.RowActions>
        ),
      },
    ],
    [t, handleDeleteClick, handleViewClick],
  );

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
      <Dialog open={deleteDialogOpen} onClose={handleDeleteCancel}>
        <DialogTitle>{t('userTypes:deleteUserType')}</DialogTitle>
        <DialogContent>
          <DialogContentText>{t('userTypes:confirmDeleteUserType')}</DialogContentText>
          {deleteUserTypeMutation.error && (
            <Alert severity="error" sx={{mt: 2}}>
              <Typography variant="body2" sx={{fontWeight: 'bold'}}>
                {deleteUserTypeMutation.error.message}
              </Typography>
            </Alert>
          )}
        </DialogContent>
        <DialogActions>
          <Button onClick={handleDeleteCancel} disabled={deleteUserTypeMutation.isPending}>
            {t('common:actions.cancel')}
          </Button>
          <Button
            onClick={() => {
              handleDeleteConfirm().catch(() => {
                // Handle error
              });
            }}
            color="error"
            variant="contained"
            disabled={deleteUserTypeMutation.isPending}
          >
            {deleteUserTypeMutation.isPending ? t('common:status.loading') : t('common:actions.delete')}
          </Button>
        </DialogActions>
      </Dialog>

      <Snackbar
        open={snackbarOpen}
        autoHideDuration={6000}
        onClose={handleCloseSnackbar}
        anchorOrigin={{vertical: 'bottom', horizontal: 'right'}}
      >
        <Alert onClose={handleCloseSnackbar} severity="error" sx={{width: '100%'}}>
          {error?.message ?? t('common:messages.saveError')}
        </Alert>
      </Snackbar>
    </>
  );
}
