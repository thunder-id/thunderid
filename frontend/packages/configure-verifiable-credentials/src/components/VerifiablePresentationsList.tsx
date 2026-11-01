// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {QueryErrorNotice, CardListingTable, type CardListingTableColumn} from '@thunderid/components';
import {useLogger} from '@thunderid/logger/react';
import {IconButton, Typography, Tooltip, ListingTable} from '@wso2/oxygen-ui';
import {Pencil, QrCode as QrCodeIcon, ShieldCheck, Trash2} from '@wso2/oxygen-ui-icons-react';
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

  const columns: CardListingTableColumn<VerifiablePresentationSummary>[] = useMemo(
    () => [
      {
        field: 'name',
        header: t('verifiable-presentations:listing.columns.name'),
        sortValue: (row) => row.name,
        minWidth: 180,
        render: (row: VerifiablePresentationSummary): JSX.Element => (
          <ListingTable.CellIcon
            icon={<ShieldCheck size={20} />}
            primary={row.name ?? '-'}
            secondary={row.handle}
            sx={{width: '100%'}}
          />
        ),
      },
      {
        field: 'ouHandle',
        header: t('verifiable-presentations:listing.columns.organizationUnit'),
        sortValue: (row) => row.ouHandle,
        minWidth: 160,
        render: (row: VerifiablePresentationSummary): JSX.Element => (
          <Typography variant="body2" sx={{fontFamily: 'monospace', fontSize: '0.875rem'}}>
            {row.ouHandle ?? row.ouId ?? '-'}
          </Typography>
        ),
      },
      {
        field: 'actions',
        header: t('verifiable-presentations:listing.columns.actions'),
        width: 160,
        align: 'center',
        render: (row: VerifiablePresentationSummary): JSX.Element => (
          <ListingTable.RowActions>
            <Tooltip title={t('verifiable-presentations:listing.verify', 'Verify')}>
              <IconButton
                size="small"
                aria-label={t('verifiable-presentations:listing.verify', 'Verify')}
                onClick={(e) => {
                  e.stopPropagation();
                  setVerifyHandle(row.handle);
                }}
              >
                <QrCodeIcon size={16} />
              </IconButton>
            </Tooltip>
            <Tooltip title={t('common:actions.edit', 'Edit')}>
              <IconButton
                size="small"
                aria-label={t('common:actions.edit', 'Edit')}
                onClick={(e) => {
                  e.stopPropagation();
                  handleEditClick(row.id);
                }}
              >
                <Pencil size={16} />
              </IconButton>
            </Tooltip>
            <Tooltip title={t('common:actions.delete', 'Delete')}>
              <IconButton
                size="small"
                color="error"
                aria-label={t('common:actions.delete', 'Delete')}
                onClick={(e) => {
                  e.stopPropagation();
                  handleDeleteClick(row.id);
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
      <CardListingTable
        rows={data ?? []}
        columns={columns}
        getRowId={(row) => row.id}
        onRowClick={(row) => {
          handleEditClick(row.id);
        }}
        loading={isLoading}
        pageSizeOptions={[5, 10, 25]}
      />

      <VerifiablePresentationDeleteDialog open={deleteDialogOpen} vpId={selectedId} onClose={handleDeleteDialogClose} />

      <VerificationDialog
        open={verifyHandle !== null}
        handle={verifyHandle}
        onClose={(): void => setVerifyHandle(null)}
      />
    </>
  );
}
