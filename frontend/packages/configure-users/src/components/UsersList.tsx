// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  AccessStatusIndicator,
  QueryErrorNotice,
  ResourceAvatar,
  getInitials,
  CardListingTable,
  type CardListingTableColumn,
} from '@thunderid/components';
import {useLogger} from '@thunderid/logger/react';
import {IconButton, Tooltip, Typography, ListingTable} from '@wso2/oxygen-ui';
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
  const routes = useUserRoutes();

  const {data: userData, isLoading, error, refetch} = useGetUsers();

  const [selectedUserId, setSelectedUserId] = useState<string | null>(null);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);

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

  const columns: CardListingTableColumn<UserWithDetails>[] = useMemo(
    () => [
      {
        field: 'name',
        header: t('users:listing.columns.name', 'Name'),
        sortValue: (row) => row.display ?? row.id,
        minWidth: 200,
        render: (row: UserWithDetails) => {
          const displayVal = row.display ?? row.id;
          const rawPicture = row.attributes?.['picture'];
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
        header: t('users:listing.columns.userId', 'User ID'),
        sortValue: (row) => row.id,
        minWidth: 200,
        render: (row: UserWithDetails) => (
          <Typography variant="body2" sx={{fontFamily: 'monospace', fontSize: '0.875rem'}}>
            {row.id}
          </Typography>
        ),
      },
      {
        field: 'status',
        header: t('users:listing.columns.status', 'Status'),
        minWidth: 110,
        render: (row: UserWithDetails) => <AccessStatusIndicator status={row.status} />,
      },
      {
        field: 'ouHandle',
        header: t('users:listing.columns.organizationUnit', 'Organization unit'),
        sortValue: (row) => row.ouHandle,
        minWidth: 150,
        render: (row: UserWithDetails) => (
          <Typography variant="body2" sx={{fontFamily: 'monospace', fontSize: '0.875rem'}}>
            {row.ouHandle ?? row.ouId ?? '-'}
          </Typography>
        ),
      },
      {
        field: 'actions',
        header: t('users:listing.columns.actions', 'Actions'),
        width: 150,
        align: 'center',
        render: (row: UserWithDetails) => (
          <ListingTable.RowActions>
            {row.isReadOnly ? (
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
                      handleEditClick(row.id);
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
    [handleDeleteClick, handleEditClick, t],
  );

  if (error) {
    return (
      <QueryErrorNotice
        error={error}
        t={t}
        variant="block"
        title={t('users:listing.error', 'Failed to load users')}
        onRetry={() => void refetch()}
      />
    );
  }

  return (
    <>
      <CardListingTable
        rows={userData?.users ?? []}
        columns={columns}
        getRowId={(row) => row.id}
        onRowClick={(row) => {
          handleEditClick(row.id);
        }}
        loading={isLoading}
        pageSizeOptions={[5, 10, 25, 50]}
      />

      {/* Delete Confirmation Dialog */}
      <UserDeleteDialog open={deleteDialogOpen} userId={selectedUserId} onClose={handleDeleteCancel} />
    </>
  );
}
