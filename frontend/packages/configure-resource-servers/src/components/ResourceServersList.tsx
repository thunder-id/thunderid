// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {QueryErrorNotice} from '@thunderid/components';
import {useDataGridLocaleText} from '@thunderid/hooks';
import {useLogger} from '@thunderid/logger/react';
import {Box, Chip, DataGrid, IconButton, ListingTable, Menu, MenuItem, Tooltip, Typography} from '@wso2/oxygen-ui';
import {EllipsisVertical, Eye, Pencil, Trash2} from '@wso2/oxygen-ui-icons-react';
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
  const dataGridLocaleText = useDataGridLocaleText();

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

  const columns: DataGrid.GridColDef<ResourceServer>[] = useMemo(
    () => [
      {
        field: 'name',
        headerName: t('resourceServers:listing.columns.name', 'Name'),
        flex: 1,
        minWidth: 200,
        renderCell: (params: DataGrid.GridRenderCellParams<ResourceServer>) => (
          <Box sx={{display: 'flex', flexDirection: 'column', justifyContent: 'center'}}>
            <Box sx={{display: 'flex', alignItems: 'center', gap: 1}}>
              <Typography variant="body2" fontWeight={500}>
                {params.row.name}
              </Typography>
              {params.row.id === defaultId && (
                <Chip
                  label={t('resourceServers:listing.default', 'Default')}
                  size="small"
                  color="primary"
                  variant="outlined"
                  sx={{height: 20, fontSize: '0.65rem'}}
                />
              )}
            </Box>
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
            label={getResourceServerTypeLabel(params.row.type, t)}
            size="small"
            color="primary"
            variant="outlined"
            sx={{fontSize: '0.7rem'}}
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
              -
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
                        await navigate(routes.detail(params.row.id));
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
                <Tooltip title={t('resourceServers:listing.actions.more', 'More actions')}>
                  <IconButton
                    size="small"
                    aria-label={t('resourceServers:listing.actions.more', 'More actions')}
                    onClick={(e) => handleMenuOpen(e, params.row)}
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
      <ListingTable.Provider variant="data-grid-card" loading={isLoading}>
        <ListingTable.Container disablePaper>
          <ListingTable.DataGrid
            rows={data?.resourceServers ?? []}
            columns={columns}
            getRowId={(row) => (row as ResourceServer).id}
            onRowClick={(params) => {
              (async (): Promise<void> => {
                await navigate(routes.detail((params.row as ResourceServer).id));
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
            // Filtering is not wired end to end, so the column filter panel stays hidden.
            disableColumnFilter
            localeText={dataGridLocaleText}
            autoHeight
            sx={{'& .MuiDataGrid-row': {cursor: 'pointer'}}}
          />
        </ListingTable.Container>
      </ListingTable.Provider>

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
