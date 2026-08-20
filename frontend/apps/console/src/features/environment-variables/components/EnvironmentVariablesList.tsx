// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useDataGridLocaleText} from '@thunderid/hooks';
import {Box, DataGrid, IconButton, ListingTable, Tooltip, Typography} from '@wso2/oxygen-ui';
import {Pencil, Trash2} from '@wso2/oxygen-ui-icons-react';
import {useMemo, useState, type JSX, type MouseEvent} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate, useParams} from 'react-router';
import EnvironmentVariableDeleteDialog from './EnvironmentVariableDeleteDialog';
import useGetEnvironmentVariables from '../api/useGetEnvironmentVariables';
import type {EnvironmentVariable} from '../models/environment-variable';

/**
 * Table of the environment variables applied to Data Planes.
 */
export default function EnvironmentVariablesList(): JSX.Element {
  const {envId = ''} = useParams<{envId: string}>();
  const {t} = useTranslation();
  const navigate = useNavigate();
  const dataGridLocaleText = useDataGridLocaleText();

  const [paginationModel, setPaginationModel] = useState<DataGrid.GridPaginationModel>({pageSize: 10, page: 0});
  const [deleteDialogOpen, setDeleteDialogOpen] = useState<boolean>(false);
  const [selectedId, setSelectedId] = useState<string>('');

  const params = useMemo(
    () => ({limit: paginationModel.pageSize, offset: paginationModel.page * paginationModel.pageSize}),
    [paginationModel],
  );
  const {data, isLoading, error} = useGetEnvironmentVariables(envId, params);

  const handleEdit = (id: string): void => {
    void navigate(`/promotions/${envId}/variables/${id}`);
  };

  const columns: DataGrid.GridColDef[] = useMemo(
    () => [
      {field: 'key', headerName: t('environmentVariables:listing.columns.key', 'Key'), flex: 1, minWidth: 200},
      {field: 'value', headerName: t('environmentVariables:listing.columns.value', 'Value'), flex: 1, minWidth: 200},
      {
        field: 'description',
        headerName: t('environmentVariables:listing.columns.description', 'Description'),
        flex: 1,
        minWidth: 160,
      },
      {
        field: 'actions',
        headerName: t('environmentVariables:listing.columns.actions', 'Actions'),
        sortable: false,
        width: 120,
        renderCell: (params: DataGrid.GridRenderCellParams) => (
          <ListingTable.RowActions>
            <Tooltip title={t('common:actions.edit', 'Edit')}>
              <IconButton
                size="small"
                onClick={(event: MouseEvent<HTMLButtonElement>) => {
                  event.stopPropagation();
                  handleEdit((params.row as EnvironmentVariable).id);
                }}
              >
                <Pencil size={16} />
              </IconButton>
            </Tooltip>
            <Tooltip title={t('common:actions.delete', 'Delete')}>
              <IconButton
                size="small"
                onClick={(event: MouseEvent<HTMLButtonElement>) => {
                  event.stopPropagation();
                  setSelectedId((params.row as EnvironmentVariable).id);
                  setDeleteDialogOpen(true);
                }}
              >
                <Trash2 size={16} />
              </IconButton>
            </Tooltip>
          </ListingTable.RowActions>
        ),
      },
    ],
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [t],
  );

  if (error) {
    return (
      <Box sx={{py: 8, textAlign: 'center'}}>
        <Typography variant="h6" color="error" gutterBottom>
          {t('environmentVariables:listing.error', 'Failed to load environment variables')}
        </Typography>
        <Typography variant="body2" color="text.secondary">
          {error.message ?? t('common:messages.somethingWentWrong', 'Something went wrong')}
        </Typography>
      </Box>
    );
  }

  return (
    <>
      <ListingTable.Provider variant="data-grid-card" loading={isLoading}>
        <ListingTable.Container disablePaper>
          <ListingTable.DataGrid
            rows={data?.environmentVariables ?? []}
            columns={columns}
            getRowId={(row) => (row as EnvironmentVariable).id}
            onRowClick={(params) => {
              handleEdit((params.row as EnvironmentVariable).id);
            }}
            paginationMode="server"
            rowCount={data?.totalResults ?? 0}
            paginationModel={paginationModel}
            onPaginationModelChange={setPaginationModel}
            pageSizeOptions={[5, 10, 25]}
            disableRowSelectionOnClick
            localeText={dataGridLocaleText}
            autoHeight
            sx={{'& .MuiDataGrid-row': {cursor: 'pointer'}}}
          />
        </ListingTable.Container>
      </ListingTable.Provider>

      <EnvironmentVariableDeleteDialog
        open={deleteDialogOpen}
        environmentVariableId={selectedId}
        onClose={() => {
          setDeleteDialogOpen(false);
        }}
      />
    </>
  );
}
