// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {QueryErrorNotice, SettingsCard} from '@thunderid/components';
import type {BasicFlowSummary} from '@thunderid/utils';
import {Chip, DataGrid, IconButton, ListingTable, Stack, Tooltip, Typography} from '@wso2/oxygen-ui';
import {Eye, Pencil} from '@wso2/oxygen-ui-icons-react';
import {useCallback, useMemo, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {Link, useNavigate} from 'react-router';
import useGetAccountLifecycleFlows from '../../api/useGetAccountLifecycleFlows';
import useSettingsRoutes from '../../hooks/useSettingsRoutes';

/** Fallback copy for the per-purpose keys, matching en-US. */
const PURPOSE_FALLBACKS: Record<string, string> = {
  userSuspendFlow: 'Suspend users',
  userUnsuspendFlow: 'Unsuspend users',
  userUnlockFlow: 'Unlock users',
  agentSuspendFlow: 'Suspend agents',
  agentUnsuspendFlow: 'Unsuspend agents',
  agentUnlockFlow: 'Unlock agents',
  applicationSuspendFlow: 'Suspend applications',
  applicationUnsuspendFlow: 'Unsuspend applications',
};

/**
 * Where suspension is configured: the administration flows behind the lifecycle operations, listed
 * as the Flows page lists them and opened in the flow editor. Nothing is written here.
 *
 * Suspension is started from each user, agent or application page and carried out by these flows, so
 * an operator looking for it on this tab would otherwise conclude it is missing. A purpose with no
 * usable flow, unset or naming a flow that does not exist, says so below the table with a link to the
 * flows list.
 *
 * @returns The rendered card
 *
 * @public
 */
export default function AccountSuspensionSection(): JSX.Element {
  const {t} = useTranslation();
  const navigate = useNavigate();
  const routes = useSettingsRoutes();
  const {data, isLoading, error, refetch} = useGetAccountLifecycleFlows();

  const tForErrors = useCallback(
    (key: string, options?: Record<string, unknown>): string => t(key.includes(':') ? key : `settings:${key}`, options),
    [t],
  );

  const handleOpenClick = useCallback(
    (flow: BasicFlowSummary): void => {
      void navigate(routes.flows.detail(flow.id));
    },
    [navigate, routes],
  );

  const columns: DataGrid.GridColDef<BasicFlowSummary>[] = useMemo(
    () => [
      {
        field: 'name',
        headerName: t('flows:listing.columns.name', 'Name'),
        flex: 1,
        minWidth: 220,
      },
      {
        field: 'flowType',
        headerName: t('flows:listing.columns.flowType', 'Type'),
        width: 180,
        renderCell: (params: DataGrid.GridRenderCellParams<BasicFlowSummary>): JSX.Element => (
          <Chip
            label={params.row.flowType}
            size="small"
            color="primary"
            variant="outlined"
            sx={{
              fontSize: '0.7rem',
            }}
          />
        ),
      },
      {
        field: 'updatedAt',
        headerName: t('flows:listing.columns.updatedAt', 'Last Updated'),
        width: 180,
        valueGetter: (_value, row): string => {
          const date = new Date(row.updatedAt);
          return date.toLocaleDateString(undefined, {
            year: 'numeric',
            month: 'short',
            day: 'numeric',
            hour: '2-digit',
            minute: '2-digit',
          });
        },
      },
      {
        field: 'actions',
        headerName: t('flows:listing.columns.actions', 'Actions'),
        width: 150,
        align: 'center',
        headerAlign: 'center',
        sortable: false,
        filterable: false,
        hideable: false,
        renderCell: (params: DataGrid.GridRenderCellParams<BasicFlowSummary>): JSX.Element => (
          <ListingTable.RowActions>
            {params.row.isReadOnly ? (
              <Tooltip title={t('common:actions.view', 'View')}>
                <IconButton
                  size="small"
                  onClick={(e) => {
                    e.stopPropagation();
                    handleOpenClick(params.row);
                  }}
                >
                  <Eye size={16} />
                </IconButton>
              </Tooltip>
            ) : (
              <Tooltip title={t('common:actions.edit', 'Edit')}>
                <IconButton
                  size="small"
                  onClick={(e) => {
                    e.stopPropagation();
                    handleOpenClick(params.row);
                  }}
                >
                  <Pencil size={16} />
                </IconButton>
              </Tooltip>
            )}
          </ListingTable.RowActions>
        ),
      },
    ],
    [handleOpenClick, t],
  );

  let body: JSX.Element;
  if (error) {
    body = (
      <QueryErrorNotice
        error={error}
        t={tForErrors}
        variant="inline"
        fallbackKey="settings:accountSuspension.load.error"
        fallbackDefaultValue="Failed to load the account lifecycle flows."
        onRetry={() => void refetch()}
      />
    );
  } else {
    const flows: BasicFlowSummary[] = data?.flows ?? [];
    const unconfigured: string[] = data?.unconfigured ?? [];
    body = (
      <Stack spacing={2}>
        {(isLoading || flows.length > 0) && (
          <ListingTable.Provider variant="data-grid-card" loading={isLoading}>
            <ListingTable.Container disablePaper>
              <ListingTable.DataGrid
                rows={flows}
                columns={columns}
                getRowId={(row): string => (row as BasicFlowSummary).id}
                onRowClick={(params) => {
                  handleOpenClick(params.row as BasicFlowSummary);
                }}
                // Eight lifecycle purposes cap the rows at eight, so no paging or column menu.
                hideFooter
                disableColumnMenu
                disableRowSelectionOnClick
                autoHeight
                sx={{
                  '& .MuiDataGrid-row': {
                    cursor: 'pointer',
                  },
                }}
              />
            </ListingTable.Container>
          </ListingTable.Provider>
        )}
        {unconfigured.length > 0 && (
          <Stack spacing={0.5} data-testid="account-suspension-unconfigured">
            {unconfigured.map((purpose) => (
              <Typography key={purpose} variant="body2">
                {t('settings:accountSuspension.notConfigured', '{{operation}}: Not configured.', {
                  operation: t(`settings:accountSuspension.purpose.${purpose}`, PURPOSE_FALLBACKS[purpose] ?? purpose),
                })}
              </Typography>
            ))}
            <Typography variant="body2">
              <Link to={routes.flows.list()}>{t('settings:accountSuspension.openFlows', 'Open flows')}</Link>
            </Typography>
          </Stack>
        )}
      </Stack>
    );
  }

  return (
    <SettingsCard
      title={t('settings:accountSuspension.title', 'Lock and suspension flows')}
      description={t(
        'settings:accountSuspension.description',
        'The flows that suspend, unsuspend and unlock users, agents and applications. Suspension ends existing sessions and tokens and blocks access until it is lifted.',
      )}
    >
      {body}
    </SettingsCard>
  );
}
