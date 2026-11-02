// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {DocumentTitle} from '@thunderid/components';
import {useToast} from '@thunderid/contexts';
import {Alert, Box, CircularProgress, PageTitle, Stack} from '@wso2/oxygen-ui';
import {Fragment, useCallback, useMemo, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import useAppliedVersions from '../api/useAppliedVersions';
import useGateways from '../api/useGateways';
import {useVersions} from '../api/useVersions';
import ApplyPanel from '../components/ApplyPanel';
import FullScreenHeader from '../components/FullScreenHeader';
import GatewayStage from '../components/GatewayStage';
import PathConnector from '../components/PathConnector';
import RevertPanel from '../components/RevertPanel';
import VersionsCard from '../components/VersionsCard';
import useDeployRoutes from '../hooks/useDeployRoutes';
import type {ApplyResult, Version} from '../models/deploy';
import type {Gateway} from '../models/gateway';
import {versionLabel} from '../utils/format';

/** How many stages sit beside the versions card before the rest wrap to another row. */
const STAGES_PER_ROW = 3;

type Open = {kind: 'apply'; version: Version; gatewayId?: string} | {kind: 'revert'; gateway: Gateway} | null;

/**
 * The Deploy page: the captured versions and every gateway, with the way to deploy any version to
 * any gateway, to revert one, and to explore what one runs.
 */
export default function DeployPage(): JSX.Element {
  const {t} = useTranslation('deploy');
  const navigate = useNavigate();
  const routes = useDeployRoutes();
  const {showToast} = useToast();
  const gateways = useGateways();
  const versions = useVersions();
  const list: Gateway[] = useMemo(() => gateways.data ?? [], [gateways.data]);
  const applied = useAppliedVersions(list.map((gateway: Gateway) => gateway.id));
  const [open, setOpen] = useState<Open>(null);

  const all: Version[] = useMemo(() => versions.data ?? [], [versions.data]);
  // Where each version is deployed, by its hash, so the list names the gateways that run it.
  const runners = useMemo(() => {
    const byVersion: Record<string, string[]> = {};
    for (const gateway of list) {
      const hash = applied.byGateway[gateway.id]?.appliedVersion;
      if (hash) byVersion[hash] = [...(byVersion[hash] ?? []), gateway.name];
    }
    return byVersion;
  }, [list, applied.byGateway]);
  const labelOf = useCallback(
    (hash: string | undefined): string => {
      const index = all.findIndex((version: Version) => version.version === hash);
      return index < 0 ? '' : versionLabel(all[index].hash ?? hash, all[index].number ?? all.length - index);
    },
    [all],
  );
  const deployed = (gateway: Gateway, result: ApplyResult): void => {
    setOpen(null);
    const failed = result.import?.summary?.failed ?? 0;
    showToast(
      failed > 0
        ? t('applyPanel.doneWithFailures', 'Deployed to {{gateway}}; {{count}} resources were refused.', {
            gateway: gateway.name,
            count: failed,
          })
        : t('applyPanel.done', 'Deployed to {{gateway}}.', {gateway: gateway.name}),
      failed > 0 ? 'warning' : 'success',
    );
  };
  const reverted = (gateway: Gateway, result: ApplyResult): void => {
    setOpen(null);
    const failed = result.import?.summary?.failed ?? 0;
    showToast(
      failed > 0
        ? t(
            'revertPanel.doneWithFailures',
            '{{gateway}} runs its earlier version again; {{count}} resources were refused.',
            {
              gateway: gateway.name,
              count: failed,
            },
          )
        : t('revertPanel.done', '{{gateway}} runs its earlier version again.', {gateway: gateway.name}),
      failed > 0 ? 'warning' : 'success',
    );
  };

  // Redeploying deploys the version a gateway runs again, so the values set on it since take effect.
  const redeploy = (gateway: Gateway): void => {
    const running = all.find((version: Version) => version.version === applied.byGateway[gateway.id]?.appliedVersion);
    if (running) setOpen({kind: 'apply', version: running, gatewayId: gateway.id});
  };

  const stage = (gateway: Gateway): JSX.Element => (
    <Fragment key={gateway.id}>
      <PathConnector variant="deploy" testId={`deploy-connector-${gateway.id}`} />
      <GatewayStage
        gateway={gateway}
        applied={applied.byGateway[gateway.id]}
        versions={all}
        onExplore={() => void navigate(routes.deploy.explore(gateway.id))}
        onConfigure={() => void navigate(routes.deploy.values(gateway.id))}
        onRevert={() => setOpen({kind: 'revert', gateway})}
        onRedeploy={() => redeploy(gateway)}
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
                      onDeploy={(version: Version) => setOpen({kind: 'apply', version})}
                      deploying={open?.kind === 'apply' ? open.version.version : undefined}
                      runners={runners}
                      targets={list.map((gateway: Gateway) => gateway.name)}
                    />
                  ) : (
                    <Box />
                  )}
                  {row.map(stage)}
                </Box>
              ))
            )}
            {open?.kind === 'apply' && (
              <ApplyPanel
                version={open.version}
                label={labelOf(open.version.version)}
                gateways={list}
                gatewayId={open.gatewayId}
                onClose={() => setOpen(null)}
                onDeployed={deployed}
              />
            )}
            {open?.kind === 'revert' && (
              <RevertPanel
                gateway={open.gateway}
                previousLabel={labelOf(applied.byGateway[open.gateway.id]?.previousVersion) || undefined}
                onClose={() => setOpen(null)}
                onReverted={reverted}
              />
            )}
          </Stack>
        )}
      </Box>
    </Box>
  );
}
