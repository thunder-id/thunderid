// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  LIFECYCLE_FLOW_PURPOSES,
  QueryErrorNotice,
  SettingsCard,
  getLifecycleFlowPurposes,
  useFlowListColumns,
  type LifecycleCategory,
} from '@thunderid/components';
import {useAdministration} from '@thunderid/contexts';
import {useDataGridLocaleText} from '@thunderid/hooks';
import type {AdministrationFlowConfigKeyValue, BasicFlowSummary} from '@thunderid/utils';
import {ListingTable, Stack, Typography} from '@wso2/oxygen-ui';
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
 * an operator looking for it on this tab would otherwise conclude it is missing. Only the operations
 * that run as flows are listed, as those are the ones the record pages offer. A purpose with no usable
 * flow, unset or naming a flow that does not exist, says so below the table with a link to the flows
 * list.
 *
 * @returns The rendered card
 *
 * @public
 */
export default function AccountSuspensionSection(): JSX.Element {
  const {t} = useTranslation();
  const navigate = useNavigate();
  const routes = useSettingsRoutes();
  const administration = useAdministration();
  const dataGridLocaleText = useDataGridLocaleText();

  const purposes: AdministrationFlowConfigKeyValue[] = useMemo(
    () =>
      (Object.keys(LIFECYCLE_FLOW_PURPOSES) as LifecycleCategory[]).flatMap((category) =>
        getLifecycleFlowPurposes(administration, category).map(([, purpose]) => purpose),
      ),
    [administration],
  );
  const {data, isLoading, error, refetch} = useGetAccountLifecycleFlows(purposes);

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

  const columns = useFlowListColumns<BasicFlowSummary>({onOpen: handleOpenClick});

  let body: JSX.Element;
  if (purposes.length === 0) {
    body = (
      <Typography variant="body2" color="text.secondary">
        {t(
          'settings:accountSuspension.noFlowOperations',
          'Suspend, unsuspend, and unlock do not run through flows in this console, so there are no flows to manage.',
        )}
      </Typography>
    );
  } else if (error) {
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
                localeText={dataGridLocaleText}
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
        'Manage the flows that suspend and unsuspend users, agents, and applications, and unlock users and agents. Suspension invalidates existing sessions and tokens and blocks access until it is lifted.',
      )}
    >
      {body}
    </SettingsCard>
  );
}
