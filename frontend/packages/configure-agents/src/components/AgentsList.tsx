// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  AccessStatusIndicator,
  QueryErrorNotice,
  ResourceAvatar,
  CardListingTable,
  type CardListingTableColumn,
} from '@thunderid/components';
import {useLogger} from '@thunderid/logger/react';
import {Box, IconButton, Tooltip, Typography, ListingTable} from '@wso2/oxygen-ui';
import {Eye, Pencil, Trash2} from '@wso2/oxygen-ui-icons-react';
import {useMemo, useCallback, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import AgentDeleteDialog from './AgentDeleteDialog';
import useGetAgents from '../api/useGetAgents';
import AgentConstants from '../constants/agent-constants';
import useAgentRoutes from '../hooks/useAgentRoutes';
import type {BasicAgent} from '../models/agent';

export default function AgentsList(): JSX.Element {
  const routes = useAgentRoutes();
  const navigate = useNavigate();
  const {t} = useTranslation();
  const logger = useLogger('AgentsList');
  const {data, isLoading, error, refetch} = useGetAgents();

  // Resolves an error through the `agents` catalog. `t` defaults to the `common` namespace, so
  // this forwards explicit `ns:` prefixes unchanged and prefixes bare keys with `agents:`, per
  // getErrorMessage's namespace-resolution contract.
  const tForErrors = useCallback(
    (key: string, options?: Record<string, unknown>): string => t(key.includes(':') ? key : `agents:${key}`, options),
    [t],
  );

  const [selectedAgentId, setSelectedAgentId] = useState<string | null>(null);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);

  const handleDeleteClick = useCallback((agentId: string): void => {
    setSelectedAgentId(agentId);
    setDeleteDialogOpen(true);
  }, []);

  const handleEditClick = useCallback(
    (agentId: string): void => {
      (async (): Promise<void> => {
        await navigate(routes.agents.detail(agentId));
      })().catch((_error: unknown) => {
        logger.error('Failed to navigate to agent', {error: _error, agentId});
      });
    },
    [logger, navigate, routes],
  );

  const handleDeleteDialogClose = (): void => {
    setDeleteDialogOpen(false);
    setSelectedAgentId(null);
  };

  const columns: CardListingTableColumn<BasicAgent>[] = useMemo(
    () => [
      {
        field: 'name',
        header: t('agents:listing.columns.name', 'Name'),
        sortValue: (row) => row.display ?? row.id,
        minWidth: 200,
        render: (row: BasicAgent): JSX.Element => (
          <ListingTable.CellIcon
            sx={{width: '100%'}}
            icon={<ResourceAvatar size={30} value={row.logoUrl} fallback={AgentConstants.DEFAULT_AVATAR} />}
            primary={row.display ?? row.id}
            secondary={row.description}
          />
        ),
      },
      {
        field: 'id',
        header: t('agents:listing.columns.agentId', 'Agent ID'),
        sortValue: (row) => row.id,
        minWidth: 200,
        render: (row: BasicAgent): JSX.Element => (
          <Typography variant="body2" sx={{fontFamily: 'monospace', fontSize: '0.875rem'}}>
            {row.id}
          </Typography>
        ),
      },
      {
        field: 'status',
        header: t('agents:listing.columns.status', 'Status'),
        minWidth: 110,
        render: (row: BasicAgent) => <AccessStatusIndicator status={row.status} category="agent" />,
      },
      {
        field: 'ouHandle',
        header: t('agents:listing.columns.organizationUnit', 'Organization unit'),
        sortValue: (row) => row.ouHandle ?? row.ouId,
        minWidth: 150,
        render: (row: BasicAgent): JSX.Element => (
          <Typography variant="body2" sx={{fontFamily: 'monospace', fontSize: '0.875rem'}}>
            {row.ouHandle ?? row.ouId ?? '-'}
          </Typography>
        ),
      },
      {
        field: 'actions',
        header: t('agents:listing.columns.actions', 'Actions'),
        width: 150,
        align: 'center',
        render: (row: BasicAgent): JSX.Element => (
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
              </>
            )}
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
        title={t('agents:listing.loadError', 'Failed to load agents')}
        onRetry={() => void refetch()}
      />
    );
  }

  return (
    <Box data-testid="agents-list">
      <CardListingTable
        rows={data?.agents ?? []}
        columns={columns}
        getRowId={(row) => row.id}
        onRowClick={(row) => {
          handleEditClick(row.id);
        }}
        loading={isLoading}
        pageSizeOptions={[5, 10, 25, 50]}
      />

      <AgentDeleteDialog open={deleteDialogOpen} agentId={selectedAgentId} onClose={handleDeleteDialogClose} />
    </Box>
  );
}
