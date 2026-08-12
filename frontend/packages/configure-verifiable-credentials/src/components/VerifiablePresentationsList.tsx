// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {QueryErrorNotice} from '@thunderid/components';
import {useDataGridLocaleText} from '@thunderid/hooks';
import {useLogger} from '@thunderid/logger/react';
import {IconButton, Typography, Tooltip, DataGrid, ListingTable} from '@wso2/oxygen-ui';
import {Pencil, QrCode as QrCodeIcon, Trash2} from '@wso2/oxygen-ui-icons-react';
import {useMemo, useCallback, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import VerifiablePresentationDeleteDialog from './VerifiablePresentationDeleteDialog';
import VerificationDialog from './VerificationDialog';
import useGetVerifiablePresentations from '../api/useGetVerifiablePresentations';
import useVerifiableCredentialRoutes from '../hooks/useVerifiableCredentialRoutes';
import type {VerifiablePresentationSummary} from '../models/vp';

/**
 * DataGrid listing of OpenID4VP presentation definitions.
 */
export default function VerifiablePresentationsList(): JSX.Element {
  const navigate = useNavigate();
  const {t} = useTranslation();
  const logger = useLogger('VerifiablePresentationsList');
  const dataGridLocaleText = useDataGridLocaleText();
  const routes = useVerifiableCredentialRoutes();

  // Resolves an error through the `verifiable-presentations` catalog. `t` defaults to the `common`
  // namespace, so this forwards explicit `ns:` prefixes unchanged and prefixes bare keys, per
  // getErrorMessage's namespace-resolution contract.
  const tForErrors = useCallback(
    (key: string, options?: Record<string, unknown>): string =>
      t(key.includes(':') ? key : `verifiable-presentations:${key}`, options),
    [t],
  );

  const {data, isLoading, error, refetch} = useGetVerifiablePresentations();

  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState<boolean>(false);
  const [verifyHandle, setVerifyHandle] = useState<string | null>(null);

  const handleEditClick = useCallback(
    (id: string): void => {
      (async (): Promise<void> => {
        await navigate(routes.verifiablePresentations.detail(id));
      })().catch((_error: unknown) => {
        logger.error('Failed to navigate to presentation definition', {error: _error, id});
      });
    },
    [navigate, logger, routes],
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
        field: 'name',
        headerName: t('verifiable-presentations:listing.columns.name'),
        flex: 1,
        minWidth: 180,
        renderCell: (params: DataGrid.GridRenderCellParams<VerifiablePresentationSummary>): JSX.Element => (
          <Typography variant="body2">{params.row.name ?? '-'}</Typography>
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
      <QueryErrorNotice
        error={error}
        t={tForErrors}
        variant="block"
        title={t('verifiable-presentations:listing.error', 'Failed to load presentation definitions')}
        onRetry={() => void refetch()}
      />
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
            // Filtering is not wired end to end, so the column filter panel stays hidden.
            disableColumnFilter
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
