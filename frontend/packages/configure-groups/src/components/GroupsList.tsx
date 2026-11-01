// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {QueryErrorNotice, CardListingTable, type CardListingTableColumn} from '@thunderid/components';
import {useLogger} from '@thunderid/logger/react';
import {IconButton, Typography, Tooltip, DataGrid, ListingTable} from '@wso2/oxygen-ui';
import {Eye, Group, Pencil, Trash2} from '@wso2/oxygen-ui-icons-react';
import {useMemo, useCallback, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import GroupDeleteDialog from './GroupDeleteDialog';
import useGetGroups from '../api/useGetGroups';
import useGroupRoutes from '../hooks/useGroupRoutes';
import type {GroupBasic} from '../models/group';

/**
 * DataGrid component for displaying the list of groups.
 */
export default function GroupsList(): JSX.Element {
  const navigate = useNavigate();
  const {t} = useTranslation();
  const logger = useLogger('GroupsList');
  const routes = useGroupRoutes();
  const [paginationModel, setPaginationModel] = useState<DataGrid.GridPaginationModel>({pageSize: 10, page: 0});

  const groupsParams = useMemo(
    () => ({
      limit: paginationModel.pageSize,
      offset: paginationModel.page * paginationModel.pageSize,
    }),
    [paginationModel],
  );
  const {data, isLoading, error, refetch} = useGetGroups(groupsParams);

  const [selectedGroupId, setSelectedGroupId] = useState<string | null>(null);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState<boolean>(false);

  const handleViewClick = useCallback(
    (groupId: string): void => {
      (async (): Promise<void> => {
        await navigate(routes.groups.detail(groupId));
      })().catch((_error: unknown) => {
        logger.error('Failed to navigate to group details', {error: _error, groupId});
      });
    },
    [navigate, logger, routes.groups],
  );

  const handleDeleteClick = useCallback((groupId: string): void => {
    setSelectedGroupId(groupId);
    setDeleteDialogOpen(true);
  }, []);

  const handleDeleteDialogClose = (): void => {
    setDeleteDialogOpen(false);
    setSelectedGroupId(null);
  };

  const columns: CardListingTableColumn<GroupBasic>[] = useMemo(
    () => [
      {
        field: 'name',
        header: t('groups:listing.columns.name', 'Name'),
        sortValue: (row) => row.name,
        minWidth: 200,
        render: (row: GroupBasic): JSX.Element => (
          <ListingTable.CellIcon
            icon={<Group size={20} />}
            primary={row.name}
            secondary={row.description === '' ? undefined : row.description}
            sx={{width: '100%'}}
          />
        ),
      },
      {
        field: 'ouHandle',
        header: t('groups:listing.columns.organizationUnit', 'Organization unit'),
        sortValue: (row) => row.ouHandle,
        minWidth: 200,
        render: (row: GroupBasic) => (
          <Typography variant="body2" sx={{fontFamily: 'monospace', fontSize: '0.875rem'}}>
            {row.ouHandle ?? row.ouId ?? '-'}
          </Typography>
        ),
      },
      {
        field: 'actions',
        header: t('groups:listing.columns.actions', 'Actions'),
        width: 150,
        align: 'center',
        render: (row: GroupBasic): JSX.Element => (
          <ListingTable.RowActions>
            {row.isReadOnly ? (
              <Tooltip title={t('common:status.readOnly', 'Read only')}>
                <IconButton
                  size="small"
                  aria-label={t('common:status.readOnly', 'Read only')}
                  disableRipple
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
                    aria-label={t('common:actions.delete', 'Delete')}
                    color="error"
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
    [handleDeleteClick, handleViewClick, t],
  );

  if (error) {
    return (
      <QueryErrorNotice
        error={error}
        t={t}
        variant="block"
        title={t('groups:listing.error', 'Failed to load groups')}
        onRetry={() => void refetch()}
      />
    );
  }

  return (
    <>
      <CardListingTable
        rows={data?.groups ?? []}
        columns={columns}
        getRowId={(row) => row.id}
        onRowClick={(row) => {
          const groupId = row.id;
          (async (): Promise<void> => {
            await navigate(routes.groups.detail(groupId));
          })().catch((_error: unknown) => {
            logger.error('Failed to navigate to group', {error: _error, groupId});
          });
        }}
        loading={isLoading}
        paginationMode="server"
        rowCount={data?.totalResults ?? 0}
        paginationModel={paginationModel}
        onPaginationModelChange={setPaginationModel}
        pageSizeOptions={[5, 10, 25]}
      />

      <GroupDeleteDialog open={deleteDialogOpen} groupId={selectedGroupId} onClose={handleDeleteDialogClose} />
    </>
  );
}
