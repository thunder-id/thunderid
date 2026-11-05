// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {DocumentTitle} from '@thunderid/components';
import {Alert, Box, CircularProgress, PageTitle, Stack} from '@wso2/oxygen-ui';
import {Fragment, useMemo, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import useAppliedVersions from '../api/useAppliedVersions';
import useGateways from '../api/useGateways';
import {useVersions} from '../api/useVersions';
import FullScreenHeader from '../components/FullScreenHeader';
import GatewayStage from '../components/GatewayStage';
import PathConnector from '../components/PathConnector';
import VersionsCard from '../components/VersionsCard';
import useDeployPanels from '../hooks/useDeployPanels';
import useDeployRoutes from '../hooks/useDeployRoutes';
import type {Version} from '../models/deploy';
import type {Gateway} from '../models/gateway';
import runnersOf from '../utils/runners';

/** How many stages sit beside the versions card before the rest wrap to another row. */
const STAGES_PER_ROW = 3;

/**
 * The Deploy page: the captured versions and every gateway, with the way to deploy any version to
 * any gateway, to revert one, and to explore what one runs.
 */
export default function DeployPage(): JSX.Element {
  const {t} = useTranslation('deploy');
  const navigate = useNavigate();
  const routes = useDeployRoutes();
  const gateways = useGateways();
  const versions = useVersions();
  const list: Gateway[] = useMemo(() => gateways.data ?? [], [gateways.data]);
  const applied = useAppliedVersions(list.map((gateway: Gateway) => gateway.id));

  const all: Version[] = useMemo(() => versions.data ?? [], [versions.data]);
  // Where each version is deployed, so the list names the gateways that run it.
  const runners = useMemo(() => runnersOf(list, applied.byGateway), [list, applied.byGateway]);
  const panels = useDeployPanels(list, all, applied.byGateway);

  const stage = (gateway: Gateway): JSX.Element => (
    <Fragment key={gateway.id}>
      <PathConnector variant="deploy" testId={`deploy-connector-${gateway.id}`} />
      <GatewayStage
        gateway={gateway}
        applied={applied.byGateway[gateway.id]}
        versions={all}
        onExplore={() => void navigate(routes.deploy.explore(gateway.id))}
        onConfigure={() => void navigate(routes.deploy.values(gateway.id))}
        onRevert={() => panels.revert(gateway)}
        onRedeploy={() => panels.redeploy(gateway)}
        onRetryRefused={(keys: string[]) => panels.retryRefused(gateway, keys)}
      />
    </Fragment>
  );
  const rows: Gateway[][] = [];
  for (let start = 0; start < list.length; start += STAGES_PER_ROW)
    rows.push(list.slice(start, start + STAGES_PER_ROW));
  const columns = (count: number): string => `minmax(0, 1.05fr)${' auto minmax(0, 1fr)'.repeat(count)}`;

  return (
    <Box sx={{minHeight: '100vh', display: 'flex', flexDirection: 'column'}}>
      <DocumentTitle title={t('page.title', 'Deploy')} />
      <FullScreenHeader
        breadcrumbs={[{key: 'deploy', label: t('header.title', 'Deploy')}]}
        onExit={() => void navigate(routes.home.list())}
      />
      <Box component="main" sx={{flex: 1, px: {xs: 4, md: 10}, pt: 8, pb: 6, width: '100%'}}>
        <PageTitle>
          <PageTitle.Header>{t('page.title', 'Deploy')}</PageTitle.Header>
          <PageTitle.SubHeader>
            {t(
              'page.description',
              'Deploy any version of the configuration to any gateway. The newest is kept up to date as the configuration changes.',
            )}
          </PageTitle.SubHeader>
        </PageTitle>
        {gateways.isError && (
          <Alert severity="error">{t('page.gatewaysFailed', 'The gateways could not be read.')}</Alert>
        )}
        {versions.isError && (
          <Alert severity="error" sx={{mb: 3}}>
            {t('page.versionsFailed', 'The configuration versions could not be read.')}
          </Alert>
        )}
        {gateways.isLoading && (
          <Box sx={{display: 'flex', justifyContent: 'center', py: 8}}>
            <CircularProgress size={28} />
          </Box>
        )}
        {gateways.isSuccess && (
          <Stack spacing={3}>
            {rows.length === 0 ? (
              <Box sx={{display: 'grid', gridTemplateColumns: columns(0), gap: 0, maxWidth: 480}}>
                <VersionsCard versions={all} isLoading={versions.isLoading} />
                <Alert severity="info" sx={{mt: 2}}>
                  {t('page.noGateways', 'Register a gateway to deploy a version to it.')}
                </Alert>
              </Box>
            ) : (
              rows.map((row: Gateway[], rowIndex: number) => (
                <Box
                  key={row[0].id}
                  sx={{display: 'grid', gridTemplateColumns: columns(row.length), alignItems: 'stretch'}}
                >
                  {rowIndex === 0 ? (
                    <VersionsCard
                      versions={all}
                      isLoading={versions.isLoading}
                      onDeploy={(version: Version) => panels.deploy(version)}
                      deploying={panels.deploying}
                      runners={runners.running}
                      partlyRunners={runners.partly}
                      targets={list.map((gateway: Gateway) => gateway.name)}
                    />
                  ) : (
                    <Box />
                  )}
                  {row.map(stage)}
                </Box>
              ))
            )}
            {panels.panels}
          </Stack>
        )}
      </Box>
    </Box>
  );
}
