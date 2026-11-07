// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {DocumentTitle} from '@thunderid/components';
import {
  Accordion,
  AccordionDetails,
  AccordionSummary,
  Alert,
  Box,
  Button,
  Card,
  CircularProgress,
  FormControlLabel,
  PageContent,
  Stack,
  Switch,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import {ChevronDown, Info} from '@wso2/oxygen-ui-icons-react';
import {useMemo, useState, type JSX, type ReactNode} from 'react';
import {Trans, useTranslation} from 'react-i18next';
import {Link, useParams} from 'react-router';
import useAppliedVersions from '../api/useAppliedVersions';
import useChangedValues, {changedCount} from '../api/useChangedValues';
import {useApplyCheck, useAppliedVersion, useRunningReferences} from '../api/useGatewayVersions';
import {useMaster, useMasterHistory} from '../api/useMaster';
import {usePipeline} from '../api/usePipeline';
import useProjectStatus from '../api/useProjectStatus';
import {useStoredValues} from '../api/useStoredValues';
import {useVersions} from '../api/useVersions';
import ActorName from '../components/ActorName';
import CopyableUrl from '../components/CopyableUrl';
import DeployedResources from '../components/DeployedResources';
import GatewayAdministration from '../components/GatewayAdministration';
import ResourceNamesProvider from '../components/ResourceNamesProvider';
import ResourceValuesView from '../components/ResourceValuesView';
import ValuesTable, {type ValueRow} from '../components/ValuesTable';
import {gatewayColor} from '../constants/gateway-colors';
import useProjectScope from '../contexts/ProjectScope';
import {clientEndpoints, gatewayEndpoints, type Endpoint} from '../explorer/endpoints';
import useChangeName from '../hooks/useChangeName';
import useDeployPanels from '../hooks/useDeployPanels';
import useDeployRoutes, {gatewaySectionOf, resourceEditOf, type GatewaySection} from '../hooks/useDeployRoutes';
import type {Change, Master, MasterEntry, ValueKind, ValueReference, Version} from '../models/deploy';
import type {PipelineStage} from '../models/pipeline';
import {humanize} from '../utils/groupByResource';
import relativeTime from '../utils/relativeTime';
import {projectRollbackOf, rollbackHint} from '../utils/rollback';
import valueRowsOf from '../utils/valueRows';

export interface GatewayPageProps {
  /** The section of the gateway shown; its overview by default. */
  section?: GatewaySection;
}

/** How many of a gateway's deploys the overview lists. */
const RECENT = 3;

/** The key a resource is named by in a deploy's check. */
const keyOf = (resourceType: string, id: string | undefined): string => `${resourceType}/${id ?? ''}`;

/** A section of the page: a heading, an optional line under it, and what it holds. */
function Section({
  title,
  subtitle = undefined,
  action = undefined,
  children,
  testId,
}: {
  title: string;
  subtitle?: string;
  action?: ReactNode;
  children: ReactNode;
  testId: string;
}): JSX.Element {
  return (
    <Card variant="outlined" sx={{p: 3}} component="section" aria-label={title} data-testid={testId}>
      <Stack direction="row" spacing={2} alignItems="flex-start" sx={{mb: 2}}>
        <Box sx={{flex: 1, minWidth: 0}}>
          <Typography variant="h6">{title}</Typography>
          {subtitle && (
            <Typography variant="body2" color="text.secondary">
              {subtitle}
            </Typography>
          )}
        </Box>
        {action}
      </Stack>
      {children}
    </Card>
  );
}

/**
 * One gateway's own pages: an overview of what it runs, resource by resource, against the
 * configuration; the values it holds, set per resource; where clients reach it; and its deploys. A
 * banner says how it stands and offers the deploy that brings it up to date. In a project, everything is
 * about the project's own resources and the ones they need.
 */
/** A gateway's own page, with the resources it runs named as its map names them. */
export default function GatewayPage({section = 'overview'}: GatewayPageProps): JSX.Element {
  const {gatewayId = ''} = useParams<{gatewayId: string}>();
  return (
    <ResourceNamesProvider targetIds={[gatewayId]}>
      <GatewayPageContent section={section} />
    </ResourceNamesProvider>
  );
}

function GatewayPageContent({section}: Required<GatewayPageProps>): JSX.Element {
  const {t} = useTranslation('deploy');
  const nameOfChange = useChangeName();
  const {gatewayId = ''} = useParams<{gatewayId: string}>();
  const routes = useDeployRoutes();
  const project = useProjectScope();
  const [showAllValues, setShowAllValues] = useState(false);
  const pipeline = usePipeline();
  const stages: PipelineStage[] = useMemo(() => pipeline.data?.stages ?? [], [pipeline.data]);
  const index = stages.findIndex((stage: PipelineStage) => stage.gateway.id === gatewayId);
  const stage = index < 0 ? undefined : stages[index];
  const before = index > 0 ? stages[index - 1] : undefined;
  const applied = useAppliedVersion(gatewayId);
  const beforeApplied = useAppliedVersion(before?.gateway.id ?? '', Boolean(before));
  const versions = useVersions();
  const all: Version[] = useMemo(() => versions.data ?? [], [versions.data]);
  const master = useMaster(gatewayId);
  const history = useMasterHistory(gatewayId);
  const running = useRunningReferences(gatewayId);
  const variables = useStoredValues(gatewayId, 'variable');
  const secrets = useStoredValues(gatewayId, 'secret');
  const status = useProjectStatus(gatewayId, before?.gateway.id);
  const runs = applied.data?.appliedVersion ?? undefined;
  const changedCheck = useChangedValues(gatewayId, runs);
  const owed = changedCount(changedCheck.changed);
  // What this gateway is compared with: the latest version for the first, what the one before runs
  // for a later one.
  const source = index === 0 ? all[0]?.version : (beforeApplied.data?.appliedVersion ?? undefined);
  const check = useApplyCheck(gatewayId, source);
  const changes: Change[] = useMemo(() => check.data?.diff.changes ?? [], [check.data]);
  // In a project, the page is about its own resources and the ones they need, as its check places them.
  const projectKeys: Set<string> | undefined = useMemo(() => {
    if (!project) return undefined;
    return new Set(
      changes
        .filter((change: Change) => change.section === 'project' || change.section === 'needed')
        .map((change: Change) => keyOf(change.resourceType, change.id)),
    );
  }, [project, changes]);
  const allReferences = useMemo(() => running.data ?? [], [running.data]);
  const references = useMemo(
    () =>
      projectKeys
        ? allReferences.filter((reference: ValueReference) =>
            projectKeys.has(keyOf(reference.resourceType, reference.resourceId)),
          )
        : allReferences,
    [allReferences, projectKeys],
  );
  const gateways = useMemo(() => (stage ? [stage.gateway] : []), [stage]);
  const everyApplied = useAppliedVersions(stages.map((entry: PipelineStage) => entry.gateway.id));
  const panels = useDeployPanels(gateways, all, everyApplied.byGateway, stages);

  if (pipeline.isLoading) {
    return (
      <PageContent>
        <Box sx={{display: 'flex', justifyContent: 'center', py: 8}}>
          <CircularProgress size={28} />
        </Box>
      </PageContent>
    );
  }
  if (!stage) {
    return (
      <PageContent>
        <Alert severity="error">
          {pipeline.isError
            ? t('gateways.failed', 'The gateways could not be read.')
            : t('gateway.notFound', 'No gateway runs on this gateway.')}
        </Alert>
      </PageContent>
    );
  }

  const label = (hash: string | undefined): string => panels.labelOf(hash);
  const tag = stage.name.toUpperCase();
  const color = gatewayColor(index);
  const next = source ? all.find((version: Version) => version.version === source) : undefined;
  const entries: MasterEntry[] = master.data?.entries ?? [];
  const entryOf = (key: string): MasterEntry | undefined => entries.find((entry: MasterEntry) => entry.key === key);
  const inScope = (resourceType: string, id: string | undefined): boolean =>
    !projectKeys || projectKeys.has(keyOf(resourceType, id));
  const pending = project
    ? (status.data?.project.pending ?? 0) + (status.data?.project.required ?? 0)
    : changes.filter((change: Change) => change.change !== 'unchanged').length;
  const projectVersion = status.data?.project.versions[0];
  const olderVersion = status.data?.project.versions[1];
  // In a project, the version its own resources came from, as the project home's card says it.
  let runningLabel: string | undefined;
  if (project) {
    runningLabel = projectVersion
      ? olderVersion
        ? t('gateway.versionPartly', '{{version}}, partly {{older}}', {
            version: label(projectVersion.hash),
            older: label(olderVersion.hash),
          })
        : label(projectVersion.hash)
      : undefined;
  } else {
    runningLabel = runs ? label(runs) : undefined;
  }
  const deployedAt = project ? status.data?.deployedAt : (applied.data?.appliedAt ?? undefined);
  const autoSyncOn = index === 0 && project !== undefined && status.data?.autoSync?.enabled === true;
  const review = (): void => {
    if (next) panels.deploy(next, gatewayId, undefined, gateways);
  };

  // The version in type as the design sets it, and the changes owed in the accent.
  const bannerParts = {
    mono: <Box component="span" sx={{fontFamily: 'monospace'}} />,
    b: <Box component="b" sx={{fontWeight: 600, color: 'primary.light'}} />,
  };
  let bannerText: ReactNode;
  if (autoSyncOn) {
    bannerText = (
      <Trans
        t={t}
        i18nKey="gateway.bannerAutoSync"
        values={{tag, version: runningLabel ?? t('gateway.nothing', 'nothing yet')}}
        components={bannerParts}
      />
    );
  } else {
    let runsText: ReactNode = <Trans t={t} i18nKey="gateway.bannerNothing" values={{tag}} components={bannerParts} />;
    if (runningLabel && deployedAt) {
      runsText = (
        <Trans
          t={t}
          i18nKey="gateway.bannerRuns"
          values={{tag, version: runningLabel, when: relativeTime(deployedAt)}}
          components={bannerParts}
        />
      );
    } else if (runningLabel) {
      runsText = (
        <Trans
          t={t}
          i18nKey="gateway.bannerRunsUndated"
          values={{tag, version: runningLabel}}
          components={bannerParts}
        />
      );
    }
    bannerText = (
      <>
        {runsText}{' '}
        {pending > 0 ? (
          <Trans t={t} i18nKey="gateway.bannerPending" count={pending} components={bannerParts} />
        ) : (
          t('gateway.bannerUpToDate', 'Everything in the configuration is deployed here.')
        )}
      </>
    );
  }

  // A strip under the header across the whole page, as the design has it: how the gateway stands, and
  // the deploy that brings it up to date while the configuration owes it changes.
  const banner = (
    <Box
      role="status"
      sx={{
        display: 'flex',
        flexWrap: 'wrap',
        alignItems: 'center',
        columnGap: 2.5,
        rowGap: 1.5,
        px: 3,
        py: 1.75,
        bgcolor: 'background.paper',
        borderBottom: 1,
        borderColor: 'divider',
      }}
      data-testid="gateway-status"
    >
      <Box aria-hidden sx={{display: 'flex', color: 'text.secondary'}}>
        <Info size={20} />
      </Box>
      <Typography component="span" sx={{flex: '1 1 420px'}}>
        {bannerText}
      </Typography>
      {next && pending > 0 && !autoSyncOn && (
        <Button
          variant="contained"
          onClick={review}
          sx={{minHeight: 40, px: 2, borderRadius: '20px', fontWeight: 600, whiteSpace: 'nowrap'}}
          data-testid="gateway-deploy"
        >
          {index === 0 ? t('gateway.reviewAndDeploy', 'Review and deploy') : t('gateways.promote', 'Promote')}
        </Button>
      )}
    </Box>
  );

  const runningOf = (change: Change): string | undefined => {
    const entry = entryOf(change.key);
    return entry && entry.state !== 'removed' && entry.sourceVersion ? label(entry.sourceVersion) : undefined;
  };
  const actionOf = (change: Change): ReactNode => {
    const edit = change.id ? resourceEditOf(routes, change.resourceType, change.id) : undefined;
    // A resource not running as the configuration has it can be brought on its own: in a project, one of
    // its own, with what it needs.
    const bringable = change.failed === true || change.change !== 'unchanged';
    const bring =
      bringable && next && (!project || change.section === 'project') ? (
        <Button
          size="small"
          variant="text"
          onClick={() => panels.deploy(next, gatewayId, [change.key], gateways)}
          data-testid="gateway-resource-deploy"
        >
          {index === 0 ? t('pipeline.deploy', 'Deploy') : t('gateways.promote', 'Promote')}
        </Button>
      ) : null;
    // A resource being removed, or one never deployed, is brought, not edited here.
    const editLink =
      edit && !(bring && (change.change === 'added' || change.change === 'deleted')) ? (
        <Button size="small" variant="text" component={Link} to={edit}>
          {/* What the project needs belongs to the organization, which edits it for every project. */}
          {project && change.section !== 'project'
            ? t('gateway.editInOrganization', 'Edit in organization settings')
            : t('gateway.editConfiguration', 'Edit configuration')}
        </Button>
      ) : null;
    if (!bring && !editLink) return null;
    return (
      <Stack direction="row" spacing={0.5} sx={{justifyContent: 'flex-end'}}>
        {bring}
        {editLink}
      </Stack>
    );
  };

  const rows = changes.filter(
    (change: Change) => !project || change.section === 'project' || change.section === 'needed',
  );
  const exploreTo = project
    ? gatewaySectionOf(routes, gatewayId, project, 'explore')
    : routes.deploy.explore(gatewayId);

  let rollbackTo: string | undefined;
  const projectRollback = project ? projectRollbackOf(status.data) : undefined;
  if (project) {
    rollbackTo = projectRollback ? label(projectRollback.to.hash) : undefined;
  } else if (applied.data?.previousVersion) {
    rollbackTo = label(applied.data.previousVersion);
  }
  const writtenBy = (deploy: Master): MasterEntry[] =>
    entries.filter(
      (entry: MasterEntry) => entry.at && entry.at === deploy.createdAt && inScope(entry.resourceType, entry.id),
    );
  const deployLine = (deploy: Master): string =>
    [
      relativeTime(deploy.createdAt),
      t('gateway.changeCount', {
        count: deploy.summary.deployed + deploy.summary.failed + deploy.summary.removed,
        defaultValue_one: '{{count}} change',
        defaultValue_other: '{{count}} changes',
      }),
    ].join(' · ');
  const deployHeader = (deploy: Master, position: number): JSX.Element => (
    <Stack direction="row" spacing={2} alignItems="center" sx={{flex: 1, flexWrap: 'wrap', rowGap: 0.5}}>
      <Typography variant="body2" fontWeight={600}>
        {t('gateway.deployNumber', 'Deploy {{number}}', {number: deploy.number})}
      </Typography>
      {deploy.createdBy && (
        <Stack direction="row" spacing={0.5} alignItems="center">
          <Typography variant="body2" color="text.secondary">
            {t('gateway.deployedBy', 'Deployed by')}
          </Typography>
          <ActorName actor={deploy.createdBy} />
        </Stack>
      )}
      <Typography variant="body2" color="text.secondary">
        {deployLine(deploy)}
      </Typography>
      {position === 0 && rollbackTo && (
        <Tooltip title={projectRollback ? rollbackHint(t, projectRollback, rollbackTo, stage.name) : ''}>
          <Button
            size="small"
            variant="text"
            color="inherit"
            sx={{ml: 'auto', color: 'text.secondary'}}
            onClick={(event) => {
              event.stopPropagation();
              panels.revert(stage.gateway, rollbackTo, projectRollback?.undo);
            }}
            data-testid="gateway-rollback"
          >
            {projectRollback?.undo
              ? t('card.undoLastDeploy', 'Undo last deploy')
              : t('card.rollBackTo', 'Roll back to {{version}}', {version: rollbackTo})}
          </Button>
        </Tooltip>
      )}
    </Stack>
  );
  const deploys = (count?: number): JSX.Element => (
    <Stack spacing={1} data-testid="gateway-history">
      {history.isError && (
        <Alert severity="error">{t('gateway.recentFailed', 'The deployments could not be read.')}</Alert>
      )}
      {history.isSuccess && (history.data ?? []).length === 0 && (
        <Typography variant="body2" color="text.secondary">
          {t('stage.nothingDeployed', 'Nothing deployed yet')}
        </Typography>
      )}
      {(history.data ?? []).slice(0, count).map((deploy: Master, position: number) => {
        if (count !== undefined) {
          return (
            <Box key={deploy.master ?? deploy.number} sx={{py: 1, borderBottom: 1, borderColor: 'divider'}}>
              {deployHeader(deploy, position)}
            </Box>
          );
        }
        const written = writtenBy(deploy);
        return (
          <Accordion key={deploy.master ?? deploy.number} disableGutters variant="outlined">
            <AccordionSummary expandIcon={<ChevronDown size={16} />}>{deployHeader(deploy, position)}</AccordionSummary>
            <AccordionDetails>
              <Typography variant="body2" color="text.secondary" sx={{mb: written.length > 0 ? 1 : 0}}>
                {t('gateway.deploySummary', '{{deployed}} deployed, {{failed}} refused, {{removed}} removed', {
                  deployed: deploy.summary.deployed,
                  failed: deploy.summary.failed,
                  removed: deploy.summary.removed,
                })}
              </Typography>
              {written.map((entry: MasterEntry) => (
                <Typography key={entry.key} variant="body2">
                  {humanize(entry.resourceType)} · {nameOfChange(entry)}
                </Typography>
              ))}
            </AccordionDetails>
          </Accordion>
        );
      })}
    </Stack>
  );

  const issuer: Endpoint = {key: 'issuer', label: ['gateway.issuer', 'Issuer'], url: stage.gateway.baseUrl};
  const endpointList = (list: Endpoint[]): JSX.Element => (
    <Stack spacing={1.5}>
      {list.map((endpoint: Endpoint) => (
        <Box key={endpoint.key}>
          <Typography variant="caption" color="text.secondary" component="div">
            {t(endpoint.label[0], endpoint.label[1])}
          </Typography>
          <CopyableUrl url={endpoint.url} />
        </Box>
      ))}
    </Stack>
  );
  const discovery = gatewayEndpoints(stage.gateway.baseUrl).filter(
    (endpoint: Endpoint) => endpoint.key === 'discovery',
  );
  const everyEndpoint = [
    issuer,
    ...gatewayEndpoints(stage.gateway.baseUrl),
    ...clientEndpoints(stage.gateway.baseUrl).filter(
      (endpoint: Endpoint) => endpoint.key !== 'discovery' && endpoint.key !== 'jwks',
    ),
  ];

  let comparedWith: string | undefined;
  if (next && index === 0) {
    comparedWith = t('gateway.comparedWithConfig', 'Compared with configuration {{version}}', {
      version: label(next.version),
    });
  } else if (next) {
    comparedWith = t('gateway.comparedWithBefore', 'Compared with {{name}}, which runs {{version}}', {
      name: before?.name,
      version: label(next.version),
    });
  }

  const overview = (
    <Stack spacing={3}>
      <Section title={t('gateway.endpoints', 'Endpoints')} testId="gateway-endpoints">
        {endpointList([issuer, ...discovery])}
      </Section>
      <Section
        title={t('gateway.deployed', 'Deployed configuration')}
        subtitle={comparedWith}
        action={
          <Button size="small" variant="text" component={Link} to={exploreTo} data-testid="gateway-explore">
            {t('gateway.exploreMap', 'Explore')}
          </Button>
        }
        testId="gateway-deployed"
      >
        {check.isError && (
          <Alert severity="error">
            {t('gateway.deployedFailed', 'What {{name}} holds could not be read.', {name: stage.name})}
          </Alert>
        )}
        {check.isLoading && <CircularProgress size={20} />}
        {check.isSuccess && rows.length === 0 && (
          <Typography variant="body2" color="text.secondary">
            {t('stage.nothingDeployed', 'Nothing deployed yet')}
          </Typography>
        )}
        {rows.length > 0 && (
          <DeployedResources
            changes={rows}
            isShared={(change: Change) => Boolean(project) && change.section === 'needed'}
            runningOf={runningOf}
            reasonOf={(change: Change) => entryOf(change.key)?.failure?.message}
            actionOf={actionOf}
          />
        )}
      </Section>
      <Section title={t('gateway.recent', 'Recent deployments')} testId="gateway-recent">
        {deploys(RECENT)}
      </Section>
    </Stack>
  );

  const valuesState = (): JSX.Element | null => {
    if (variables.isLoading || secrets.isLoading || running.isLoading) {
      return (
        <Box sx={{display: 'flex', justifyContent: 'center', py: 8}}>
          <CircularProgress size={28} />
        </Box>
      );
    }
    if (variables.isError || secrets.isError || running.isError) {
      return (
        <Alert severity="error">
          {t('values.readFailed', 'The values {{gateway}} holds could not be read.', {gateway: stage.name})}
        </Alert>
      );
    }
    return null;
  };
  const otherRows = (kind: ValueKind): ValueRow[] =>
    valueRowsOf(kind, (kind === 'secret' ? secrets.data : variables.data) ?? [], references).filter(
      (entry: ValueRow) => entry.references.length === 0,
    );
  const values = valuesState() ?? (
    <Stack spacing={3}>
      {owed > 0 && (
        <Alert
          severity="warning"
          data-testid="gateway-redeploy-notice"
          action={
            <Button color="inherit" size="small" onClick={() => panels.redeploy(stage.gateway)}>
              {t('stage.redeploy', 'Redeploy')}
            </Button>
          }
        >
          {t('gateway.savedRedeploy', {
            count: owed,
            defaultValue_one: 'Saved. {{count}} value changed since the last deploy: redeploy to apply it.',
            defaultValue_other: 'Saved. {{count}} values changed since the last deploy: redeploy to apply them.',
          })}
        </Alert>
      )}
      <ResourceValuesView
        key={gatewayId}
        gatewayId={gatewayId}
        name={stage.name}
        references={references}
        variables={variables.data ?? []}
        secrets={secrets.data ?? []}
      />
      <FormControlLabel
        control={
          <Switch
            checked={showAllValues}
            onChange={(_event, checked: boolean) => setShowAllValues(checked)}
            data-testid="gateway-show-all-values"
          />
        }
        label={t('environment.showAllValues', 'Show all environment values')}
      />
      {showAllValues &&
        (['variable', 'secret'] as ValueKind[]).map((kind: ValueKind) => (
          <Section
            key={kind}
            title={
              kind === 'secret'
                ? t('gateway.otherSecrets', 'Other secrets on {{name}}', {name: stage.name})
                : t('gateway.otherVariables', 'Other variables on {{name}}', {name: stage.name})
            }
            subtitle={t('gateway.otherValuesHint', 'Values belong to the gateway: no resource shown above uses these.')}
            testId={`gateway-other-${kind}`}
          >
            <ValuesTable gatewayId={gatewayId} kind={kind} rows={otherRows(kind)} />
          </Section>
        ))}
    </Stack>
  );

  const titles: Record<GatewaySection, string> = {
    overview: t('gateway.title', '{{name}} gateway', {name: stage.name}),
    values: t('gateway.valuesTitle', 'Variables & secrets'),
    endpoints: t('gateway.endpoints', 'Endpoints'),
    history: t('gateway.historyTitle', 'Deployment history'),
  };

  return (
    <Box sx={{display: 'flex', flexDirection: 'column', width: '100%', minWidth: 0}}>
      {banner}
      <PageContent>
        <DocumentTitle title={`${stage.name} · ${titles[section]}`} />
        <Stack spacing={3}>
          <Box>
            <Stack direction="row" spacing={1} alignItems="center" sx={{mb: 0.5}}>
              <Box sx={{width: 8, height: 8, borderRadius: '50%', bgcolor: color.light}} />
              <Typography
                variant="caption"
                sx={{fontFamily: 'monospace'}}
                color="text.secondary"
                data-testid="gateway-tag"
              >
                {runningLabel ? `${tag} · ${runningLabel}` : tag}
              </Typography>
            </Stack>
            <Typography variant="h4" component="h1">
              {titles[section]}
            </Typography>
          </Box>
          {section === 'overview' && <GatewayAdministration tag={tag} adminConsoleUrl={stage.adminConsoleUrl} />}
          {section === 'overview' && overview}
          {section === 'values' && values}
          {section === 'endpoints' && (
            <Card variant="outlined" sx={{p: 3}} data-testid="gateway-every-endpoint">
              {endpointList(everyEndpoint)}
            </Card>
          )}
          {section === 'history' && <Box data-testid="gateway-full-history">{deploys()}</Box>}
        </Stack>
        {panels.panels}
      </PageContent>
    </Box>
  );
}
