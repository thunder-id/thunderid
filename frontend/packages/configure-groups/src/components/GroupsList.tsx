// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {QueryErrorNotice} from '@thunderid/components';
import {useDataGridLocaleText} from '@thunderid/hooks';
import {useLogger} from '@thunderid/logger/react';
import {IconButton, Typography, Tooltip, DataGrid, ListingTable} from '@wso2/oxygen-ui';
import {Eye, Pencil, Trash2} from '@wso2/oxygen-ui-icons-react';
import {useMemo, useCallback, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import GroupDeleteDialog from './GroupDeleteDialog';
import {useIsManagedResource} from '@thunderid/contexts';
import useGetGroups from '../api/useGetGroups';
import useGroupRoutes from '../hooks/useGroupRoutes';
import type {GroupBasic} from '../models/group';

/**
 * DataGrid component for displaying the list of groups.
 */
export default function GroupsList(): JSX.Element {
  const navigate = useNavigate();
  // A group applied from the control plane is read only here, the same as a declarative one.
  const isManagedGroup = useIsManagedResource('group');
  const {t} = useTranslation();
  const logger = useLogger('GroupsList');
  const dataGridLocaleText = useDataGridLocaleText();
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

  const columns: DataGrid.GridColDef<GroupBasic>[] = useMemo(
    () => [
      {
        field: 'name',
        headerName: t('groups:listing.columns.name', 'Name'),
        flex: 1,
        minWidth: 200,
        renderCell: (params: DataGrid.GridRenderCellParams<GroupBasic>): JSX.Element => (
          <Typography variant="body2">{params.row.name}</Typography>
        ),
      },
      {
        field: 'description',
        headerName: t('groups:listing.columns.description', 'Description'),
        flex: 1.5,
        minWidth: 250,
        valueGetter: (_value, row): string => row.description ?? '-',
      },
      {
        field: 'ouHandle',
        headerName: t('groups:listing.columns.organizationUnit', 'Organization Unit'),
        flex: 1,
        minWidth: 200,
        renderCell: (params: DataGrid.GridRenderCellParams<GroupBasic>) => (
          <Typography variant="body2" sx={{fontFamily: 'monospace', fontSize: '0.875rem'}}>
            {params.row.ouHandle ?? params.row.ouId ?? '-'}
          </Typography>
        ),
      },
      {
        field: 'actions',
        headerName: t('groups:listing.columns.actions', 'Actions'),
        width: 150,
        align: 'center',
        headerAlign: 'center',
        sortable: false,
        filterable: false,
        hideable: false,
        renderCell: (params: DataGrid.GridRenderCellParams<GroupBasic>): JSX.Element => (
          <ListingTable.RowActions>
            {params.row.isReadOnly || isManagedGroup(params.row.id) ? (
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
    [handleDeleteClick, handleViewClick, t, isManagedGroup],
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
      <ListingTable.Provider variant="data-grid-card" loading={isLoading}>
        <ListingTable.Container disablePaper>
          <ListingTable.DataGrid
            rows={data?.groups ?? []}
            columns={columns}
            getRowId={(row) => (row as GroupBasic).id}
            onRowClick={(params) => {
              const groupId = (params.row as GroupBasic).id;
              (async (): Promise<void> => {
                await navigate(routes.groups.detail(groupId));
              })().catch((_error: unknown) => {
                logger.error('Failed to navigate to group', {error: _error, groupId});
              });
            }}
            paginationMode="server"
            rowCount={data?.totalResults ?? 0}
            paginationModel={paginationModel}
            onPaginationModelChange={setPaginationModel}
            pageSizeOptions={[5, 10, 25]}
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

      <GroupDeleteDialog open={deleteDialogOpen} groupId={selectedGroupId} onClose={handleDeleteDialogClose} />
    </>
  );
}
