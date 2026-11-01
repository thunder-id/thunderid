// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {QueryErrorNotice, CardListingTable, type CardListingTableColumn} from '@thunderid/components';
import {useLogger} from '@thunderid/logger/react';
import {
  Box,
  Chip,
  DataGrid,
  IconButton,
  ListingTable,
  Menu,
  MenuItem,
  Stack,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import {EllipsisVertical, Eye, Pencil, Server, Trash2} from '@wso2/oxygen-ui-icons-react';
import {useCallback, useMemo, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import ResourceServerDeleteDialog from './ResourceServerDeleteDialog';
import SetDefaultResourceServerDialog from './SetDefaultResourceServerDialog';
import useGetDefaultResourceServer from '../api/useGetDefaultResourceServer';
import useGetResourceServers from '../api/useGetResourceServers';
import {getResourceServerTypeLabel} from '../config/resource-server-types';
import useResourceServerRoutes from '../hooks/useResourceServerRoutes';
import {isDefaultEligibleType, type ResourceServer} from '../models/resource-server';

export default function ResourceServersList(): JSX.Element {
  const navigate = useNavigate();
  const routes = useResourceServerRoutes();
  const {t} = useTranslation();
  const logger = useLogger('ResourceServersList');

  // Resolves an error through the `resourceServers` catalog. `t` defaults to the `common`
  // namespace, so this forwards explicit `ns:` prefixes unchanged and prefixes bare keys with
  // `resourceServers:`, per getErrorMessage's namespace-resolution contract.
  const tForErrors = useCallback(
    (key: string, options?: Record<string, unknown>): string =>
      t(key.includes(':') ? key : `resourceServers:${key}`, options),
    [t],
  );

  const [paginationModel, setPaginationModel] = useState<DataGrid.GridPaginationModel>({pageSize: 10, page: 0});
  const [deleteTarget, setDeleteTarget] = useState<ResourceServer | null>(null);
  const [setDefaultTarget, setSetDefaultTarget] = useState<ResourceServer | null>(null);
  const [menuAnchor, setMenuAnchor] = useState<HTMLElement | null>(null);
  const [menuTarget, setMenuTarget] = useState<ResourceServer | null>(null);

  const {data, isLoading, error, refetch} = useGetResourceServers({
    limit: paginationModel.pageSize,
    offset: paginationModel.page * paginationModel.pageSize,
  });
  const {data: defaultConfig, isLoading: isDefaultLoading, error: defaultError} = useGetDefaultResourceServer();
  const defaultId = defaultConfig?.merged?.resourceServerId;
  // A declarative (read-only) default is locked: the backend rejects any write to it.
  const isDefaultReady = !isDefaultLoading && !defaultError;
  const isDefaultLocked = Boolean(defaultConfig?.readOnly?.resourceServerId);

  // Why the action cannot apply to this row, or undefined when it can. It stays visible either way
  // so the actions column keeps its shape, and the reason is shown so a disabled entry is not a
  // mystery.
  const makeDefaultBlockedReason = useCallback(
    (resourceServer: ResourceServer): string | undefined => {
      if (!isDefaultReady) {
        return t('resourceServers:setDefault.unavailableReason', 'Checking the current default resource server.');
      }
      if (isDefaultLocked) {
        return t(
          'resourceServers:setDefault.lockedReason',
          'The default resource server is fixed by the deployment configuration.',
        );
      }
      if (resourceServer.id === defaultId) {
        return t('resourceServers:setDefault.alreadyDefaultReason', 'This is already the default resource server.');
      }
      if (!isDefaultEligibleType(resourceServer.type)) {
        return t(
          'resourceServers:setDefault.ineligibleTypeReason',
          'Only API and custom resource servers can be the default.',
        );
      }
      return undefined;
    },
    [t, isDefaultReady, isDefaultLocked, defaultId],
  );

  const handleMenuOpen = useCallback((event: React.MouseEvent<HTMLElement>, resourceServer: ResourceServer): void => {
    event.stopPropagation();
    setMenuAnchor(event.currentTarget);
    setMenuTarget(resourceServer);
  }, []);

  const menuBlockedReason = menuTarget ? makeDefaultBlockedReason(menuTarget) : undefined;

  const handleMenuClose = useCallback((): void => {
    setMenuAnchor(null);
    setMenuTarget(null);
  }, []);

  const columns: CardListingTableColumn<ResourceServer>[] = useMemo(
    () => [
      {
        field: 'name',
        header: t('resourceServers:listing.columns.name', 'Name'),
        sortValue: (row) => row.name,
        minWidth: 200,
        render: (row: ResourceServer) => (
          <ListingTable.CellIcon
            icon={<Server size={20} />}
            primary={
              <Stack direction="row" spacing={1} alignItems="center">
                <span>{row.name}</span>
                {row.id === defaultId && (
                  <Chip label={t('resourceServers:listing.default', 'Default')} size="small" variant="outlined" />
                )}
              </Stack>
            }
            secondary={
              row.isReadOnly ? (
                <Chip
                  label={t('resourceServers:listing.systemResourceServer', 'System resource server')}
                  size="small"
                  sx={{mt: 0.25, height: 18, fontSize: '0.65rem', width: 'fit-content'}}
                />
              ) : undefined
            }
            sx={{width: '100%'}}
          />
        ),
      },
      {
        field: 'type',
        header: t('resourceServers:listing.columns.type', 'Type'),
        sortValue: (row) => row.type,
        minWidth: 120,
        render: (row: ResourceServer) => (
          <Chip label={getResourceServerTypeLabel(row.type, t)} size="small" variant="outlined" />
        ),
      },
      {
        field: 'identifier',
        header: t('resourceServers:listing.columns.identifier', 'Identifier'),
        sortValue: (row) => row.identifier,
        minWidth: 240,
        render: (row: ResourceServer) =>
          row.identifier ? (
            <Typography variant="body2" color="text.secondary" sx={{fontFamily: 'monospace', fontSize: '0.8rem'}}>
              {row.identifier}
            </Typography>
          ) : (
            <Typography variant="body2" color="text.disabled">
              -
            </Typography>
          ),
      },
      {
        field: 'actions',
        header: t('resourceServers:listing.columns.actions', 'Actions'),
        width: 150,
        align: 'center',
        render: (row: ResourceServer): JSX.Element => (
          <ListingTable.RowActions>
            {row.isReadOnly ? (
              <Tooltip title={t('common:status.readOnly', 'Read only')}>
                <IconButton
                  size="small"
                  disableRipple
                  aria-label={t('common:status.readOnly', 'Read only')}
                  sx={{cursor: 'default'}}
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
                      (async (): Promise<void> => {
                        await navigate(routes.detail(row.id));
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
                    aria-label={t('common:actions.delete', 'Delete')}
                    onClick={(e) => {
                      e.stopPropagation();
                      setDeleteTarget(row);
                    }}
                  >
                    <Trash2 size={16} />
                  </IconButton>
                </Tooltip>
                <Tooltip title={t('resourceServers:listing.actions.more', 'More actions')}>
                  <IconButton
                    size="small"
                    aria-label={t('resourceServers:listing.actions.more', 'More actions')}
                    onClick={(e) => handleMenuOpen(e, row)}
                  >
                    <EllipsisVertical size={16} />
                  </IconButton>
                </Tooltip>
              </>
            )}
          </ListingTable.RowActions>
        ),
      },
    ],
    [t, navigate, routes, logger, defaultId, handleMenuOpen],
  );

  if (error) {
    return (
      <QueryErrorNotice
        error={error}
        t={tForErrors}
        variant="block"
        title={t('resourceServers:listing.error', 'Failed to load resource servers')}
        onRetry={() => void refetch()}
      />
    );
  }

  return (
    <>
      <CardListingTable
        rows={data?.resourceServers ?? []}
        columns={columns}
        getRowId={(row) => row.id}
        onRowClick={(row) => {
          (async (): Promise<void> => {
            await navigate(routes.detail(row.id));
          })().catch((err: unknown) => {
            logger.error('Failed to navigate to resource server detail', {error: err});
          });
        }}
        loading={isLoading}
        paginationMode="server"
        rowCount={data?.totalResults ?? 0}
        paginationModel={paginationModel}
        onPaginationModelChange={setPaginationModel}
        pageSizeOptions={[5, 10, 25]}
      />

      <Menu
        anchorEl={menuAnchor}
        open={menuAnchor !== null}
        onClose={handleMenuClose}
        anchorOrigin={{vertical: 'bottom', horizontal: 'right'}}
        transformOrigin={{vertical: 'top', horizontal: 'right'}}
      >
        <MenuItem
          disabled={menuBlockedReason !== undefined}
          onClick={() => {
            // A disabled MenuItem is a list item, so it keeps its handler and is only kept inert by
            // pointer-events. Guard here too, whatever dispatches the click.
            if (menuBlockedReason !== undefined) return;
            setSetDefaultTarget(menuTarget);
            handleMenuClose();
          }}
        >
          <Box>
            <Typography variant="body2">
              {t('resourceServers:setDefault.action', 'Make default resource server')}
            </Typography>
            {menuBlockedReason !== undefined && (
              <Typography variant="caption" color="text.secondary" display="block">
                {menuBlockedReason}
              </Typography>
            )}
          </Box>
        </MenuItem>
      </Menu>

      <ResourceServerDeleteDialog
        open={deleteTarget !== null}
        resourceServer={deleteTarget}
        onClose={() => setDeleteTarget(null)}
        onSuccess={() => setDeleteTarget(null)}
      />

      <SetDefaultResourceServerDialog
        open={setDefaultTarget !== null}
        resourceServer={setDefaultTarget}
        onClose={() => setSetDefaultTarget(null)}
        onSuccess={() => setSetDefaultTarget(null)}
      />
    </>
  );
}
