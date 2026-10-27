// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {AccessStatusIndicator, QueryErrorNotice, ResourceAvatar} from '@thunderid/components';
import {useDataGridLocaleText} from '@thunderid/hooks';
import {useLogger} from '@thunderid/logger/react';
import {
  AgentList,
  AgentQueryKeys,
  useThunderID,
  type AdminListRenderProps,
  type BasicAgent as SdkBasicAgent,
} from '@thunderid/react';
import {Box, IconButton, Tooltip, Typography, ListingTable, DataGrid} from '@wso2/oxygen-ui';
import {Eye, Pencil, Trash2} from '@wso2/oxygen-ui-icons-react';
import {useMemo, useCallback, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import AgentDeleteDialog from './AgentDeleteDialog';
import AgentConstants from '../constants/agent-constants';
import useAgentRoutes from '../hooks/useAgentRoutes';
import type {BasicAgent} from '../models/agent';

export default function AgentsList(): JSX.Element {
  const routes = useAgentRoutes();
  const navigate = useNavigate();
  const {t} = useTranslation();
  const logger = useLogger('AgentsList');
  const dataGridLocaleText = useDataGridLocaleText();
  const {invalidator} = useThunderID();
  const [pageSize, setPageSize] = useState<number>(10);

  // Resolves an error through the `agents` catalog. `t` defaults to the `common` namespace, so
  // this forwards explicit `ns:` prefixes unchanged and prefixes bare keys with `agents:`, per
  // getErrorMessage's namespace-resolution contract.
  const tForErrors = useCallback(
    (key: string, options?: Record<string, unknown>): string => t(key.includes(':') ? key : `agents:${key}`, options),
    [t],
  );

  const [selectedAgentId, setSelectedAgentId] = useState<string | null>(null);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);

  const handleDeleteClick = useCallback((agentId: string): void => {
    setSelectedAgentId(agentId);
    setDeleteDialogOpen(true);
  }, []);

  const handleEditClick = useCallback(
    (agentId: string): void => {
      (async (): Promise<void> => {
        await navigate(routes.agents.detail(agentId));
      })().catch((_error: unknown) => {
        logger.error('Failed to navigate to agent', {error: _error, agentId});
      });
    },
    [logger, navigate, routes],
  );

  const handleDeleteDialogClose = (): void => {
    setDeleteDialogOpen(false);
    setSelectedAgentId(null);
  };

  const columns: DataGrid.GridColDef<BasicAgent>[] = useMemo(
    () => [
      {
        field: 'name',
        headerName: t('agents:listing.columns.name', 'Name'),
        flex: 1,
        minWidth: 200,
        renderCell: (params: DataGrid.GridRenderCellParams<BasicAgent>): JSX.Element => (
          <ListingTable.CellIcon
            sx={{width: '100%'}}
            icon={<ResourceAvatar size={30} value={params.row.logoUrl} fallback={AgentConstants.DEFAULT_AVATAR} />}
            primary={params.row.display ?? params.row.id}
            secondary={params.row.description}
          />
        ),
      },
      {
        field: 'id',
        headerName: t('agents:listing.columns.agentId', 'Agent ID'),
        flex: 1,
        minWidth: 200,
        renderCell: (params: DataGrid.GridRenderCellParams<BasicAgent>): JSX.Element => (
          <Typography variant="body2" sx={{fontFamily: 'monospace', fontSize: '0.875rem'}}>
            {params.row.id}
          </Typography>
        ),
      },
      {
        field: 'status',
        headerName: t('agents:listing.columns.status', 'Status'),
        flex: 0.4,
        minWidth: 110,
        sortable: false,
        renderCell: (params: DataGrid.GridRenderCellParams<BasicAgent>) => (
          <AccessStatusIndicator status={params.row.status} category="agent" tabIndex={params.tabIndex} />
        ),
      },
      {
        field: 'ouHandle',
        headerName: t('agents:listing.columns.organizationUnit', 'Organization unit'),
        flex: 0.5,
        minWidth: 150,
        renderCell: (params: DataGrid.GridRenderCellParams<BasicAgent>): JSX.Element => (
          <Typography variant="body2" sx={{fontFamily: 'monospace', fontSize: '0.875rem'}}>
            {params.row.ouHandle ?? params.row.ouId ?? '-'}
          </Typography>
        ),
      },
      {
        field: 'actions',
        headerName: t('agents:listing.columns.actions', 'Actions'),
        width: 150,
        align: 'center',
        headerAlign: 'center',
        sortable: false,
        filterable: false,
        hideable: false,
        renderCell: (params: DataGrid.GridRenderCellParams<BasicAgent>): JSX.Element => (
          <ListingTable.RowActions>
            {params.row.isReadOnly ? (
              <Tooltip title={t('common:status.readOnly', 'Read only')}>
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

  const renderList = ({
    error,
    isLoading,
    limit,
    offset,
    onPageChange,
    rows,
    totalResults,
  }: AdminListRenderProps<SdkBasicAgent>): JSX.Element => {
    if (error) {
      return (
        <QueryErrorNotice
          error={new Error(error)}
          t={tForErrors}
          variant="block"
          title={t('agents:listing.loadError', 'Failed to load agents')}
          onRetry={() => invalidator?.invalidate([AgentQueryKeys.AGENTS])}
        />
      );
    }

    return (
      <ListingTable.Provider variant="data-grid-card" loading={isLoading}>
        <ListingTable.Container disablePaper>
          <ListingTable.DataGrid
            rows={rows as BasicAgent[]}
            columns={columns}
            getRowId={(row): string => (row as BasicAgent).id}
            onRowClick={(params) => {
              handleEditClick((params.row as BasicAgent).id);
            }}
            paginationMode="server"
            rowCount={totalResults}
            paginationModel={{page: Math.floor(offset / limit), pageSize: limit}}
            onPaginationModelChange={(model: DataGrid.GridPaginationModel) => {
              if (model.pageSize !== limit) {
                setPageSize(model.pageSize);
                onPageChange?.(0);
                return;
              }
              onPageChange?.(model.page * limit);
            }}
            pageSizeOptions={[5, 10, 25, 50]}
            disableRowSelectionOnClick
            // Filtering is not wired end to end, so the column filter panel stays hidden.
            disableColumnFilter
            localeText={dataGridLocaleText}
            sx={{
              height: 'auto',
              '& .MuiDataGrid-row': {cursor: 'pointer'},
            }}
          />
        </ListingTable.Container>
      </ListingTable.Provider>
    );
  };

  return (
    <Box data-testid="agents-list">
      <AgentList limit={pageSize} searchable={false}>
        {renderList}
      </AgentList>

      <AgentDeleteDialog open={deleteDialogOpen} agentId={selectedAgentId} onClose={handleDeleteDialogClose} />
    </Box>
  );
}
