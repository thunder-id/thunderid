// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {QueryErrorNotice} from '@thunderid/components';
import {useDataGridLocaleText} from '@thunderid/hooks';
import {useLogger} from '@thunderid/logger/react';
import {IconButton, Stack, Tooltip, Typography, ListingTable, DataGrid} from '@wso2/oxygen-ui';
import {Eye, Pencil, Trash2} from '@wso2/oxygen-ui-icons-react';
import {useCallback, useMemo, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import AgentTypeDeleteDialog from './edit-agent-type/AgentTypeDeleteDialog';
import useGetAllAgentTypes from '../api/useGetAllAgentTypes';
import useAgentTypeRoutes from '../hooks/useAgentTypeRoutes';
import type {AgentTypeListItem} from '../models/agent-type';

/**
 * Table of every agent type, with row actions to edit or delete one. Choosing a row opens the
 * agent type.
 *
 * @public
 */
export default function AgentTypesList(): JSX.Element {
  const navigate = useNavigate();
  const {t} = useTranslation();
  const logger = useLogger('AgentTypesList');
  const routes = useAgentTypeRoutes();
  const dataGridLocaleText = useDataGridLocaleText();

  const {data: agentTypes, isLoading, error: agentTypesRequestError, refetch} = useGetAllAgentTypes();

  const [selectedAgentTypeId, setSelectedAgentTypeId] = useState<string | null>(null);

  const handleDeleteClick = useCallback((agentTypeId: string): void => {
    setSelectedAgentTypeId(agentTypeId);
  }, []);

  const handleViewClick = useCallback(
    (agentTypeId: string): void => {
      (async (): Promise<void> => {
        await navigate(routes.detail(agentTypeId));
      })().catch((error: unknown) => {
        logger.error('Failed to navigate to agent type', {error, agentTypeId});
      });
    },
    [logger, navigate, routes],
  );

  const columns: DataGrid.GridColDef<AgentTypeListItem>[] = useMemo(
    () => [
      {
        field: 'displayName',
        headerName: t('agentTypes:listing.columns.name', 'Name'),
        flex: 1.5,
        minWidth: 220,
        renderCell: (params: DataGrid.GridRenderCellParams<AgentTypeListItem>) => (
          <Stack justifyContent="center" sx={{height: '100%'}}>
            <Typography variant="body2">{params.row.displayName}</Typography>
            <Typography variant="caption" color="text.secondary">
              {params.row.handle}
            </Typography>
          </Stack>
        ),
      },
      {
        field: 'id',
        headerName: t('agentTypes:listing.columns.id', 'Agent type ID'),
        width: 350,
        renderCell: (params: DataGrid.GridRenderCellParams<AgentTypeListItem>) => (
          <Typography variant="body2" sx={{fontFamily: 'monospace'}}>
            {params.row.id}
          </Typography>
        ),
      },
      {
        field: 'ouHandle',
        headerName: t('agentTypes:listing.columns.organizationUnit', 'Organization unit'),
        flex: 1,
        minWidth: 220,
        renderCell: (params: DataGrid.GridRenderCellParams<AgentTypeListItem>) => (
          <Typography variant="body2" sx={{fontFamily: 'monospace'}}>
            {params.row.ouHandle ?? params.row.ouId ?? t('common:messages.noData', 'No data available')}
          </Typography>
        ),
      },
      {
        field: 'actions',
        headerName: t('agentTypes:listing.columns.actions', 'Actions'),
        width: 150,
        align: 'center',
        headerAlign: 'center',
        sortable: false,
        filterable: false,
        hideable: false,
        renderCell: (params: DataGrid.GridRenderCellParams<AgentTypeListItem>) => (
          <ListingTable.RowActions>
            {params.row.isReadOnly ? (
              <Tooltip title={t('common:status.readOnly', 'Read only')}>
                <IconButton
                  aria-label={t('common:status.readOnly', 'Read only')}
                  size="small"
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
                    aria-label={t('common:actions.edit', 'Edit')}
                    size="small"
                    onClick={(e) => {
                      e.stopPropagation();
                      handleViewClick(params.row.id);
                    }}
                  >
                    <Pencil size={16} />
                  </IconButton>
                </Tooltip>
                <Tooltip title={t('common:actions.delete', 'Delete')}>
                  <IconButton
                    aria-label={t('common:actions.delete', 'Delete')}
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
    [t, handleDeleteClick, handleViewClick],
  );

  if (agentTypesRequestError) {
    return (
      <QueryErrorNotice
        error={agentTypesRequestError}
        t={(key, options) => t(key.includes(':') ? key : `agentTypes:${key}`, options)}
        variant="block"
        title={t('agentTypes:listing.error', 'Failed to load agent types')}
        onRetry={() => void refetch()}
      />
    );
  }

  return (
    <>
      <ListingTable.Provider variant="data-grid-card" loading={isLoading}>
        <ListingTable.Container disablePaper>
          <ListingTable.DataGrid
            rows={agentTypes ?? []}
            columns={columns}
            getRowId={(row) => (row as AgentTypeListItem).id}
            onRowClick={(params) => {
              handleViewClick((params.row as AgentTypeListItem).id);
            }}
            initialState={{
              pagination: {
                paginationModel: {pageSize: 10},
              },
            }}
            pageSizeOptions={[5, 10, 25, 50]}
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

      <AgentTypeDeleteDialog
        open={selectedAgentTypeId !== null}
        agentTypeId={selectedAgentTypeId}
        onClose={() => setSelectedAgentTypeId(null)}
      />
    </>
  );
}
