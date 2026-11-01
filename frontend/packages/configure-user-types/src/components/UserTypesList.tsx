// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {QueryErrorNotice, CardListingTable, type CardListingTableColumn} from '@thunderid/components';
import {useLogger} from '@thunderid/logger/react';
import {getErrorMessage} from '@thunderid/utils';
import {
  Chip,
  IconButton,
  Tooltip,
  Typography,
  Alert,
  ListingTable,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogContentText,
  DialogActions,
  Button,
} from '@wso2/oxygen-ui';
import {Eye, Pencil, Trash2, UserRoundCog} from '@wso2/oxygen-ui-icons-react';
import {useCallback, useMemo, useState} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import useDeleteUserType from '../api/useDeleteUserType';
import useGetUserTypes from '../api/useGetUserTypes';
import useUserTypeRoutes from '../hooks/useUserTypeRoutes';
import type {UserTypeListItem} from '../types/user-types';

export default function UserTypesList() {
  const navigate = useNavigate();
  const {t} = useTranslation();
  const logger = useLogger('UserTypesList');
  const routes = useUserTypeRoutes();

  const {data: userTypesData, isLoading, error: userTypesRequestError, refetch} = useGetUserTypes();
  const deleteUserTypeMutation = useDeleteUserType();

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

  const columns: CardListingTableColumn<UserTypeListItem>[] = useMemo(
    () => [
      {
        field: 'displayName',
        header: t('userTypes:listing.columns.name', 'Name'),
        sortValue: (row) => row.displayName,
        minWidth: 220,
        render: (row: UserTypeListItem) => (
          <ListingTable.CellIcon
            icon={<UserRoundCog size={20} />}
            primary={row.displayName}
            secondary={row.handle}
            sx={{width: '100%'}}
          />
        ),
      },
      {
        field: 'id',
        header: t('userTypes:listing.columns.id', 'User type ID'),
        sortValue: (row) => row.id,
        width: 350,
        render: (row: UserTypeListItem) => (
          <Typography variant="body2" sx={{fontFamily: 'monospace', fontSize: '0.875rem'}}>
            {row.id}
          </Typography>
        ),
      },
      {
        field: 'ouHandle',
        header: t('userTypes:listing.columns.organizationUnit', 'Organization unit'),
        sortValue: (row) => row.ouHandle,
        minWidth: 220,
        render: (row: UserTypeListItem) => (
          <Typography variant="body2" sx={{fontFamily: 'monospace', fontSize: '0.875rem'}}>
            {row.ouHandle ?? row.ouId ?? t('common:messages.noData')}
          </Typography>
        ),
      },
      {
        field: 'allowSelfRegistration',
        header: t('userTypes:listing.columns.allowSelfRegistration', 'Self registration'),
        sortValue: (row) => (row.allowSelfRegistration ? 1 : 0),
        width: 200,
        render: (row: UserTypeListItem) => (
          <Chip
            label={row.allowSelfRegistration ? t('common:status.enabled') : t('common:status.disabled')}
            color={row.allowSelfRegistration ? 'success' : 'default'}
            size="small"
          />
        ),
      },
      {
        field: 'actions',
        header: t('userTypes:listing.columns.actions', 'Actions'),
        width: 150,
        align: 'center',
        render: (row: UserTypeListItem) => (
          <ListingTable.RowActions>
            {row.isReadOnly ? (
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
                <Tooltip title={t('common:actions.edit', 'Edit')}>
                  <IconButton
                    size="small"
                    aria-label={t('common:actions.edit', 'Edit')}
                    onClick={(e) => {
                      e.stopPropagation();
                      handleViewClick(row.id);
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
                      handleDeleteClick(row.id);
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
      <CardListingTable
        rows={userTypesData?.types ?? []}
        columns={columns}
        getRowId={(row) => row.id}
        onRowClick={(row) => {
          handleViewClick(row.id);
        }}
        loading={isLoading}
        pageSizeOptions={[5, 10, 25, 50]}
      />

      {/* Delete Confirmation Dialog */}
      <Dialog open={deleteDialogOpen} onClose={handleDeleteCancel}>
        <DialogTitle>{t('userTypes:deleteUserType')}</DialogTitle>
        <DialogContent>
          <DialogContentText>{t('userTypes:confirmDeleteUserType')}</DialogContentText>
          {deleteUserTypeMutation.error && (
            <Alert severity="error" sx={{mt: 2}}>
              <Typography variant="body2" sx={{fontWeight: 'bold'}}>
                {getErrorMessage(
                  deleteUserTypeMutation.error,
                  (key, options) => t(key.includes(':') ? key : `userTypes:${key}`, options),
                  'delete.error',
                  'Failed to delete user type. Please try again.',
                )}
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
    </>
  );
}
