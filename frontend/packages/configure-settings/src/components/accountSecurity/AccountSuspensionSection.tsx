// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {LIFECYCLE_FLOW_PURPOSES, QueryErrorNotice, SettingsCard} from '@thunderid/components';
import {Box, CircularProgress, Divider, Stack, Typography} from '@wso2/oxygen-ui';
import {useCallback, type JSX} from 'react';
import {Trans, useTranslation} from 'react-i18next';
import {Link} from 'react-router';
import useGetAccountLifecycleFlows from '../../api/useGetAccountLifecycleFlows';
import useSettingsRoutes from '../../hooks/useSettingsRoutes';

type LifecycleCategory = keyof typeof LIFECYCLE_FLOW_PURPOSES;
type LifecycleOperation = 'suspend' | 'unsuspend' | 'unlock';

/** The categories in the order the card lists them. */
const CATEGORIES: readonly LifecycleCategory[] = ['user', 'agent', 'application'];

/** The operations in the order each category lists them. A category without one skips it. */
const OPERATIONS: readonly LifecycleOperation[] = ['suspend', 'unsuspend', 'unlock'];

/** Fallback copy for the dynamic keys, matching en-US. */
const CATEGORY_FALLBACKS: Record<LifecycleCategory, string> = {
  user: 'Users',
  agent: 'Agents',
  application: 'Applications',
};
const OPERATION_FALLBACKS: Record<LifecycleOperation, string> = {
  suspend: 'Suspend flow',
  unsuspend: 'Unsuspend flow',
  unlock: 'Unlock flow',
};

/**
 * Where suspension is configured: the administration flow behind each lifecycle operation, linked
 * to the flow editor. Nothing is written here.
 *
 * Suspension is started from each user, agent or application page and carried out by these flows, so
 * an operator looking for it on this tab would otherwise conclude it is missing. A purpose with no
 * usable flow, unset or naming a flow that does not exist, says so and links to the flows list.
 *
 * @returns The rendered card
 *
 * @public
 */
export default function AccountSuspensionSection(): JSX.Element {
  const {t} = useTranslation();
  const routes = useSettingsRoutes();
  const {data, isLoading, error, refetch} = useGetAccountLifecycleFlows();

  const tForErrors = useCallback(
    (key: string, options?: Record<string, unknown>): string => t(key.includes(':') ? key : `settings:${key}`, options),
    [t],
  );

  let body: JSX.Element;
  if (isLoading) {
    body = <CircularProgress size={20} />;
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
    const flows: Record<string, string> = data ?? {};
    body = (
      <Stack spacing={2} divider={<Divider />}>
        {CATEGORIES.map((category) => {
          const title: string = t(`settings:accountSuspension.${category}.title`, CATEGORY_FALLBACKS[category]);
          return (
            <Box key={category} component="section" aria-label={title} data-testid={`account-suspension-${category}`}>
              <Typography variant="subtitle2" sx={{mb: 1}}>
                {title}
              </Typography>
              <Stack spacing={0.5}>
                {OPERATIONS.map((operation) => {
                  const purpose: string | undefined = LIFECYCLE_FLOW_PURPOSES[category][operation];
                  if (!purpose) {
                    return null;
                  }
                  const label: string = t(
                    `settings:accountSuspension.operation.${operation}`,
                    OPERATION_FALLBACKS[operation],
                  );
                  const flowId: string | undefined = flows[purpose];
                  return (
                    <Typography key={operation} variant="body2">
                      {flowId ? (
                        <Link to={routes.flows.detail(flowId)}>{label}</Link>
                      ) : (
                        <Trans
                          i18nKey="settings:accountSuspension.notConfigured"
                          defaults="{{operation}}: Not configured. <flowsLink>Open flows</flowsLink>"
                          values={{operation: label}}
                          components={{flowsLink: <Link to={routes.flows.list()} />}}
                        />
                      )}
                    </Typography>
                  );
                })}
              </Stack>
            </Box>
          );
        })}
      </Stack>
    );
  }

  return (
    <SettingsCard
      title={t('settings:accountSuspension.title', 'Account suspension')}
      description={t(
        'settings:accountSuspension.description',
        'Suspension ends existing sessions and tokens and blocks access until it is lifted. Start it, or unlock a locked account, from each user, agent or application page. These flows carry out each action.',
      )}
    >
      {body}
    </SettingsCard>
  );
}
