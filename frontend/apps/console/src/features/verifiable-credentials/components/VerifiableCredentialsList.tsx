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
import {Box, IconButton, Typography, Tooltip, DataGrid, ListingTable} from '@wso2/oxygen-ui';
import {Pencil, QrCode as QrCodeIcon, Trash2} from '@wso2/oxygen-ui-icons-react';
import {useMemo, useCallback, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import CredentialOfferDialog from './CredentialOfferDialog';
import VerifiableCredentialDeleteDialog from './VerifiableCredentialDeleteDialog';
import RouteConfig from '../../../configs/RouteConfig';
import useGetVerifiableCredentials from '../api/useGetVerifiableCredentials';
import type {VerifiableCredentialSummary} from '../models/vc';

/**
 * DataGrid listing of OpenID4VCI credential configurations.
 */
export default function VerifiableCredentialsList(): JSX.Element {
  const navigate = useNavigate();
  const {t} = useTranslation();
  const logger = useLogger('VerifiableCredentialsList');
  const dataGridLocaleText = useDataGridLocaleText();

  const {data, isLoading, error} = useGetVerifiableCredentials();

  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState<boolean>(false);
  const [offerHandle, setOfferHandle] = useState<string | null>(null);

  const handleEditClick = useCallback(
    (id: string): void => {
      (async (): Promise<void> => {
        await navigate(RouteConfig.verifiableCredentials.detail(id));
      })().catch((_error: unknown) => {
        logger.error('Failed to navigate to credential configuration', {error: _error, id});
      });
    },
    [navigate, logger],
  );

  const handleDeleteClick = useCallback((id: string): void => {
    setSelectedId(id);
    setDeleteDialogOpen(true);
  }, []);

  const handleDeleteDialogClose = (): void => {
    setDeleteDialogOpen(false);
    setSelectedId(null);
  };

  const columns: DataGrid.GridColDef<VerifiableCredentialSummary>[] = useMemo(
    () => [
      {
        field: 'name',
        headerName: t('verifiable-credentials:listing.columns.name'),
        flex: 1,
        minWidth: 180,
        renderCell: (params: DataGrid.GridRenderCellParams<VerifiableCredentialSummary>): JSX.Element => (
          <Typography variant="body2">{params.row.name ?? '-'}</Typography>
        ),
      },
      {
        field: 'ouHandle',
        headerName: t('verifiable-credentials:listing.columns.organizationUnit'),
        flex: 1,
        minWidth: 160,
        renderCell: (params: DataGrid.GridRenderCellParams<VerifiableCredentialSummary>): JSX.Element => (
          <Typography variant="body2" sx={{fontFamily: 'monospace', fontSize: '0.875rem'}}>
            {params.row.ouHandle ?? params.row.ouId ?? '-'}
          </Typography>
        ),
      },
      {
        field: 'actions',
        headerName: t('verifiable-credentials:listing.columns.actions'),
        width: 160,
        align: 'center',
        headerAlign: 'center',
        sortable: false,
        filterable: false,
        hideable: false,
        renderCell: (params: DataGrid.GridRenderCellParams<VerifiableCredentialSummary>): JSX.Element => (
          <ListingTable.RowActions>
            <Tooltip title={t('verifiable-credentials:listing.offer')}>
              <IconButton
                size="small"
                onClick={(e) => {
                  e.stopPropagation();
                  setOfferHandle(params.row.handle);
                }}
              >
                <QrCodeIcon size={16} />
              </IconButton>
            </Tooltip>
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
          {t('verifiable-credentials:listing.error')}
        </Typography>
        <Typography variant="body2" color="text.secondary">
          {error.message ?? t('common:messages.somethingWentWrong')}
        </Typography>
      </Box>
    );
  }

  return (
    <>
      <ListingTable.Provider variant="data-grid-card" loading={isLoading}>
        <ListingTable.Container disablePaper>
          <ListingTable.DataGrid
            rows={data ?? []}
            columns={columns}
            getRowId={(row) => (row as VerifiableCredentialSummary).id}
            onRowClick={(params) => {
              handleEditClick((params.row as VerifiableCredentialSummary).id);
            }}
            pageSizeOptions={[5, 10, 25]}
            disableRowSelectionOnClick
            localeText={dataGridLocaleText}
            autoHeight
            sx={{
              '& .MuiDataGrid-row': {cursor: 'pointer'},
            }}
          />
        </ListingTable.Container>
      </ListingTable.Provider>

      <VerifiableCredentialDeleteDialog open={deleteDialogOpen} vcId={selectedId} onClose={handleDeleteDialogClose} />

      <CredentialOfferDialog
        open={offerHandle !== null}
        handle={offerHandle}
        onClose={(): void => setOfferHandle(null)}
      />
    </>
  );
}
