// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  AccessStatusIndicator,
  EditOrViewAction,
  QueryErrorNotice,
  ResourceAvatar,
  ResourceDeployStatus,
  useDeployStatusSource,
} from '@thunderid/components';
import {useGetAllAgentTypes} from '@thunderid/configure-agent-types';
import {useCapabilitiesContext, useProjectScope} from '@thunderid/contexts';
import {useDataGridLocaleText, useResolveDisplayName} from '@thunderid/hooks';
import {useLogger} from '@thunderid/logger/react';
import {Box, Chip, IconButton, Stack, Tab, Tabs, Tooltip, Typography, ListingTable, DataGrid} from '@wso2/oxygen-ui';
import {Eye, Trash2} from '@wso2/oxygen-ui-icons-react';
import {useMemo, useCallback, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import AgentDeleteDialog from './AgentDeleteDialog';
import useGetAgents from '../api/useGetAgents';
import AgentConstants from '../constants/agent-constants';
import useAgentRoutes from '../hooks/useAgentRoutes';
import useLifespanEndText from '../hooks/useLifespanEndText';
import {PERSONAL_AGENT_KIND, type BasicAgent} from '../models/agent';

/**
 * Which agents the list shows: every agent, or only personal agents.
 */
type AgentsListView = 'all' | 'personal';

export default function AgentsList(): JSX.Element {
  const routes = useAgentRoutes();
  const navigate = useNavigate();
  const {t} = useTranslation();
  const {can} = useCapabilitiesContext();
  const logger = useLogger('AgentsList');
  const dataGridLocaleText = useDataGridLocaleText();
  const lifespanEndText = useLifespanEndText();
  const [view, setView] = useState<AgentsListView>('all');
  // A project's agents list filters only by organization unit, and personal agents are registered on a data
  // plane rather than in a project, so the personal view is offered only outside a project.
  const {projectId} = useProjectScope();
  const showsPersonalView = !projectId;
  const {data, isLoading, error, refetch} = useGetAgents(
    showsPersonalView && view === 'personal' ? {kind: PERSONAL_AGENT_KIND} : undefined,
  );
  const {data: agentTypes} = useGetAllAgentTypes();
  // A stable handlers object keeps resolveDisplayName, and with it the grid's columns, stable.
  const displayNameHandlers = useMemo(() => ({t}), [t]);
  const {resolveDisplayName} = useResolveDisplayName({handlers: displayNameHandlers});

  // An agent names its type by handle. The type's display name is shown when the types have loaded,
  // and the handle otherwise.
  const agentTypeNames = useMemo(
    () => new Map((agentTypes ?? []).map((type) => [type.handle, resolveDisplayName(type.displayName) || type.handle])),
    [agentTypes, resolveDisplayName],
  );

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

  // When the lifespan ends, or that it has no end, says more than the status's own text, for an agent
  // that can still authenticate and for one that expired. A suspension or a lock keeps its own text.
  const lifespanHint = useCallback(
    (agent: BasicAgent): string | undefined => {
      if (agent.status && agent.status.value !== 'ACTIVE' && agent.status.value !== 'EXPIRED') return undefined;
      if (agent.lifespan?.phase === 'UNBOUNDED') return t('agents:lifespan.noExpiry', 'No expiry date');

      return lifespanEndText(agent.validUntil ?? agent.status?.details?.since);
    },
    [lifespanEndText, t],
  );

  const deploys = useDeployStatusSource() !== null;
  const columns: DataGrid.GridColDef<BasicAgent>[] = useMemo(
    () => [
      {
        field: 'name',
        headerName: t('agents:listing.columns.name', 'Name'),
        flex: 1,
        minWidth: 200,
        renderCell: (params: DataGrid.GridRenderCellParams<BasicAgent>): JSX.Element => (
          <ListingTable.CellIcon
            sx={{width: '100%'}}
            icon={<ResourceAvatar size={30} value={params.row.logoUrl} fallback={AgentConstants.DEFAULT_AVATAR} />}
            primary={
              params.row.kind === PERSONAL_AGENT_KIND ? (
                <Stack direction="row" alignItems="center" spacing={1} component="span">
                  <span>{params.row.display ?? params.row.id}</span>
                  <Chip size="small" color="primary" variant="outlined" label={t('agents:kind.personal', 'Personal')} />
                </Stack>
              ) : (
                (params.row.display ?? params.row.id)
              )
            }
            secondary={params.row.description}
          />
        ),
      },
      {
        field: 'id',
        headerName: t('agents:listing.columns.agentId', 'Agent ID'),
        flex: 1,
        minWidth: 200,
        renderCell: (params: DataGrid.GridRenderCellParams<BasicAgent>): JSX.Element => (
          <Typography variant="body2" sx={{fontFamily: 'monospace', fontSize: '0.875rem'}}>
            {params.row.id}
          </Typography>
        ),
      },
      {
        field: 'status',
        headerName: t('agents:listing.columns.status', 'Status'),
        flex: 0.4,
        minWidth: 110,
        sortable: false,
        renderCell: (params: DataGrid.GridRenderCellParams<BasicAgent>) => (
          <AccessStatusIndicator
            status={params.row.status}
            category="agent"
            hint={lifespanHint(params.row)}
            tabIndex={params.tabIndex}
          />
        ),
      },
      {
        field: 'type',
        headerName: t('agents:listing.columns.agentType', 'Agent type'),
        flex: 0.5,
        minWidth: 150,
        valueGetter: (_value, row): string => agentTypeNames.get(row.type) ?? row.type,
        renderCell: (params: DataGrid.GridRenderCellParams<BasicAgent>): JSX.Element => (
          <Typography variant="body2">{agentTypeNames.get(params.row.type) ?? params.row.type}</Typography>
        ),
      },
      {
        field: 'ouHandle',
        headerName: t('agents:listing.columns.organizationUnit', 'Organization unit'),
        flex: 0.5,
        minWidth: 150,
        renderCell: (params: DataGrid.GridRenderCellParams<BasicAgent>): JSX.Element => (
          <Typography variant="body2" sx={{fontFamily: 'monospace', fontSize: '0.875rem'}}>
            {params.row.ouHandle ?? params.row.ouId ?? '-'}
          </Typography>
        ),
      },
      // Where each agent is along the deploy path is shown only where the app says so.
      ...(deploys
        ? [
            {
              field: 'deployment',
              headerName: t('agents:listing.columns.deployment', 'Deployment'),
              flex: 0.6,
              minWidth: 150,
              sortable: false,
              renderCell: (params: DataGrid.GridRenderCellParams<BasicAgent>): JSX.Element => (
                <ResourceDeployStatus resourceType="agent" id={params.row.id} tabIndex={params.tabIndex} />
              ),
            } satisfies DataGrid.GridColDef<BasicAgent>,
          ]
        : []),
      {
        field: 'actions',
        headerName: t('agents:listing.columns.actions', 'Actions'),
        width: 150,
        align: 'center',
        headerAlign: 'center',
        sortable: false,
        filterable: false,
        hideable: false,
        renderCell: (params: DataGrid.GridRenderCellParams<BasicAgent>): JSX.Element => (
          <ListingTable.RowActions>
            {params.row.isReadOnly || params.row.isOwnedByOU === false ? (
              <Tooltip title={t('common:status.readOnly', 'Read only')}>
                <IconButton
                  aria-label={t('common:status.readOnly', 'Read only')}
                  size="small"
                  disableRipple
                  sx={{cursor: 'default'}}
                >
                  <Eye size={16} />
                </IconButton>
              </Tooltip>
            ) : (
              <>
                <EditOrViewAction canEdit={can('agent', 'update')} onClick={() => handleEditClick(params.row.id)} />
                {can('agent', 'delete') && (
                  <Tooltip title={t('common:actions.delete', 'Delete')}>
                    <IconButton
                      aria-label={t('common:actions.delete', 'Delete')}
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
                )}
              </>
            )}
          </ListingTable.RowActions>
        ),
      },
    ],
    [agentTypeNames, can, deploys, handleDeleteClick, handleEditClick, lifespanHint, t],
  );

  // Kept when the list fails to load, so a failing view can be switched away from.
  const viewTabs = showsPersonalView && (
    <Tabs
      value={view}
      aria-label={t('agents:listing.view.label', 'Agents to show')}
      onChange={(_event, next: AgentsListView) => setView(next)}
      sx={{mb: 2}}
    >
      <Tab value="all" label={t('agents:listing.view.all', 'All')} />
      <Tab value="personal" label={t('agents:listing.view.personal', 'Personal agents')} />
    </Tabs>
  );

  if (error) {
    return (
      <>
        {viewTabs}
        <QueryErrorNotice
          error={error}
          t={tForErrors}
          variant="block"
          title={t('agents:listing.loadError', 'Failed to load agents')}
          onRetry={() => void refetch()}
        />
      </>
    );
  }

  return (
    <Box data-testid="agents-list">
      {viewTabs}
      <ListingTable.Provider variant="data-grid-card" loading={isLoading}>
        <ListingTable.Container disablePaper>
          <ListingTable.DataGrid
            rows={data?.agents ?? []}
            columns={columns}
            getRowId={(row): string => (row as BasicAgent).id}
            onRowClick={(params) => {
              handleEditClick((params.row as BasicAgent).id);
            }}
            initialState={{
              pagination: {paginationModel: {pageSize: 10}},
            }}
            pageSizeOptions={[5, 10, 25, 50]}
            disableRowSelectionOnClick
            // Filtering is not wired end to end, so the column filter panel stays hidden.
            disableColumnFilter
            localeText={dataGridLocaleText}
            sx={{
              height: 'auto',
              '& .MuiDataGrid-row': {cursor: 'pointer'},
            }}
          />
        </ListingTable.Container>
      </ListingTable.Provider>

      <AgentDeleteDialog open={deleteDialogOpen} agentId={selectedAgentId} onClose={handleDeleteDialogClose} />
    </Box>
  );
}
