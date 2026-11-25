// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {QueryErrorNotice, ResourceAvatar, SettingsCard} from '@thunderid/components';
import {useDataGridLocaleText} from '@thunderid/hooks';
import {useLogger} from '@thunderid/logger/react';
import {Box, Chip, DataGrid, IconButton} from '@wso2/oxygen-ui';
import {Pencil} from '@wso2/oxygen-ui-icons-react';
import {useCallback, useMemo, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import useGetUserOwnedAgents from '../../api/useGetUserOwnedAgents';
import UserConstants from '../../constants/user-constants';
import {useUserAgentRoutes} from '../../hooks/useUserRoutes';
import type {OwnedAgent} from '../../models/users';
import getUserErrorMessage from '../../utils/getUserErrorMessage';

interface OwnedAgentsSectionProps {
  userId: string;
}

/**
 * Lists the agents that name the user as their owner, in the style of a group's Members table. Each row
 * opens the agent's page.
 */
export default function OwnedAgentsSection({userId}: OwnedAgentsSectionProps): JSX.Element {
  const {t} = useTranslation();
  const logger = useLogger('OwnedAgentsSection');
  const navigate = useNavigate();
  const agentRoutes = useUserAgentRoutes();
  const dataGridLocaleText = useDataGridLocaleText();
  const {data, isLoading, error, refetch} = useGetUserOwnedAgents(userId);

  const handleOpenClick = useCallback(
    (agentId: string): void => {
      (async (): Promise<void> => {
        await navigate(agentRoutes.detail(agentId));
      })().catch((_error: unknown) => {
        logger.error('Failed to navigate to agent', {error: _error, agentId});
      });
    },
    [agentRoutes, logger, navigate],
  );

  const columns: DataGrid.GridColDef<OwnedAgent>[] = useMemo(
    () => [
      {
        field: 'avatar',
        headerName: '',
        width: 70,
        sortable: false,
        filterable: false,
        renderCell: (params: DataGrid.GridRenderCellParams<OwnedAgent>): JSX.Element => (
          <Box sx={{display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100%'}}>
            <ResourceAvatar size={30} value={params.row.logoUrl} fallback={UserConstants.DEFAULT_AGENT_AVATAR} />
          </Box>
        ),
      },
      {
        field: 'display',
        headerName: t('users:manageUser.sections.ownedAgents.columns.name', 'Name'),
        flex: 1,
        minWidth: 200,
        valueGetter: (_value: unknown, row: OwnedAgent) => row.display ?? row.id,
      },
      {
        field: 'id',
        headerName: t('users:manageUser.sections.ownedAgents.columns.agentId', 'Agent ID'),
        flex: 1,
        minWidth: 250,
      },
      {
        field: 'kind',
        headerName: t('users:manageUser.sections.ownedAgents.columns.kind', 'Kind'),
        flex: 0.6,
        minWidth: 120,
        // The column menu filters on this value, so a filter for "Personal" lists the personal agents.
        valueGetter: (_value: unknown, row: OwnedAgent) =>
          row.kind === UserConstants.PERSONAL_AGENT_KIND
            ? t('users:manageUser.sections.ownedAgents.personal', 'Personal')
            : '',
        renderCell: (params: DataGrid.GridRenderCellParams<OwnedAgent>): JSX.Element | null =>
          params.row.kind === UserConstants.PERSONAL_AGENT_KIND ? (
            <Chip
              size="small"
              color="primary"
              variant="outlined"
              label={t('users:manageUser.sections.ownedAgents.personal', 'Personal')}
            />
          ) : null,
      },
      {
        field: 'actions',
        headerName: '',
        width: 60,
        sortable: false,
        filterable: false,
        renderCell: (params: DataGrid.GridRenderCellParams<OwnedAgent>): JSX.Element => (
          <IconButton
            size="small"
            aria-label={t('common:actions.edit', 'Edit')}
            onClick={(e) => {
              e.stopPropagation();
              handleOpenClick(params.row.id);
            }}
          >
            <Pencil size={14} />
          </IconButton>
        ),
      },
    ],
    [handleOpenClick, t],
  );

  const title = t('users:manageUser.sections.ownedAgents.title', 'Owned agents');
  const description = t(
    'users:manageUser.sections.ownedAgents.description',
    'Agents that name this user as their owner.',
  );

  if (error) {
    return (
      <SettingsCard title={title} description={description}>
        <QueryErrorNotice
          error={error}
          t={(key, options) => t(key.includes(':') ? key : `users:${key}`, options)}
          resolveErrorMessage={getUserErrorMessage}
          variant="inline"
          onRetry={() => void refetch()}
          fallbackKey="users:manageUser.sections.ownedAgents.error"
          fallbackDefaultValue="Failed to load the agents this user owns"
        />
      </SettingsCard>
    );
  }

  return (
    <SettingsCard title={title} description={description} slotProps={{content: {sx: {p: 0}}}}>
      <Box sx={{height: 400, width: '100%'}}>
        <DataGrid.DataGrid
          rows={data?.agents ?? []}
          columns={columns}
          loading={isLoading}
          getRowId={(row): string => row.id}
          onRowClick={(params) => handleOpenClick((params.row as OwnedAgent).id)}
          initialState={{pagination: {paginationModel: {pageSize: 10}}}}
          pageSizeOptions={[5, 10, 25]}
          disableRowSelectionOnClick
          localeText={{
            ...dataGridLocaleText,
            noRowsLabel: t('users:manageUser.sections.ownedAgents.empty', "This user doesn't own any agents."),
          }}
          sx={{'& .MuiDataGrid-row': {cursor: 'pointer'}}}
        />
      </Box>
    </SettingsCard>
  );
}
