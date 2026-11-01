// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {QueryErrorNotice, ResourceAvatar, CardListingTable, type CardListingTableColumn} from '@thunderid/components';
import {getDisplayNameForCode, toFlagEmoji, useGetLanguages} from '@thunderid/i18n';
import {useLogger} from '@thunderid/logger/react';
import {Chip, IconButton, ListingTable, Tooltip} from '@wso2/oxygen-ui';
import {Pencil, Trash2} from '@wso2/oxygen-ui-icons-react';
import {useCallback, useMemo, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import TranslationConstants from '../constants/translation-constants';
import TranslationDeleteDialog from '@/components/TranslationDeleteDialog';
import useTranslationRoutes from '@/hooks/useTranslationRoutes';

export default function TranslationsList(): JSX.Element {
  const {t} = useTranslation('translations');
  const navigate = useNavigate();
  const logger = useLogger('TranslationsList');
  const routes = useTranslationRoutes();

  const [selectedLanguage, setSelectedLanguage] = useState<string | null>(null);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState<boolean>(false);

  const {data, isLoading, error, refetch} = useGetLanguages();

  const handleEditClick = useCallback(
    (language: string): void => {
      (async (): Promise<void> => {
        await navigate(routes.detail(language));
      })().catch((_error: unknown) => {
        logger.error('Failed to navigate to translation editor', {error: _error, language});
      });
    },
    [logger, navigate, routes],
  );

  const handleDeleteClick = useCallback((language: string): void => {
    setSelectedLanguage(language);
    setDeleteDialogOpen(true);
  }, []);

  const handleDeleteDialogClose = (): void => {
    setDeleteDialogOpen(false);
    setSelectedLanguage(null);
  };

  const rows = useMemo(() => (data?.languages ?? []).map((code) => ({id: code, code})), [data?.languages]);

  const columns: CardListingTableColumn<{id: string; code: string}>[] = useMemo(
    () => [
      {
        field: 'code',
        header: t('listing.columns.language'),
        sortValue: (row) => row.code,
        minWidth: 240,
        render: (row: {id: string; code: string}): JSX.Element => (
          <ListingTable.CellIcon
            sx={{width: '100%'}}
            icon={
              <ResourceAvatar
                transparent
                variant="rounded"
                value={toFlagEmoji(row.code)}
                size={30}
                fallback={TranslationConstants.DEFAULT_AVATAR}
                sx={{fontSize: '1.8rem'}}
              />
            }
            primary={getDisplayNameForCode(row.code)}
            secondary={
              <Chip
                label={row.code}
                size="small"
                variant="outlined"
                sx={{fontSize: '0.7rem', fontFamily: 'monospace', height: 18}}
              />
            }
          />
        ),
      },
      {
        field: 'actions',
        header: t('listing.columns.actions'),
        width: 150,
        align: 'center',
        render: (row: {id: string; code: string}): JSX.Element => (
          <ListingTable.RowActions>
            <Tooltip title={t('common:actions.edit', 'Edit')}>
              <IconButton
                size="small"
                aria-label={t('common:actions.edit', 'Edit')}
                onClick={(e) => {
                  e.stopPropagation();
                  handleEditClick(row.code);
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
                  handleDeleteClick(row.code);
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
        t={t}
        title={t('listing.error', 'Failed to load languages')}
        onRetry={() => void refetch()}
      />
    );
  }

  return (
    <>
      <CardListingTable
        rows={rows}
        columns={columns}
        getRowId={(row) => row.id}
        onRowClick={(row) => {
          handleEditClick(row.code);
        }}
        loading={isLoading}
        pageSizeOptions={[5, 10, 25]}
      />

      <TranslationDeleteDialog open={deleteDialogOpen} language={selectedLanguage} onClose={handleDeleteDialogClose} />
    </>
  );
}
