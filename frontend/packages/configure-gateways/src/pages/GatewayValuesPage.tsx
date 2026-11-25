// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {DocumentTitle} from '@thunderid/components';
import {useHiddenResources} from '@thunderid/contexts';
import {Alert, Box, CircularProgress, PageTitle, Stack, Tab, Tabs} from '@wso2/oxygen-ui';
import {useMemo, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate, useParams} from 'react-router';
import useGateways from '../api/useGateways';
import {useRunningReferences} from '../api/useGatewayVersions';
import {useStoredValueCount, useStoredValuesNamed} from '../api/useStoredValues';
import {useVersion} from '../api/useVersions';
import DeployCard from '../components/DeployCard';
import FullScreenHeader from '../components/FullScreenHeader';
import StoredValuesList from '../components/StoredValuesList';
import ValuesTable from '../components/ValuesTable';
import useDeployRoutes from '../hooks/useDeployRoutes';
import type {ValueKind} from '../models/deploy';
import type {Gateway} from '../models/gateway';
import {missingRowsOf, referencedNames} from '../utils/valueRows';

/**
 * The variables and secrets one gateway holds, with what the version it runs refers to each for, and
 * the way to set, replace or remove each. Values the version needs and the gateway lacks come first,
 * found by looking up only the names it refers to; the rest are read a page at a time.
 */
export default function GatewayValuesPage(): JSX.Element {
  const {t} = useTranslation('deploy');
  const navigate = useNavigate();
  const routes = useDeployRoutes();
  const {gatewayId = ''} = useParams<{gatewayId: string}>();
  const [tab, setTab] = useState<ValueKind>('variable');
  const gateways = useGateways();
  const gateway = gateways.data?.find((entry: Gateway) => entry.id === gatewayId);
  const running = useRunningReferences(gatewayId);
  // The configuration as it stands names the resource a value belongs to when the version this gateway runs
  // does not refer to it. It only labels rows, so it neither holds the page back nor fails it.
  const current = useVersion('current');
  const references = useMemo(() => running.data ?? [], [running.data]);
  const currentReferences = useMemo(() => current.data?.references ?? [], [current.data]);
  // Only the names the running version refers to are looked up, to find the ones the gateway lacks.
  const variableNames = useMemo(() => referencedNames(references, 'variable'), [references]);
  const secretNames = useMemo(() => referencedNames(references, 'secret'), [references]);
  const variables = useStoredValuesNamed(gatewayId, 'variable', variableNames, running.isSuccess);
  const secrets = useStoredValuesNamed(gatewayId, 'secret', secretNames, running.isSuccess);
  const variableCount = useStoredValueCount(gatewayId, 'variable');
  const secretCount = useStoredValueCount(gatewayId, 'secret');
  // The values of hidden resources are left out, but rows are read with them so they do not show as unused.
  const {isHiddenResource} = useHiddenResources();
  const missingVariables = useMemo(
    () => missingRowsOf('variable', variables.data ?? [], references, isHiddenResource),
    [variables.data, references, isHiddenResource],
  );
  const missingSecrets = useMemo(
    () => missingRowsOf('secret', secrets.data ?? [], references, isHiddenResource),
    [secrets.data, references, isHiddenResource],
  );
  const missing = missingVariables.length + missingSecrets.length;
  const missingRows = tab === 'secret' ? missingSecrets : missingVariables;
  const back = (): void => void navigate(routes.deploy.list());

  let body: JSX.Element;
  if (gateways.isLoading || running.isLoading || variables.isLoading || secrets.isLoading) {
    body = (
      <Box sx={{display: 'flex', justifyContent: 'center', py: 8}}>
        <CircularProgress size={28} />
      </Box>
    );
  } else if (!gateway) {
    body = <Alert severity="error">{t('explorer.noGateway', 'No gateway is registered under this id.')}</Alert>;
  } else if (running.isError) {
    body = (
      <Alert severity="error">
        {t('values.referencesFailed', 'Which values the version {{gateway}} runs refers to could not be read.', {
          gateway: gateway.name,
        })}
      </Alert>
    );
  } else if (variables.isError || secrets.isError) {
    body = (
      <Alert severity="error">
        {t('values.readFailed', 'The values {{gateway}} holds could not be read.', {gateway: gateway.name})}
      </Alert>
    );
  } else {
    body = (
      <Stack spacing={3}>
        {missing > 0 && (
          <Alert severity="warning">
            {t('values.missing', 'The version {{gateway}} runs refers to {{count}} values it does not hold.', {
              gateway: gateway.name,
              count: missing,
            })}
          </Alert>
        )}
        <DeployCard
          header={
            <Tabs value={tab} onChange={(_event, value: ValueKind) => setTab(value)} sx={{px: 2}}>
              <Tab
                value="variable"
                label={
                  variableCount.data === undefined
                    ? t('values.variablesTabPlain', 'Variables')
                    : t('values.variablesTab', 'Variables ({{count}})', {count: variableCount.data})
                }
                sx={{textTransform: 'none'}}
              />
              <Tab
                value="secret"
                label={
                  secretCount.data === undefined
                    ? t('values.secretsTabPlain', 'Secrets')
                    : t('values.secretsTab', 'Secrets ({{count}})', {count: secretCount.data})
                }
                sx={{textTransform: 'none'}}
              />
            </Tabs>
          }
        >
          <Stack spacing={2} sx={{px: 2, pb: 2}}>
            {missingRows.length > 0 && (
              <ValuesTable key={`missing:${gatewayId}:${tab}`} gatewayId={gatewayId} kind={tab} rows={missingRows} />
            )}
            <StoredValuesList
              key={`${gatewayId}:${tab}`}
              gatewayId={gatewayId}
              kind={tab}
              references={references}
              current={currentReferences}
            />
          </Stack>
        </DeployCard>
      </Stack>
    );
  }
  return (
    <Box sx={{minHeight: '100vh', display: 'flex', flexDirection: 'column'}}>
      <DocumentTitle
        title={
          gateway ? t('values.title', 'Configure {{gateway}}', {gateway: gateway.name}) : t('header.title', 'Deploy')
        }
      />
      <FullScreenHeader
        breadcrumbs={[
          {key: 'deploy', label: t('header.title', 'Deploy'), onClick: back},
          {key: 'gateway', label: gateway?.name ?? gatewayId},
        ]}
        onExit={back}
      />
      <Box component="main" sx={{flex: 1, px: {xs: 4, md: 10}, pt: 8, pb: 6, width: '100%'}}>
        <PageTitle>
          <PageTitle.Header>
            {t('values.title', 'Configure {{gateway}}', {gateway: gateway?.name ?? ''})}
          </PageTitle.Header>
          <PageTitle.SubHeader>
            {t(
              'values.description',
              'The variables and secrets this gateway holds. The configuration refers to them by name; each gateway holds its own.',
            )}
          </PageTitle.SubHeader>
        </PageTitle>
        {body}
      </Box>
    </Box>
  );
}
