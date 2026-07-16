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
import {Box, Chip, IconButton, Tooltip, Typography, ListingTable, DataGrid} from '@wso2/oxygen-ui';
import {Eye, Pencil, Trash2} from '@wso2/oxygen-ui-icons-react';
import {useMemo, useCallback, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import FlowDeleteDialog from './FlowDeleteDialog';
import useGetFlows from '../api/useGetFlows';
import type {BasicFlowDefinition} from '../models/responses';

export default function FlowsList(): JSX.Element {
  const navigate = useNavigate();
  const {t} = useTranslation();
  const logger = useLogger('FlowsList');
  const dataGridLocaleText = useDataGridLocaleText();
  const {data, isLoading, error} = useGetFlows();

  const [selectedFlow, setSelectedFlow] = useState<BasicFlowDefinition | null>(null);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState<boolean>(false);

  const handleDeleteClick = useCallback((flow: BasicFlowDefinition): void => {
    setSelectedFlow(flow);
    setDeleteDialogOpen(true);
  }, []);

  const handleDeleteDialogClose = (): void => {
    setDeleteDialogOpen(false);
    setSelectedFlow(null);
  };

  const handleEditClick = useCallback(
    (flow: BasicFlowDefinition): void => {
      (async (): Promise<void> => {
        await navigate(`/flows/signin/${flow.id}`);
      })().catch((_error: unknown) => {
        logger.error('Failed to navigate to flow builder', {error: _error, flowId: flow.id});
      });
    },
    [logger, navigate],
  );

  const columns: DataGrid.GridColDef<BasicFlowDefinition>[] = useMemo(
    () => [
      {
        field: 'name',
        headerName: t('flows:listing.columns.name'),
        flex: 1,
        minWidth: 220,
      },
      {
        field: 'flowType',
        headerName: t('flows:listing.columns.flowType'),
        width: 180,
        renderCell: (params: DataGrid.GridRenderCellParams<BasicFlowDefinition>): JSX.Element => (
          <Chip
            label={params.row.flowType}
            size="small"
            color="primary"
            variant="outlined"
            sx={{
              fontSize: '0.7rem',
            }}
          />
        ),
      },
      {
        field: 'updatedAt',
        headerName: t('flows:listing.columns.updatedAt'),
        width: 180,
        valueGetter: (_value, row): string => {
          const date = new Date(row.updatedAt);
          return date.toLocaleDateString(undefined, {
            year: 'numeric',
            month: 'short',
            day: 'numeric',
            hour: '2-digit',
            minute: '2-digit',
          });
        },
      },
      {
        field: 'actions',
        headerName: t('flows:listing.columns.actions'),
        width: 150,
        align: 'center',
        headerAlign: 'center',
        sortable: false,
        filterable: false,
        hideable: false,
        renderCell: (params: DataGrid.GridRenderCellParams<BasicFlowDefinition>): JSX.Element | null => {
          return (
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
                        handleEditClick(params.row);
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
                        handleDeleteClick(params.row);
                      }}
                    >
                      <Trash2 size={16} />
                    </IconButton>
                  </Tooltip>
                </>
              )}
            </ListingTable.RowActions>
          );
        },
      },
    ],
    [handleDeleteClick, handleEditClick, t],
  );

  if (error) {
    return (
      <Box sx={{textAlign: 'center', py: 8}}>
        <Typography variant="h6" color="error" gutterBottom>
          {t('flows:listing.error.title')}
        </Typography>
        <Typography variant="body2" color="text.secondary">
          {error.message ?? t('flows:listing.error.unknown')}
        </Typography>
      </Box>
    );
  }

  return (
    <>
      <ListingTable.Provider variant="data-grid-card" loading={isLoading}>
        <ListingTable.Container disablePaper>
          <ListingTable.DataGrid
            rows={data?.flows ?? []}
            columns={columns}
            getRowId={(row): string => (row as BasicFlowDefinition).id}
            onRowClick={(params) => {
              if (!(params.row as BasicFlowDefinition).isReadOnly) {
                handleEditClick(params.row as BasicFlowDefinition);
              }
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
              '& .MuiDataGrid-row.row-clickable': {
                cursor: 'pointer',
              },
              '& .MuiDataGrid-row.row-not-clickable': {
                cursor: 'default',
              },
            }}
          />
        </ListingTable.Container>
      </ListingTable.Provider>

      <FlowDeleteDialog open={deleteDialogOpen} flowId={selectedFlow?.id ?? null} onClose={handleDeleteDialogClose} />
    </>
  );
}
