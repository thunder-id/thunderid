// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {QueryErrorNotice, CardListingTable, type CardListingTableColumn} from '@thunderid/components';
import {useLogger} from '@thunderid/logger/react';
import {IconButton, Typography, Tooltip, ListingTable} from '@wso2/oxygen-ui';
import {IdCard, Pencil, QrCode as QrCodeIcon, Trash2} from '@wso2/oxygen-ui-icons-react';
import {useMemo, useCallback, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import CredentialOfferDialog from './CredentialOfferDialog';
import VerifiableCredentialDeleteDialog from './VerifiableCredentialDeleteDialog';
import useGetVerifiableCredentials from '../api/useGetVerifiableCredentials';
import useVerifiableCredentialRoutes from '../hooks/useVerifiableCredentialRoutes';
import type {VerifiableCredentialSummary} from '../models/vc';

/**
 * DataGrid listing of OpenID4VCI credential configurations.
 */
export default function VerifiableCredentialsList(): JSX.Element {
  const navigate = useNavigate();
  const {t} = useTranslation();
  const logger = useLogger('VerifiableCredentialsList');
  const routes = useVerifiableCredentialRoutes();

  const {data, isLoading, error, refetch} = useGetVerifiableCredentials();

  // Resolves an error through the `verifiable-credentials` catalog. `t` defaults to the `common`
  // namespace, so this forwards explicit `ns:` prefixes unchanged and prefixes bare keys with
  // `verifiable-credentials:`, per getErrorMessage's namespace-resolution contract.
  const tForErrors = useCallback(
    (key: string, options?: Record<string, unknown>): string =>
      t(key.includes(':') ? key : `verifiable-credentials:${key}`, options),
    [t],
  );

  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState<boolean>(false);
  const [offerHandle, setOfferHandle] = useState<string | null>(null);

  const handleEditClick = useCallback(
    (id: string): void => {
      (async (): Promise<void> => {
        await navigate(routes.verifiableCredentials.detail(id));
      })().catch((_error: unknown) => {
        logger.error('Failed to navigate to credential configuration', {error: _error, id});
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

  const columns: CardListingTableColumn<VerifiableCredentialSummary>[] = useMemo(
    () => [
      {
        field: 'name',
        header: t('verifiable-credentials:listing.columns.name'),
        sortValue: (row) => row.name,
        minWidth: 180,
        render: (row: VerifiableCredentialSummary): JSX.Element => (
          <ListingTable.CellIcon
            icon={<IdCard size={20} />}
            primary={row.name ?? '-'}
            secondary={row.handle}
            sx={{width: '100%'}}
          />
        ),
      },
      {
        field: 'ouHandle',
        header: t('verifiable-credentials:listing.columns.organizationUnit'),
        sortValue: (row) => row.ouHandle,
        minWidth: 160,
        render: (row: VerifiableCredentialSummary): JSX.Element => (
          <Typography variant="body2" sx={{fontFamily: 'monospace', fontSize: '0.875rem'}}>
            {row.ouHandle ?? row.ouId ?? '-'}
          </Typography>
        ),
      },
      {
        field: 'actions',
        header: t('verifiable-credentials:listing.columns.actions'),
        width: 160,
        align: 'center',
        render: (row: VerifiableCredentialSummary): JSX.Element => (
          <ListingTable.RowActions>
            <Tooltip title={t('verifiable-credentials:listing.offer', 'Generate offer')}>
              <IconButton
                size="small"
                aria-label={t('verifiable-credentials:listing.offer', 'Generate offer')}
                onClick={(e) => {
                  e.stopPropagation();
                  setOfferHandle(row.handle);
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
        title={t('verifiable-credentials:listing.error', 'Failed to load credential templates')}
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

      <VerifiableCredentialDeleteDialog open={deleteDialogOpen} vcId={selectedId} onClose={handleDeleteDialogClose} />

      <CredentialOfferDialog
        open={offerHandle !== null}
        handle={offerHandle}
        onClose={(): void => setOfferHandle(null)}
      />
    </>
  );
}
