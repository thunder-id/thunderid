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
import {Alert, Box, Chip, DataGrid, IconButton, ListingTable, Tooltip, Typography} from '@wso2/oxygen-ui';
import {Eye, Pencil, Trash2} from '@wso2/oxygen-ui-icons-react';
import {useMemo, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import ResourceServerDeleteDialog from './ResourceServerDeleteDialog';
import useGetResourceServers from '../api/useGetResourceServers';
import {getResourceServerTypeIcon, getResourceServerTypeLabel} from '../config/resource-server-types';
import type {ResourceServer} from '../models/resource-server';

export default function ResourceServersList(): JSX.Element {
  const navigate = useNavigate();
  const {t} = useTranslation();
  const logger = useLogger('ResourceServersList');
  const dataGridLocaleText = useDataGridLocaleText();

  const [paginationModel, setPaginationModel] = useState<DataGrid.GridPaginationModel>({pageSize: 10, page: 0});
  const [deleteTarget, setDeleteTarget] = useState<ResourceServer | null>(null);

  const {data, isLoading, error} = useGetResourceServers({
    limit: paginationModel.pageSize,
    offset: paginationModel.page * paginationModel.pageSize,
  });

  const columns: DataGrid.GridColDef<ResourceServer>[] = useMemo(
    () => [
      {
        field: 'name',
        headerName: t('resourceServers:listing.columns.name', 'Name'),
        flex: 1,
        minWidth: 200,
        renderCell: (params: DataGrid.GridRenderCellParams<ResourceServer>) => (
          <Box sx={{display: 'flex', flexDirection: 'column', justifyContent: 'center'}}>
            <Typography variant="body2" fontWeight={500}>
              {params.row.name}
            </Typography>
            {params.row.isReadOnly && (
              <Chip
                label={t('resourceServers:listing.systemResourceServer', 'System resource server')}
                size="small"
                sx={{mt: 0.25, height: 18, fontSize: '0.65rem', width: 'fit-content'}}
              />
            )}
          </Box>
        ),
      },
      {
        field: 'type',
        headerName: t('resourceServers:listing.columns.type', 'Type'),
        flex: 0.8,
        minWidth: 120,
        renderCell: (params: DataGrid.GridRenderCellParams<ResourceServer>) => (
          <Chip
            icon={
              <Box sx={{display: 'flex', alignItems: 'center', '& > *': {width: 16, height: 16}}}>
                {getResourceServerTypeIcon(params.row.type)}
              </Box>
            }
            label={getResourceServerTypeLabel(params.row.type, t)}
            size="small"
            variant="outlined"
            sx={{px: 0.5, fontSize: '0.75rem'}}
          />
        ),
      },
      {
        field: 'identifier',
        headerName: t('resourceServers:listing.columns.identifier', 'Identifier'),
        flex: 1.5,
        minWidth: 240,
        renderCell: (params: DataGrid.GridRenderCellParams<ResourceServer>) =>
          params.row.identifier ? (
            <Typography variant="body2" color="text.secondary" sx={{fontFamily: 'monospace', fontSize: '0.8rem'}}>
              {params.row.identifier}
            </Typography>
          ) : (
            <Typography variant="body2" color="text.disabled">
              —
            </Typography>
          ),
      },
      {
        field: 'handle',
        headerName: t('resourceServers:listing.columns.handle', 'Handle'),
        width: 160,
        renderCell: (params: DataGrid.GridRenderCellParams<ResourceServer>) =>
          params.row.handle ? (
            <Chip
              label={params.row.handle}
              size="small"
              variant="outlined"
              sx={{fontFamily: 'monospace', fontSize: '0.75rem'}}
            />
          ) : (
            <Typography variant="body2" color="text.disabled">
              —
            </Typography>
          ),
      },
      {
        field: 'actions',
        headerName: t('resourceServers:listing.columns.actions', 'Actions'),
        width: 150,
        align: 'center',
        headerAlign: 'center',
        sortable: false,
        filterable: false,
        hideable: false,
        renderCell: (params: DataGrid.GridRenderCellParams<ResourceServer>): JSX.Element => (
          <ListingTable.RowActions>
            {params.row.isReadOnly ? (
              <Tooltip title={t('common:status.readOnly', 'Read Only')}>
                <IconButton size="small" disableRipple sx={{cursor: 'default'}}>
                  <Eye size={16} />
                </IconButton>
              </Tooltip>
            ) : (
              <>
                <Tooltip title={t('common:actions.edit', 'Edit')}>
                  <IconButton
                    size="small"
                    onClick={(e) => {
                      e.stopPropagation();
                      (async (): Promise<void> => {
                        await navigate(`/resource-servers/${params.row.id}`);
                      })().catch((err: unknown) => {
                        logger.error('Failed to navigate to resource server detail', {error: err});
                      });
                    }}
                  >
                    <Pencil size={16} />
                  </IconButton>
                </Tooltip>
                <Tooltip title={t('common:actions.delete', 'Delete')}>
                  <IconButton
                    size="small"
                    color="error"
                    onClick={(e) => {
                      e.stopPropagation();
                      setDeleteTarget(params.row);
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
    [t, navigate, logger],
  );

  if (error) {
    return (
      <Alert severity="error" sx={{mt: 2}}>
        {t('resourceServers:listing.error', 'Failed to load resource servers.')}
      </Alert>
    );
  }

  return (
    <>
      <ListingTable.Provider variant="data-grid-card" loading={isLoading}>
        <ListingTable.Container disablePaper>
          <ListingTable.DataGrid
            rows={data?.resourceServers ?? []}
            columns={columns}
            getRowId={(row) => (row as ResourceServer).id}
            onRowClick={(params) => {
              (async (): Promise<void> => {
                await navigate(`/resource-servers/${(params.row as ResourceServer).id}`);
              })().catch((err: unknown) => {
                logger.error('Failed to navigate to resource server detail', {error: err});
              });
            }}
            rowCount={data?.totalResults ?? 0}
            paginationMode="server"
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

      <ResourceServerDeleteDialog
        open={deleteTarget !== null}
        resourceServer={deleteTarget}
        onClose={() => setDeleteTarget(null)}
        onSuccess={() => setDeleteTarget(null)}
      />
    </>
  );
}
