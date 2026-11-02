// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {DocumentTitle} from '@thunderid/components';
import {Alert, Box, CircularProgress, PageTitle, Stack, Tab, Tabs} from '@wso2/oxygen-ui';
import {useMemo, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate, useParams} from 'react-router';
import useGateways from '../api/useGateways';
import {useRunningReferences} from '../api/useGatewayVersions';
import {useStoredValues} from '../api/useStoredValues';
import DeployCard from '../components/DeployCard';
import FullScreenHeader from '../components/FullScreenHeader';
import ValuesTable, {type ValueRow} from '../components/ValuesTable';
import useDeployRoutes from '../hooks/useDeployRoutes';
import type {StoredValue, ValueKind, ValueReference} from '../models/deploy';
import type {Gateway} from '../models/gateway';

/** Every name of one kind the gateway holds or the version it runs refers to, named rows first. */
function rowsOf(kind: ValueKind, stored: StoredValue[], references: ValueReference[]): ValueRow[] {
  const byName = new Map<string, ValueRow>();
  references
    .filter((reference: ValueReference) => reference.kind === kind)
    .forEach((reference: ValueReference) => {
      const row = byName.get(reference.name) ?? {name: reference.name, references: []};
      row.references.push(reference);
      byName.set(reference.name, row);
    });
  stored.forEach((value: StoredValue) => {
    const row = byName.get(value.name) ?? {name: value.name, references: []};
    row.stored = value;
    byName.set(value.name, row);
  });
  return [...byName.values()].sort((a: ValueRow, b: ValueRow) => {
    const unset = (row: ValueRow): number => (row.stored ? 1 : 0);
    return unset(a) - unset(b) || a.name.localeCompare(b.name);
  });
}

/**
 * The variables and secrets one gateway holds, with what the version it runs refers to each for, and
 * the way to set, replace or remove each. Values the version needs and the gateway lacks come first.
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
  const variables = useStoredValues(gatewayId, 'variable');
  const secrets = useStoredValues(gatewayId, 'secret');
  const references = useMemo(() => running.data ?? [], [running.data]);
  const variableRows = useMemo(
    () => rowsOf('variable', variables.data ?? [], references),
    [variables.data, references],
  );
  const secretRows = useMemo(() => rowsOf('secret', secrets.data ?? [], references), [secrets.data, references]);
  const missing = [...variableRows, ...secretRows].filter(
    (row: ValueRow) => !row.stored && row.references.length > 0,
  ).length;
  const back = (): void => void navigate(routes.deploy.list());

  let body: JSX.Element;
  if (gateways.isLoading || variables.isLoading || secrets.isLoading || running.isLoading) {
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
                label={t('values.variablesTab', 'Variables ({{count}})', {count: variableRows.length})}
                sx={{textTransform: 'none'}}
              />
              <Tab
                value="secret"
                label={t('values.secretsTab', 'Secrets ({{count}})', {count: secretRows.length})}
                sx={{textTransform: 'none'}}
              />
            </Tabs>
          }
        >
          <Box sx={{px: 2, pb: 2}}>
            <ValuesTable
              key={`${gatewayId}:${tab}`}
              gatewayId={gatewayId}
              kind={tab}
              rows={tab === 'secret' ? secretRows : variableRows}
            />
          </Box>
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
