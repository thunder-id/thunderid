// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {QueryErrorNotice} from '@thunderid/components';
import {useDataGridLocaleText} from '@thunderid/hooks';
import {useLogger} from '@thunderid/logger/react';
import {Chip, IconButton, Tooltip, ListingTable, DataGrid} from '@wso2/oxygen-ui';
import {Eye, Pencil, Trash2} from '@wso2/oxygen-ui-icons-react';
import {useMemo, useCallback, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import FlowDeleteDialog from './FlowDeleteDialog';
import useGetFlows from '../api/useGetFlows';
import useFlowRoutes from '../hooks/useFlowRoutes';
import type {BasicFlowDefinition} from '../models/responses';

export default function FlowsList(): JSX.Element {
  const navigate = useNavigate();
  const flowRoutes = useFlowRoutes();
  const {t} = useTranslation();
  const logger = useLogger('FlowsList');
  const dataGridLocaleText = useDataGridLocaleText();
  const {data, isLoading, error, refetch} = useGetFlows();

  // Resolves an error through the `flows` catalog. `t` defaults to the `common` namespace, so
  // this forwards explicit `ns:` prefixes unchanged and prefixes bare keys with `flows:`, per
  // getErrorMessage's namespace-resolution contract.
  const tForErrors = useCallback(
    (key: string, options?: Record<string, unknown>): string => t(key.includes(':') ? key : `flows:${key}`, options),
    [t],
  );

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

  const handleOpenClick = useCallback(
    (flow: BasicFlowDefinition): void => {
      (async (): Promise<void> => {
        await navigate(flowRoutes.flows.detail(flow.id));
      })().catch((_error: unknown) => {
        logger.error('Failed to navigate to flow builder', {error: _error, flowId: flow.id});
      });
    },
    [flowRoutes, logger, navigate],
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
                <Tooltip title={t('common:actions.view', 'View')}>
                  <IconButton
                    size="small"
                    onClick={(e) => {
                      e.stopPropagation();
                      handleOpenClick(params.row);
                    }}
                  >
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
                        handleOpenClick(params.row);
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
    [handleDeleteClick, handleOpenClick, t],
  );

  if (error) {
    return (
      <QueryErrorNotice
        error={error}
        t={tForErrors}
        variant="block"
        title={t('flows:listing.error.title')}
        fallbackKey="listing.error.unknown"
        fallbackDefaultValue="An unknown error occurred"
        onRetry={() => void refetch()}
      />
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
              handleOpenClick(params.row as BasicFlowDefinition);
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

      <FlowDeleteDialog open={deleteDialogOpen} flowId={selectedFlow?.id ?? null} onClose={handleDeleteDialogClose} />
    </>
  );
}
