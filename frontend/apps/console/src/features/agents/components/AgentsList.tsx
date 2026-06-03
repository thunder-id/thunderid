/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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
import {Box, IconButton, Tooltip, Typography, ListingTable, DataGrid} from '@wso2/oxygen-ui';
import {Eye, Pencil, Trash2} from '@wso2/oxygen-ui-icons-react';
import {useMemo, useCallback, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import AgentDeleteDialog from './AgentDeleteDialog';
import useGetAgents from '../api/useGetAgents';
import type {BasicAgent} from '../models/agent';

export default function AgentsList(): JSX.Element {
  const navigate = useNavigate();
  const {t} = useTranslation();
  const logger = useLogger('AgentsList');
  const dataGridLocaleText = useDataGridLocaleText();
  const {data, isLoading, error} = useGetAgents();

  const [selectedAgentId, setSelectedAgentId] = useState<string | null>(null);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);

  const handleDeleteClick = useCallback((agentId: string): void => {
    setSelectedAgentId(agentId);
    setDeleteDialogOpen(true);
  }, []);

  const handleEditClick = useCallback(
    (agentId: string): void => {
      (async (): Promise<void> => {
        await navigate(`/agents/${agentId}`);
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
          <Box sx={{display: 'flex', alignItems: 'center', gap: 1, width: '100%', overflow: 'hidden'}}>
            <ListingTable.CellIcon
              sx={{flex: 1, minWidth: 0}}
              icon={
                <Box
                  sx={{
                    width: 30,
                    height: 30,
                    borderRadius: '50%',
                    bgcolor: 'primary.light',
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    fontSize: '1rem',
                  }}
                >
                  🤖
                </Box>
              }
              primary={params.row.name}
              secondary={params.row.description}
            />
          </Box>
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
      <Box sx={{textAlign: 'center', py: 8}}>
        <Typography variant="h6" color="error" gutterBottom>
          {t('agents:listing.loadError', 'Failed to load agents')}
        </Typography>
        <Typography variant="body2" color="text.secondary">
          {error.message ?? 'Unknown error'}
        </Typography>
      </Box>
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
