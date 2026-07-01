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
import VerifiablePresentationDeleteDialog from './VerifiablePresentationDeleteDialog';
import VerificationDialog from './VerificationDialog';
import useGetVerifiablePresentations from '../api/useGetVerifiablePresentations';
import type {VerifiablePresentationSummary} from '../models/vp';

/**
 * DataGrid listing of OpenID4VP presentation definitions.
 */
export default function VerifiablePresentationsList(): JSX.Element {
  const navigate = useNavigate();
  const {t} = useTranslation();
  const logger = useLogger('VerifiablePresentationsList');
  const dataGridLocaleText = useDataGridLocaleText();

  const {data, isLoading, error} = useGetVerifiablePresentations();

  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState<boolean>(false);
  const [verifyHandle, setVerifyHandle] = useState<string | null>(null);

  const handleEditClick = useCallback(
    (id: string): void => {
      (async (): Promise<void> => {
        await navigate(`/verifiable-presentations/${id}`);
      })().catch((_error: unknown) => {
        logger.error('Failed to navigate to presentation definition', {error: _error, id});
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

  const columns: DataGrid.GridColDef<VerifiablePresentationSummary>[] = useMemo(
    () => [
      {
        field: 'displayName',
        headerName: t('verifiable-presentations:listing.columns.name'),
        flex: 1,
        minWidth: 180,
        renderCell: (params: DataGrid.GridRenderCellParams<VerifiablePresentationSummary>): JSX.Element => (
          <Typography variant="body2">{params.row.displayName ?? '-'}</Typography>
        ),
      },
      {
        field: 'handle',
        headerName: t('verifiable-presentations:listing.columns.handle'),
        flex: 1,
        minWidth: 180,
        renderCell: (params: DataGrid.GridRenderCellParams<VerifiablePresentationSummary>): JSX.Element => (
          <Typography variant="body2" sx={{fontFamily: 'monospace', fontSize: '0.875rem'}}>
            {params.row.handle}
          </Typography>
        ),
      },
      {
        field: 'vct',
        headerName: t('verifiable-presentations:listing.columns.vct'),
        flex: 1.5,
        minWidth: 240,
        renderCell: (params: DataGrid.GridRenderCellParams<VerifiablePresentationSummary>): JSX.Element => (
          <Typography variant="body2" sx={{fontFamily: 'monospace', fontSize: '0.875rem'}}>
            {params.row.vct}
          </Typography>
        ),
      },
      {
        field: 'ouHandle',
        headerName: t('verifiable-presentations:listing.columns.organizationUnit'),
        flex: 1,
        minWidth: 160,
        renderCell: (params: DataGrid.GridRenderCellParams<VerifiablePresentationSummary>): JSX.Element => (
          <Typography variant="body2" sx={{fontFamily: 'monospace', fontSize: '0.875rem'}}>
            {params.row.ouHandle ?? params.row.ouId ?? '-'}
          </Typography>
        ),
      },
      {
        field: 'actions',
        headerName: t('verifiable-presentations:listing.columns.actions'),
        width: 160,
        align: 'center',
        headerAlign: 'center',
        sortable: false,
        filterable: false,
        hideable: false,
        renderCell: (params: DataGrid.GridRenderCellParams<VerifiablePresentationSummary>): JSX.Element => (
          <ListingTable.RowActions>
            <Tooltip title={t('verifiable-presentations:listing.verify')}>
              <IconButton
                size="small"
                onClick={(e) => {
                  e.stopPropagation();
                  setVerifyHandle(params.row.handle);
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
          {t('verifiable-presentations:listing.error')}
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
            getRowId={(row) => (row as VerifiablePresentationSummary).id}
            onRowClick={(params) => {
              handleEditClick((params.row as VerifiablePresentationSummary).id);
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

      <VerifiablePresentationDeleteDialog open={deleteDialogOpen} vpId={selectedId} onClose={handleDeleteDialogClose} />

      <VerificationDialog
        open={verifyHandle !== null}
        handle={verifyHandle}
        onClose={(): void => setVerifyHandle(null)}
      />
    </>
  );
}
