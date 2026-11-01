// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  AccessStatusIndicator,
  QueryErrorNotice,
  ResourceAvatar,
  CardListingTable,
  type CardListingTableColumn,
} from '@thunderid/components';
import {useConfig} from '@thunderid/contexts';
import {useLogger} from '@thunderid/logger/react';
import {Box, Chip, IconButton, Tooltip, Typography, ListingTable} from '@wso2/oxygen-ui';
import {Eye, Pencil, Trash2} from '@wso2/oxygen-ui-icons-react';
import {useCallback, useMemo, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import ApplicationDeleteDialog from './ApplicationDeleteDialog';
import useGetApplications from '../api/useGetApplications';
import ApplicationConstants from '../constants/application-constants';
import useApplicationRoutes from '../hooks/useApplicationRoutes';
import type {BasicApplication} from '../models/application';
import getApplicationErrorMessage from '../utils/getApplicationErrorMessage';
import getTemplateMetadata from '../utils/getTemplateMetadata';
import isSystemConsoleClient from '../utils/isSystemConsoleClient';

export default function ApplicationsList(): JSX.Element {
  const routes = useApplicationRoutes();
  const navigate = useNavigate();
  const {config} = useConfig();
  const {t} = useTranslation();
  const logger = useLogger('ApplicationsList');
  const {data, isLoading, error, refetch} = useGetApplications();
  const consoleClientId = config?.client?.client_id;

  // Resolves an error through the `applications` catalog. `t` defaults to the `common` namespace,
  // so this forwards explicit `ns:` prefixes unchanged and prefixes bare keys with `applications:`,
  // per getErrorMessage's namespace-resolution contract.
  const tForErrors = useCallback(
    (key: string, options?: Record<string, unknown>): string =>
      t(key.includes(':') ? key : `applications:${key}`, options),
    [t],
  );

  const [selectedAppId, setSelectedAppId] = useState<string | null>(null);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState<boolean>(false);

  const handleDeleteClick = useCallback((appId: string): void => {
    setSelectedAppId(appId);
    setDeleteDialogOpen(true);
  }, []);

  const handleEditClick = useCallback(
    (appId: string): void => {
      (async (): Promise<void> => {
        await navigate(routes.applications.detail(appId));
      })().catch((_error: unknown) => {
        logger.error('Failed to navigate to application', {error: _error, applicationId: appId});
      });
    },
    [logger, navigate, routes],
  );

  const handleDeleteDialogClose = (): void => {
    setDeleteDialogOpen(false);
    setSelectedAppId(null);
  };

  const columns: CardListingTableColumn<BasicApplication>[] = useMemo(
    () => [
      {
        field: 'name',
        header: t('applications:listing.columns.name'),
        sortValue: (row) => row.name,
        minWidth: 260,
        render: (row: BasicApplication): JSX.Element => (
          <ListingTable.CellIcon
            sx={{width: '100%'}}
            icon={
              <ResourceAvatar
                variant="rounded"
                value={row.logoUrl}
                size={30}
                fallback={ApplicationConstants.DEFAULT_AVATAR}
              />
            }
            primary={row.name}
            secondary={row.description}
          />
        ),
      },
      {
        field: 'template',
        header: t('applications:listing.columns.template'),
        sortValue: (row) => getTemplateMetadata(row.template)?.displayName,
        minWidth: 120,
        render: (row: BasicApplication): JSX.Element => {
          const templateMetadata = getTemplateMetadata(row.template);
          return templateMetadata ? (
            <Chip label={templateMetadata.displayName} size="small" variant="outlined" />
          ) : (
            <>-</>
          );
        },
      },
      {
        field: 'clientId',
        header: t('applications:listing.columns.clientId'),
        sortValue: (row) => row.clientId,
        minWidth: 200,
        render: (row: BasicApplication): JSX.Element =>
          row.clientId ? (
            <Typography variant="body2" sx={{fontFamily: 'monospace', fontSize: '0.875rem'}}>
              {row.clientId}
            </Typography>
          ) : (
            <>-</>
          ),
      },
      {
        field: 'status',
        header: t('applications:listing.columns.status', 'Status'),
        minWidth: 110,
        render: (row: BasicApplication): JSX.Element => (
          <AccessStatusIndicator status={row.status} category="application" />
        ),
      },
      {
        field: 'actions',
        header: t('applications:listing.columns.actions'),
        width: 150,
        align: 'center',
        render: (row: BasicApplication): JSX.Element => (
          <ListingTable.RowActions>
            {row.isReadOnly ? (
              <Tooltip title={t('common:status.readOnly', 'Read only')}>
                <IconButton
                  size="small"
                  disableRipple
                  sx={{cursor: 'default'}}
                  aria-label={t('common:status.readOnly', 'Read only')}
                >
                  <Eye size={16} />
                </IconButton>
              </Tooltip>
            ) : (
              <>
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
                {!isSystemConsoleClient(row.clientId, consoleClientId) && (
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
                )}
              </>
            )}
          </ListingTable.RowActions>
        ),
      },
    ],
    [consoleClientId, handleDeleteClick, handleEditClick, t],
  );

  if (error) {
    return (
      <QueryErrorNotice
        error={error}
        t={tForErrors}
        title={t('applications:listing.error', 'Failed to load applications')}
        resolveErrorMessage={getApplicationErrorMessage}
        onRetry={() => void refetch()}
      />
    );
  }

  return (
    <Box data-testid="applications-list">
      <CardListingTable
        rows={data?.applications ?? []}
        columns={columns}
        getRowId={(row) => row.id}
        onRowClick={(row) => {
          handleEditClick(row.id);
        }}
        loading={isLoading}
        pageSizeOptions={[5, 10, 25, 50]}
      />

      <ApplicationDeleteDialog
        open={deleteDialogOpen}
        applicationId={selectedAppId}
        onClose={handleDeleteDialogClose}
      />
    </Box>
  );
}
