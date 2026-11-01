// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {QueryErrorNotice, CardListingTable, type CardListingTableColumn} from '@thunderid/components';
import {useLogger} from '@thunderid/logger/react';
import {IconButton, Typography, Tooltip, DataGrid, ListingTable} from '@wso2/oxygen-ui';
import {Eye, ShieldCheck, Pencil, Trash2} from '@wso2/oxygen-ui-icons-react';
import {useMemo, useCallback, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import RoleDeleteDialog from './RoleDeleteDialog';
import useGetRoles from '../api/useGetRoles';
import useRoleRoutes from '../hooks/useRoleRoutes';
import type {RoleSummary} from '../models/role';

/**
 * DataGrid component for displaying the list of roles.
 */
export default function RolesList(): JSX.Element {
  const navigate = useNavigate();
  const {t} = useTranslation();
  const logger = useLogger('RolesList');
  const routes = useRoleRoutes();
  const [paginationModel, setPaginationModel] = useState<DataGrid.GridPaginationModel>({pageSize: 10, page: 0});

  const rolesParams = useMemo(
    () => ({
      limit: paginationModel.pageSize,
      offset: paginationModel.page * paginationModel.pageSize,
    }),
    [paginationModel],
  );
  const {data, isLoading, error, refetch} = useGetRoles(rolesParams);

  const [selectedRoleId, setSelectedRoleId] = useState<string | null>(null);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState<boolean>(false);

  const handleViewClick = useCallback(
    (roleId: string): void => {
      (async (): Promise<void> => {
        await navigate(routes.roles.detail(roleId));
      })().catch((_error: unknown) => {
        logger.error('Failed to navigate to role details', {error: _error, roleId});
      });
    },
    [navigate, logger, routes.roles],
  );

  const handleDeleteClick = useCallback((roleId: string): void => {
    setSelectedRoleId(roleId);
    setDeleteDialogOpen(true);
  }, []);

  const handleDeleteDialogClose = (): void => {
    setDeleteDialogOpen(false);
    setSelectedRoleId(null);
  };

  const columns: CardListingTableColumn<RoleSummary>[] = useMemo(
    () => [
      {
        field: 'name',
        header: t('roles:listing.columns.name'),
        sortValue: (row) => row.name,
        minWidth: 200,
        render: (row: RoleSummary): JSX.Element => (
          <ListingTable.CellIcon
            icon={<ShieldCheck size={20} />}
            primary={row.name}
            secondary={row.description === '' ? undefined : row.description}
            sx={{width: '100%'}}
          />
        ),
      },
      {
        field: 'ouHandle',
        header: t('roles:listing.columns.organizationUnit'),
        sortValue: (row) => row.ouHandle ?? row.ouId,
        minWidth: 200,
        render: (row: RoleSummary) => (
          <Typography variant="body2" sx={{fontFamily: 'monospace', fontSize: '0.875rem'}}>
            {row.ouHandle ?? row.ouId ?? '-'}
          </Typography>
        ),
      },
      {
        field: 'actions',
        header: t('roles:listing.columns.actions'),
        width: 150,
        align: 'center',
        render: (row: RoleSummary): JSX.Element => (
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
        title={t('roles:listing.error', 'Failed to load roles')}
        onRetry={() => void refetch()}
      />
    );
  }

  return (
    <>
      <CardListingTable
        rows={data?.roles ?? []}
        columns={columns}
        getRowId={(row) => row.id}
        onRowClick={(row) => {
          const roleId = row.id;
          (async (): Promise<void> => {
            await navigate(routes.roles.detail(roleId));
          })().catch((_error: unknown) => {
            logger.error('Failed to navigate to role', {error: _error, roleId});
          });
        }}
        loading={isLoading}
        paginationMode="server"
        rowCount={data?.totalResults ?? 0}
        paginationModel={paginationModel}
        onPaginationModelChange={setPaginationModel}
        pageSizeOptions={[5, 10, 25]}
      />

      <RoleDeleteDialog open={deleteDialogOpen} roleId={selectedRoleId} onClose={handleDeleteDialogClose} />
    </>
  );
}
