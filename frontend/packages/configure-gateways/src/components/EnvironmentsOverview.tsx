// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Alert, Box, CircularProgress, Stack, Typography} from '@wso2/oxygen-ui';
import {Fragment, useMemo, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import AutoApplySwitch from './AutoApplySwitch';
import GatewayStage from './GatewayStage';
import PathConnector from './PathConnector';
import VersionsCard from './VersionsCard';
import useAppliedVersions from '../api/useAppliedVersions';
import {usePipeline} from '../api/usePipeline';
import {useVersions} from '../api/useVersions';
import useDeployPanels from '../hooks/useDeployPanels';
import useDeployRoutes from '../hooks/useDeployRoutes';
import type {Version} from '../models/deploy';
import type {Gateway} from '../models/gateway';
import type {PipelineStage} from '../models/pipeline';
import runnersOf from '../utils/runners';

/** How many environments sit beside the versions card before the rest wrap to another row. */
const STAGES_PER_ROW = 3;

export interface EnvironmentsOverviewProps {
  /** Opens an environment's own page; its route otherwise. */
  onOpenEnvironment?: (gatewayId: string) => void;
}

/**
 * The environments configuration is deployed and promoted through, in order, beside the versions it is
 * captured as: a deploy enters at the first environment, and each later one is promoted to from the one
 * before it. Each environment links to its own console, and the first says whether saved changes reach
 * it on their own.
 */
export default function EnvironmentsOverview({onOpenEnvironment = undefined}: EnvironmentsOverviewProps): JSX.Element {
  const {t} = useTranslation('deploy');
  const navigate = useNavigate();
  const routes = useDeployRoutes();
  const pipeline = usePipeline();
  const versions = useVersions();
  const stages: PipelineStage[] = useMemo(() => pipeline.data?.stages ?? [], [pipeline.data]);
  const gateways: Gateway[] = useMemo(() => stages.map((stage: PipelineStage) => stage.gateway), [stages]);
  const applied = useAppliedVersions(gateways.map((gateway: Gateway) => gateway.id));
  const all: Version[] = useMemo(() => versions.data ?? [], [versions.data]);
  const panels = useDeployPanels(gateways, all, applied.byGateway, stages);
  const names = useMemo(
    () => Object.fromEntries(stages.map((stage: PipelineStage) => [stage.gateway.id, stage.name])),
    [stages],
  );
  const runners = useMemo(
    () => runnersOf(gateways, applied.byGateway, (gateway: Gateway) => names[gateway.id] ?? gateway.name),
    [gateways, applied.byGateway, names],
  );
  const first = stages[0];

  // Promoting takes the version the environment before runs to this one. There is nothing to promote
  // while that one runs nothing, or while this one already runs all of it.
  const promotable = (index: number): Version | undefined => {
    const before = applied.byGateway[stages[index - 1].gateway.id];
    const here = applied.byGateway[stages[index].gateway.id];
    if (!before?.appliedVersion) return undefined;
    if (here?.appliedVersion === before.appliedVersion && here.complete !== false) return undefined;
    return all.find((version: Version) => version.version === before.appliedVersion);
  };

  const stage = (entry: PipelineStage, index: number): JSX.Element => {
    const {gateway} = entry;
    const version = index === 0 ? undefined : promotable(index);
    return (
      <Fragment key={gateway.id}>
        {index === 0 ? (
          <PathConnector variant="deploy" testId={`environment-connector-${gateway.id}`} />
        ) : (
          <PathConnector
            variant="promote"
            label={t('environments.promote', 'Promote')}
            disabled={!version}
            onClick={() => version && panels.deploy(version, gateway.id, undefined, [gateway])}
            testId={`environment-promote-${gateway.id}`}
          />
        )}
        <GatewayStage
          gateway={gateway}
          name={entry.name}
          adminConsoleUrl={entry.adminConsoleUrl}
          applied={applied.byGateway[gateway.id]}
          versions={all}
          onOpen={() =>
            onOpenEnvironment ? onOpenEnvironment(gateway.id) : void navigate(routes.deploy.environment(gateway.id))
          }
          onExplore={() => void navigate(routes.deploy.explore(gateway.id))}
          onConfigure={() => void navigate(routes.deploy.values(gateway.id))}
          onRevert={() => panels.revert(gateway)}
          onRedeploy={() => panels.redeploy(gateway)}
          onRetryRefused={(keys: string[]) => panels.retryRefused(gateway, keys)}
          extra={index === 0 ? <AutoApplySwitch target={entry.name} /> : undefined}
        />
      </Fragment>
    );
  };

  const rows: PipelineStage[][] = [];
  for (let start = 0; start < stages.length; start += STAGES_PER_ROW)
    rows.push(stages.slice(start, start + STAGES_PER_ROW));
  const columns = (count: number): string => `minmax(0, 1.05fr)${' auto minmax(0, 1fr)'.repeat(count)}`;

  return (
    <Stack spacing={2} component="section" aria-label={t('environments.title', 'Environments')}>
      <Box>
        <Typography variant="h5">{t('environments.title', 'Environments')}</Typography>
        <Typography variant="body2" color="text.secondary">
          {stages.length > 1
            ? t(
                'environments.description',
                'A deploy enters at {{first}}; each later environment is promoted to from the one before it. Changes go live in an environment only when they are deployed there.',
                {first: first?.name},
              )
            : t(
                'environments.descriptionSingle',
                'Your configuration deploys to {{first}}. Changes go live there only when they are deployed.',
                {first: first?.name ?? ''},
              )}
        </Typography>
      </Box>
      {pipeline.isError && (
        <Alert severity="error">{t('environments.failed', 'The environments could not be read.')}</Alert>
      )}
      {versions.isError && (
        <Alert severity="error">{t('page.versionsFailed', 'The configuration versions could not be read.')}</Alert>
      )}
      {pipeline.isLoading && (
        <Box sx={{display: 'flex', justifyContent: 'center', py: 6}}>
          <CircularProgress size={28} />
        </Box>
      )}
      {pipeline.isSuccess && stages.length === 0 && (
        <Alert severity="info">
          {t('environments.none', 'Register a gateway to have an environment to deploy to.')}
        </Alert>
      )}
      {rows.map((row: PipelineStage[], rowIndex: number) => (
        <Box
          key={row[0].gateway.id}
          sx={{display: 'grid', gridTemplateColumns: columns(row.length), alignItems: 'stretch'}}
        >
          {rowIndex === 0 ? (
            <VersionsCard
              versions={all}
              isLoading={versions.isLoading}
              onDeploy={(version: Version) =>
                first && panels.deploy(version, first.gateway.id, undefined, [first.gateway])
              }
              deploying={panels.deploying}
              runners={runners.running}
              partlyRunners={runners.partly}
              targets={stages.map((entry: PipelineStage) => entry.name)}
              deployTargets={first ? [first.name] : []}
            />
          ) : (
            <Box />
          )}
          {row.map((entry: PipelineStage) => stage(entry, stages.indexOf(entry)))}
        </Box>
      ))}
      {panels.panels}
    </Stack>
  );
}
