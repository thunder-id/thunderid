// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Alert, Box, Button, CircularProgress, Stack, Typography} from '@wso2/oxygen-ui';
import {ArrowRight, Rocket, Send} from '@wso2/oxygen-ui-icons-react';
import {Fragment, useMemo, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import AutoApplySwitch from './AutoApplySwitch';
import GatewayCard, {type GatewayStanding} from './GatewayCard';
import GatewaysStrip from './GatewaysStrip';
import type {VersionRunner} from './GatewayTag';
import PipelineVersions from './PipelineVersions';
import useAppliedVersions from '../api/useAppliedVersions';
import {useApplyCheck} from '../api/useGatewayVersions';
import {usePipeline} from '../api/usePipeline';
import {useProjectStatuses} from '../api/useProjectStatus';
import {useVersions} from '../api/useVersions';
import useProjectScope from '../contexts/ProjectScope';
import useDeployPanels from '../hooks/useDeployPanels';
import useDeployRoutes, {gatewaySectionOf} from '../hooks/useDeployRoutes';
import type {AppliedVersion, Change, Version} from '../models/deploy';
import type {Gateway} from '../models/gateway';
import type {PipelineStage} from '../models/pipeline';
import type {ProjectStatus} from '../models/projectStatus';
import {versionLabel} from '../utils/format';
import {projectRollbackOf} from '../utils/rollback';

export interface GatewaysOverviewProps {
  /** Opens a gateway's own page; its route otherwise. */
  onOpenGateway?: (gatewayId: string) => void;
  /** Above the headline, such as the project the gateways are shown for. */
  eyebrow?: string;
}

/** No gateways, for when there is no project to read the status of. */
const NO_STAGES: PipelineStage[] = [];

/** The arrow from the versions to the first gateway, which is where every deploy enters. */
function DeployJoint({testId}: {testId: string}): JSX.Element {
  const {t} = useTranslation('deploy');
  return (
    <Stack
      alignItems="center"
      justifyContent="center"
      spacing={0.5}
      sx={{flex: '0 0 auto', pt: 3.5, px: 0.5, color: 'primary.main'}}
      data-testid={testId}
    >
      <Typography variant="caption" fontWeight={600} color="inherit">
        {t('pipeline.deployJoint', 'Deploy')}
      </Typography>
      <Box sx={{display: 'flex', alignItems: 'center', gap: 0.5}} aria-hidden>
        <Box sx={{display: 'flex'}} data-testid="path-rocket">
          <Rocket size={14} />
        </Box>
        <ArrowRight size={18} />
      </Box>
    </Stack>
  );
}

/** What joins one gateway to the next: a promote, offered while there is something to carry. */
function PromoteJoint({
  onClick,
  disabled,
  testId,
}: {
  onClick: () => void;
  disabled: boolean;
  testId: string;
}): JSX.Element {
  const {t} = useTranslation('deploy');
  return (
    <Stack
      alignItems="center"
      justifyContent="center"
      spacing={0.5}
      sx={{flex: '0 0 auto', color: disabled ? 'text.secondary' : 'primary.main'}}
      data-testid={testId}
    >
      <Button
        variant="text"
        size="small"
        onClick={onClick}
        disabled={disabled}
        sx={{minWidth: 0, px: 1, borderRadius: 100, fontSize: 12}}
      >
        {t('gateways.promote', 'Promote')}
      </Button>
      <Box sx={{display: 'flex', alignItems: 'center', gap: 0.5}} aria-hidden>
        <Box sx={{display: 'flex'}} data-testid="path-plane">
          <Send size={14} />
        </Box>
        <ArrowRight size={18} />
      </Box>
    </Stack>
  );
}

/**
 * How many changes a deploy check found, which is how far the first gateway is behind. For a
 * project, only its own changes and what they cannot run without count.
 */
function changeCount(changes: Change[] | undefined): number | undefined {
  if (!changes) return undefined;
  return changes.filter(
    (change: Change) =>
      change.change !== 'unchanged' &&
      (change.section === undefined || change.section === 'project' || change.required === true),
  ).length;
}

/**
 * The gateways configuration is deployed and promoted through, in order, beside the versions it is
 * captured as: a deploy enters at the first gateway, and each later one is promoted to from the one
 * before it. The gateways sit on one row that scrolls sideways, each saying how it stands.
 */
export default function GatewaysOverview({
  onOpenGateway = undefined,
  eyebrow = undefined,
}: GatewaysOverviewProps): JSX.Element {
  const {t} = useTranslation('deploy');
  const navigate = useNavigate();
  const routes = useDeployRoutes();
  const project = useProjectScope();
  const pipeline = usePipeline();
  const versions = useVersions();
  const stages: PipelineStage[] = useMemo(() => pipeline.data?.stages ?? [], [pipeline.data]);
  const gateways: Gateway[] = useMemo(() => stages.map((stage: PipelineStage) => stage.gateway), [stages]);
  const applied = useAppliedVersions(gateways.map((gateway: Gateway) => gateway.id));
  const statuses = useProjectStatuses(project ? stages : NO_STAGES);
  const statusOf = (gatewayId: string): ProjectStatus | undefined => statuses.byGateway[gatewayId];
  // What of the project's own waits there: its changes and what they need that the gateway lacks.
  const projectOwes = (gatewayId: string): number => {
    const status = statusOf(gatewayId);
    return status ? status.project.pending + status.project.required : 0;
  };
  const all: Version[] = useMemo(() => versions.data ?? [], [versions.data]);
  const panels = useDeployPanels(gateways, all, applied.byGateway, stages);
  const first = stages[0];
  const latest = all[0];
  const firstApplied: AppliedVersion | undefined = first ? applied.byGateway[first.gateway.id] : undefined;
  const firstRuns = firstApplied?.appliedVersion ?? undefined;
  // Behind only once what it runs is known, so a card does not flash "behind" while it loads.
  const firstBehind =
    Boolean(first && latest) &&
    !applied.isLoading &&
    (firstRuns !== latest?.version || firstApplied?.complete === false);
  // How many changes the latest version would bring to the first gateway, as a deploy would check.
  const check = useApplyCheck(first?.gateway.id ?? '', latest?.version, firstBehind);
  const firstChanges = changeCount(check.data?.diff.changes);
  // A project is behind only by changes of its own: the latest version can differ in another's alone.
  const firstOwes = firstBehind && firstChanges !== 0;

  // The versions each gateway runs: for a project, the versions its own resources there came from,
  // since a gateway can run another project's newer version.
  const runners = useMemo(() => {
    const byHash: Record<string, VersionRunner[]> = {};
    stages.forEach((stage: PipelineStage, index: number) => {
      const hashes = project
        ? (statuses.byGateway[stage.gateway.id]?.project.versions ?? []).map((share) => share.hash)
        : [applied.byGateway[stage.gateway.id]?.appliedVersion].filter((hash): hash is string => Boolean(hash));
      hashes.forEach((hash: string) => {
        byHash[hash] = [...(byHash[hash] ?? []), {name: stage.name, index}];
      });
    });
    return byHash;
  }, [stages, applied.byGateway, project, statuses.byGateway]);

  const positionOf = (hash: string | undefined): number =>
    hash ? all.findIndex((version: Version) => version.version === hash) : -1;

  const standingOf = (index: number): GatewayStanding | undefined => {
    if (project) {
      // A project is described by its own resources: a promotion can carry part of a version, so what
      // the gateway last received says nothing about the project.
      const status = statusOf(stages[index].gateway.id);
      if (!status) return undefined;
      const owes = projectOwes(stages[index].gateway.id);
      if (index === 0) return owes > 0 ? {kind: 'behind', changes: owes} : {kind: 'inSync'};
      const previous = stages[index - 1].name;
      return owes > 0 ? {kind: 'projectBehindPrevious', count: owes, previous} : {kind: 'matches', previous};
    }
    if (applied.isLoading) return undefined;
    if (index === 0) {
      if (!latest) return undefined;
      return firstOwes ? {kind: 'behind', changes: firstChanges} : {kind: 'inSync'};
    }
    const previous = stages[index - 1];
    const before = applied.byGateway[previous.gateway.id]?.appliedVersion;
    const here = applied.byGateway[stages[index].gateway.id]?.appliedVersion;
    if (!before) return undefined;
    if (here === before) return {kind: 'matches', previous: previous.name};
    const behind = positionOf(here) - positionOf(before);
    if (positionOf(here) >= 0 && positionOf(before) >= 0 && behind > 0)
      return {kind: 'versionsBehind', previous: previous.name, count: behind};
    return {kind: 'ready', previous: previous.name};
  };

  // Promoting takes the version the gateway before runs to this one. There is nothing to promote
  // while that one runs nothing, or while this one already runs all of it.
  const promotable = (index: number): Version | undefined => {
    const before = applied.byGateway[stages[index - 1].gateway.id];
    const here = applied.byGateway[stages[index].gateway.id];
    if (!before?.appliedVersion) return undefined;
    if (project) {
      // A project's promotion is offered while its own changes, what they need or shared updates wait.
      const status = statusOf(stages[index].gateway.id);
      if (!status || (projectOwes(stages[index].gateway.id) === 0 && status.shared.pending === 0)) return undefined;
      return all.find((version: Version) => version.version === before.appliedVersion);
    }
    if (here?.appliedVersion === before.appliedVersion && here.complete !== false) return undefined;
    return all.find((version: Version) => version.version === before.appliedVersion);
  };

  const deployToFirst = (version: Version): void => {
    if (first) panels.deploy(version, first.gateway.id, undefined, [first.gateway]);
  };

  const cardOf = (stage: PipelineStage, index: number, version: Version | undefined): JSX.Element => {
    const {gateway} = stage;
    return (
      <GatewayCard
        stage={stage}
        index={index}
        applied={applied.byGateway[gateway.id]}
        versions={all}
        standing={standingOf(index)}
        projectStatus={project ? statusOf(gateway.id) : undefined}
        inProject={Boolean(project)}
        statusFailed={Boolean(project) && Boolean(statuses.failed[gateway.id])}
        onRetryStatus={() => statuses.retry(gateway.id)}
        previousName={index > 0 ? stages[index - 1].name : undefined}
        onReview={
          index === 0
            ? latest && (() => deployToFirst(latest))
            : version && (() => panels.deploy(version, gateway.id, undefined, [gateway]))
        }
        onOpen={() =>
          onOpenGateway ? onOpenGateway(gateway.id) : void navigate(routes.deploy.gateway(gateway.id, project))
        }
        onExplore={() =>
          void navigate(
            project ? gatewaySectionOf(routes, gateway.id, project, 'explore') : routes.deploy.explore(gateway.id),
          )
        }
        onConfigure={() =>
          void navigate(
            project ? gatewaySectionOf(routes, gateway.id, project, 'values') : routes.deploy.values(gateway.id),
          )
        }
        onRevert={() => {
          const back = projectRollbackOf(statusOf(gateway.id));
          panels.revert(gateway, back ? versionLabel(back.to.hash, back.to.number) : undefined, back?.undo);
        }}
        onDeploy={
          index === 0 && latest && (project ? projectOwes(gateway.id) > 0 : firstOwes)
            ? () => deployToFirst(latest)
            : undefined
        }
        onRedeploy={() => panels.redeploy(gateway)}
        onRetryRefused={(keys: string[]) => panels.retryRefused(gateway, keys)}
        extra={index === 0 ? <AutoApplySwitch target={stage.name} targetId={gateway.id} compact /> : undefined}
      />
    );
  };

  // A project that runs in one gateway has no pipeline to show: its gateway stands alone.
  const single = Boolean(project) && stages.length === 1;
  // The lead names production as where promotions end only when the last gateway is it.
  const lastIsProduction = /^prod(uction)?$/i.test(stages[stages.length - 1]?.name.trim() ?? '');
  // How many changes the latest version brings to the first gateway, as its card says.
  const latestChanges =
    project && first ? (statusOf(first.gateway.id) ? projectOwes(first.gateway.id) : undefined) : firstChanges;

  return (
    <Stack spacing={5} component="section" aria-label={t('gateways.title', 'Gateways')}>
      <Stack spacing={1.25} sx={{maxWidth: 900}}>
        <Typography variant="overline" color="primary" sx={{fontWeight: 600, letterSpacing: '0.08em'}}>
          {(eyebrow ?? t('pipeline.eyebrow', 'Gateways')).toUpperCase()}
        </Typography>
        <Typography variant="h1" sx={{fontSize: {xs: '2rem', md: '2.5rem'}, fontWeight: 400, lineHeight: 1.15}}>
          {t('pipeline.headline', 'Identity and access, deployed like code')}
        </Typography>
        <Typography variant="body1" color="text.secondary">
          {single
            ? t(
                'pipeline.leadProjectSingle',
                'This project runs in one gateway. Your configuration deploys straight to it.',
              )
            : stages.length > 1
              ? lastIsProduction
                ? t(
                    'pipeline.leadProduction',
                    'Set up your applications, sign-in flows and access rules once. Deploy them to your first gateway, then promote them through each gateway to production, just like a CI/CD pipeline. Changes go live in a gateway only when you deploy them there.',
                  )
                : t(
                    'pipeline.lead',
                    'Set up your applications, sign-in flows and access rules once. Deploy them to your first gateway, then promote them through each gateway to the last, just like a CI/CD pipeline. Changes go live in a gateway only when you deploy them there.',
                  )
              : t(
                  'pipeline.leadSingle',
                  'Set up your applications, sign-in flows and access rules once, then deploy them to your gateway. Changes go live there only when you deploy them.',
                )}
        </Typography>
      </Stack>
      {pipeline.isError && <Alert severity="error">{t('gateways.failed', 'The gateways could not be read.')}</Alert>}
      {versions.isError && (
        <Alert severity="error">{t('page.versionsFailed', 'The configuration versions could not be read.')}</Alert>
      )}
      {pipeline.isLoading && (
        <Box sx={{display: 'flex', justifyContent: 'center', py: 6}}>
          <CircularProgress size={28} />
        </Box>
      )}
      {pipeline.isSuccess && stages.length === 0 && (
        <Alert severity="info">{t('gateways.none', 'Register a gateway to have a gateway to deploy to.')}</Alert>
      )}
      {first && single && (
        <Stack
          component="section"
          aria-label={t('pipeline.singleRegion', 'Your gateway')}
          spacing={1.5}
          sx={{maxWidth: 360}}
          data-testid="single-gateway"
        >
          <Typography variant="overline" color="text.secondary" sx={{fontWeight: 600}}>
            {t('pipeline.singleTitle', 'Your gateway')}
          </Typography>
          {cardOf(first, 0, undefined)}
          <Typography variant="caption" color="text.secondary">
            {t('pipeline.singleHint', 'Add another gateway to get a full deployment pipeline.')}
          </Typography>
        </Stack>
      )}
      {first && !single && (
        <Box
          component="section"
          aria-label={t('pipeline.region', 'Deployment pipeline')}
          sx={{display: 'flex', flexWrap: 'wrap', alignItems: 'stretch', gap: 2}}
        >
          <PipelineVersions
            versions={all}
            isLoading={versions.isLoading}
            runners={runners}
            firstRuns={project ? (statusOf(first.gateway.id)?.project.versions[0]?.hash ?? undefined) : firstRuns}
            firstName={first.name}
            latestChanges={latestChanges}
            onDeploy={deployToFirst}
            deploying={panels.deploying}
          />
          <DeployJoint testId={`gateway-connector-${first.gateway.id}`} />
          <GatewaysStrip count={stages.length}>
            {stages.map((stage: PipelineStage, index: number) => {
              const {gateway} = stage;
              const version = index === 0 ? undefined : promotable(index);
              return (
                <Fragment key={gateway.id}>
                  {index > 0 && (
                    <PromoteJoint
                      disabled={!version}
                      onClick={() => version && panels.deploy(version, gateway.id, undefined, [gateway])}
                      testId={`gateway-promote-${gateway.id}`}
                    />
                  )}
                  {cardOf(stage, index, version)}
                </Fragment>
              );
            })}
          </GatewaysStrip>
        </Box>
      )}
      {panels.panels}
    </Stack>
  );
}
