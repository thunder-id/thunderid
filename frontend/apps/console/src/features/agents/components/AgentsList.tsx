// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {QueryErrorNotice, ResourceAvatar} from '@thunderid/components';
import {useDataGridLocaleText} from '@thunderid/hooks';
import {useLogger} from '@thunderid/logger/react';
import {Box, IconButton, Tooltip, Typography, ListingTable, DataGrid} from '@wso2/oxygen-ui';
import {Eye, Pencil, Trash2} from '@wso2/oxygen-ui-icons-react';
import {useMemo, useCallback, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import AgentDeleteDialog from './AgentDeleteDialog';
import RouteConfig from '../../../configs/RouteConfig';
import useGetAgents from '../api/useGetAgents';
import AgentConstants from '../constants/agent-constants';
import type {BasicAgent} from '../models/agent';

export default function AgentsList(): JSX.Element {
  const navigate = useNavigate();
  const {t} = useTranslation();
  const logger = useLogger('AgentsList');
  const dataGridLocaleText = useDataGridLocaleText();
  const {data, isLoading, error, refetch} = useGetAgents();

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
        await navigate(RouteConfig.agents.detail(agentId));
      })().catch((_error: unknown) => {
        logger.error('Failed to navigate to agent', {error: _error, agentId});
      });
    },
    [logger, navigate],
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
            primary={params.row.name}
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
        field: 'ouHandle',
        headerName: t('agents:listing.columns.organizationUnit', 'Organization Unit'),
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

  if (error) {
    return (
      <QueryErrorNotice
        error={error}
        t={tForErrors}
        variant="block"
        title={t('agents:listing.loadError', 'Failed to load agents')}
        onRetry={() => void refetch()}
      />
    );
  }

  return (
    <Box data-testid="agents-list">
      <ListingTable.Provider variant="data-grid-card" loading={isLoading}>
        <ListingTable.Container disablePaper>
          <ListingTable.DataGrid
            rows={data?.agents ?? []}
            columns={columns}
            getRowId={(row): string => (row as BasicAgent).id}
            onRowClick={(params) => {
              handleEditClick((params.row as BasicAgent).id);
            }}
            initialState={{
              pagination: {paginationModel: {pageSize: 10}},
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

      <AgentDeleteDialog open={deleteDialogOpen} agentId={selectedAgentId} onClose={handleDeleteDialogClose} />
    </Box>
  );
}
