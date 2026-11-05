// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {DocumentTitle} from '@thunderid/components';
import {
  Alert,
  Box,
  Button,
  Card,
  CircularProgress,
  PageContent,
  PageTitle,
  Stack,
  Tab,
  Tabs,
  Typography,
} from '@wso2/oxygen-ui';
import {ArrowRight, ExternalLink} from '@wso2/oxygen-ui-icons-react';
import {useMemo, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate, useParams} from 'react-router';
import {useAppliedConfiguration, useAppliedVersion, useRunningReferences} from '../api/useGatewayVersions';
import {useMaster, useMasterHistory} from '../api/useMaster';
import {usePipeline} from '../api/usePipeline';
import {useStoredValues} from '../api/useStoredValues';
import {useVersions} from '../api/useVersions';
import ActorName from '../components/ActorName';
import AutoApplySwitch from '../components/AutoApplySwitch';
import CopyableUrl from '../components/CopyableUrl';
import ResourceValuesView from '../components/ResourceValuesView';
import Timestamp from '../components/Timestamp';
import ValuesTable, {type ValueRow} from '../components/ValuesTable';
import {gatewayEndpoints, type Endpoint} from '../explorer/endpoints';
import {buildGraph, LANES, laneOfType, type Lane, type LaneSummary} from '../explorer/model';
import useDeployPanels from '../hooks/useDeployPanels';
import useDeployRoutes from '../hooks/useDeployRoutes';
import type {Master, MasterEntry, ValueKind, Version} from '../models/deploy';
import type {PipelineStage} from '../models/pipeline';
import {versionLabel} from '../utils/format';
import valueRowsOf from '../utils/valueRows';

type View = 'overview' | 'resources' | ValueKind;

/** What an environment holds of one kind of resource, as the explorer groups them. */
interface KindSummary {
  lane: Lane;
  count: number;
  values: number;
  refused: number;
  removed: number;
}

/** How many of an environment's deploys the overview lists. */
const RECENT = 5;

/**
 * One environment's own page: the version it runs and how it compares with the latest, where clients
 * reach it, what it holds resource by resource, its recent deploys, and its variables and secrets.
 */
export default function EnvironmentPage(): JSX.Element {
  const {t} = useTranslation('deploy');
  const {gatewayId = ''} = useParams<{gatewayId: string}>();
  const navigate = useNavigate();
  const routes = useDeployRoutes();
  const [view, setView] = useState<View>('overview');
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
  const configuration = useAppliedConfiguration(gatewayId);
  const history = useMasterHistory(gatewayId);
  const running = useRunningReferences(gatewayId);
  const variables = useStoredValues(gatewayId, 'variable');
  const secrets = useStoredValues(gatewayId, 'secret');
  const references = useMemo(() => running.data ?? [], [running.data]);
  const variableRows = useMemo(
    () => valueRowsOf('variable', variables.data ?? [], references),
    [variables.data, references],
  );
  const secretRows = useMemo(() => valueRowsOf('secret', secrets.data ?? [], references), [secrets.data, references]);
  const missing = [...variableRows, ...secretRows].filter(
    (row: ValueRow) => !row.stored && row.references.length > 0,
  ).length;
  const graph = useMemo(
    () => buildGraph(configuration.data?.resources ?? [], references),
    [configuration.data, references],
  );
  const kinds: KindSummary[] = useMemo(() => {
    const masterEntries: MasterEntry[] = master.data?.entries ?? [];
    const count = (lane: Lane, state: MasterEntry['state']): number =>
      masterEntries.filter((entry: MasterEntry) => entry.state === state && laneOfType(entry.resourceType) === lane)
        .length;
    return LANES.map((lane: Lane) => {
      const summary = graph.lanes.find((entry: LaneSummary) => entry.lane.id === lane.id);
      return {
        lane,
        count: summary?.count ?? 0,
        values: summary?.values ?? 0,
        refused: count(lane, 'failed'),
        removed: count(lane, 'removed'),
      };
    }).filter((kind: KindSummary) => kind.count + kind.refused + kind.removed > 0);
  }, [graph, master.data]);
  const gateways = useMemo(() => (stage ? [stage.gateway] : []), [stage]);
  const panels = useDeployPanels(gateways, all, applied.data ? {[gatewayId]: applied.data} : {});

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
            ? t('environments.failed', 'The environments could not be read.')
            : t('environment.notFound', 'No environment runs on this gateway.')}
        </Alert>
      </PageContent>
    );
  }

  const label = (hash: string | undefined): string => panels.labelOf(hash) || versionLabel(hash);
  const runs = applied.data?.appliedVersion;
  // What this environment takes next: the latest version for the first, what the one before runs for a
  // later one. Nothing, while it already runs that whole.
  const source = index === 0 ? all[0]?.version : beforeApplied.data?.appliedVersion;
  const next =
    source && (source !== runs || applied.data?.complete === false)
      ? all.find((version: Version) => version.version === source)
      : undefined;

  const endpoints: Endpoint[] = gatewayEndpoints(stage.gateway.baseUrl);
  const overview = (
    <Stack spacing={3}>
      <Card variant="outlined" sx={{p: 3}} component="section" aria-label={t('environment.endpoints', 'Endpoints')}>
        <Typography variant="h6" sx={{mb: 2}}>
          {t('environment.endpoints', 'Endpoints')}
        </Typography>
        <Stack spacing={1.5}>
          <Box>
            <Typography variant="caption" color="text.secondary" component="div">
              {t('environment.baseUrl', 'Base URL')}
            </Typography>
            <CopyableUrl url={stage.gateway.baseUrl} />
          </Box>
          {endpoints.map((endpoint: Endpoint) => (
            <Box key={endpoint.key}>
              <Typography variant="caption" color="text.secondary" component="div">
                {t(endpoint.label[0], endpoint.label[1])}
              </Typography>
              <CopyableUrl url={endpoint.url} />
            </Box>
          ))}
        </Stack>
      </Card>

      <Card
        variant="outlined"
        sx={{p: 3}}
        component="section"
        aria-label={t('environment.deployed', 'Deployed configuration')}
      >
        <Stack direction="row" spacing={2} alignItems="flex-start" sx={{mb: 2, flexWrap: 'wrap', rowGap: 1}}>
          <Box sx={{flex: 1, minWidth: 240}}>
            <Typography variant="h6">{t('environment.deployed', 'Deployed configuration')}</Typography>
            <Typography variant="body2" color="text.secondary">
              {t('environment.deployedHint', 'What {{name}} holds, by kind of resource.', {name: stage.name})}
            </Typography>
          </Box>
          <Button
            variant="outlined"
            endIcon={<ArrowRight size={16} />}
            onClick={() => void navigate(routes.deploy.explore(gatewayId))}
            data-testid="environment-explore"
          >
            {t('environment.explore', 'Explore deployed configuration')}
          </Button>
        </Stack>
        {(master.isError || configuration.isError) && (
          <Alert severity="error">
            {t('environment.deployedFailed', 'What {{name}} holds could not be read.', {name: stage.name})}
          </Alert>
        )}
        {master.isSuccess && configuration.isSuccess && kinds.length === 0 && (
          <Typography variant="body2" color="text.secondary">
            {t('stage.nothingDeployed', 'Nothing deployed yet')}
          </Typography>
        )}
        {kinds.length > 0 && (
          <Box
            data-testid="environment-resources"
            sx={{display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(240px, 1fr))', gap: 1.5}}
          >
            {kinds.map((kind: KindSummary) => (
              <Stack
                key={kind.lane.id}
                direction="row"
                alignItems="center"
                spacing={1}
                sx={{px: 1.5, py: 1, border: 1, borderColor: 'divider', borderRadius: 1}}
              >
                <Box sx={{width: 8, height: 8, borderRadius: 0.5, bgcolor: kind.lane.color, flexShrink: 0}} />
                <Typography variant="body2" sx={{flex: 1, minWidth: 0}} noWrap>
                  {t(kind.lane.title[0].replace(/^deploy:/, ''), kind.lane.title[1])}
                </Typography>
                <Stack alignItems="flex-end">
                  <Typography variant="caption" color="text.secondary">
                    {kind.values > 0
                      ? t('explorer.panel.laneValues', '{{count}} · {{values}} values', {
                          count: kind.count,
                          values: kind.values,
                        })
                      : t('explorer.panel.laneShared', '{{count}} · shared', {count: kind.count})}
                  </Typography>
                  {kind.refused > 0 && (
                    <Typography variant="caption" color="error.main">
                      {t('environment.kindRefused', '{{count}} refused', {count: kind.refused})}
                    </Typography>
                  )}
                  {kind.removed > 0 && (
                    <Typography variant="caption" color="text.secondary">
                      {t('environment.kindRemoved', '{{count}} removed', {count: kind.removed})}
                    </Typography>
                  )}
                </Stack>
              </Stack>
            ))}
          </Box>
        )}
      </Card>

      <Card
        variant="outlined"
        sx={{p: 3}}
        component="section"
        aria-label={t('environment.recent', 'Recent deployments')}
      >
        <Typography variant="h6" sx={{mb: 2}}>
          {t('environment.recent', 'Recent deployments')}
        </Typography>
        {history.isError && (
          <Alert severity="error">{t('environment.recentFailed', 'The deployments could not be read.')}</Alert>
        )}
        {history.isSuccess && (history.data ?? []).length === 0 && (
          <Typography variant="body2" color="text.secondary">
            {t('stage.nothingDeployed', 'Nothing deployed yet')}
          </Typography>
        )}
        <Stack spacing={1.5} data-testid="environment-history">
          {(history.data ?? []).slice(0, RECENT).map((entry: Master) => (
            <Stack
              key={entry.master ?? entry.number}
              direction="row"
              spacing={2}
              alignItems="center"
              sx={{flexWrap: 'wrap'}}
            >
              <Typography variant="body2" fontWeight={600}>
                {t('environment.deployNumber', 'Deploy {{number}}', {number: entry.number})}
              </Typography>
              <Timestamp value={entry.createdAt} />
              {entry.createdBy && <ActorName actor={entry.createdBy} />}
              <Typography variant="caption" color="text.secondary">
                {t('environment.deploySummary', '{{deployed}} deployed, {{failed}} refused, {{removed}} removed', {
                  deployed: entry.summary.deployed,
                  failed: entry.summary.failed,
                  removed: entry.summary.removed,
                })}
              </Typography>
            </Stack>
          ))}
        </Stack>
      </Card>
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

  const resources = (): JSX.Element =>
    valuesState() ?? (
      <ResourceValuesView
        key={gatewayId}
        gatewayId={gatewayId}
        name={stage.name}
        references={references}
        variables={variables.data ?? []}
        secrets={secrets.data ?? []}
      />
    );

  const values = (kind: ValueKind): JSX.Element =>
    valuesState() ?? (
      <Stack spacing={2}>
        {missing > 0 && (
          <Alert severity="warning">
            {t('values.missing', 'The version {{gateway}} runs refers to {{count}} values it does not hold.', {
              gateway: stage.name,
              count: missing,
            })}
          </Alert>
        )}
        <ValuesTable
          key={`${gatewayId}:${kind}`}
          gatewayId={gatewayId}
          kind={kind}
          rows={kind === 'secret' ? secretRows : variableRows}
        />
      </Stack>
    );

  return (
    <PageContent>
      <DocumentTitle title={stage.name} />
      <PageTitle>
        <PageTitle.Header>{stage.name}</PageTitle.Header>
        <PageTitle.SubHeader>
          {runs
            ? t('environment.runs', 'Runs {{version}}.', {version: label(runs)})
            : t('stage.nothingDeployed', 'Nothing deployed yet')}
        </PageTitle.SubHeader>
        <PageTitle.Actions>
          {stage.adminConsoleUrl && (
            <Button
              component="a"
              href={stage.adminConsoleUrl}
              target="_blank"
              rel="noopener noreferrer"
              variant="outlined"
              endIcon={<ExternalLink size={16} />}
              data-testid="environment-admin-console"
            >
              {t('stage.adminConsole', 'Open the admin portal')}
            </Button>
          )}
        </PageTitle.Actions>
      </PageTitle>

      <Stack spacing={3}>
        <Alert
          severity={next ? 'warning' : 'info'}
          data-testid="environment-status"
          action={
            next ? (
              <Button
                color="inherit"
                size="small"
                onClick={() => panels.deploy(next, gatewayId, undefined, gateways)}
                data-testid="environment-deploy"
              >
                {index === 0
                  ? t('environment.reviewAndDeploy', 'Review and deploy')
                  : t('environments.promote', 'Promote')}
              </Button>
            ) : undefined
          }
        >
          {next
            ? t('environment.behind', '{{version}} is ready for {{name}}, which runs {{running}}.', {
                version: label(next.version),
                name: stage.name,
                running: runs ? label(runs) : t('environment.nothing', 'nothing yet'),
              })
            : t('environment.upToDate', '{{name}} runs everything there is for it now.', {name: stage.name})}
        </Alert>
        {index === 0 && (
          <Card variant="outlined" sx={{p: 2}}>
            <AutoApplySwitch target={stage.name} />
          </Card>
        )}
        <Tabs value={view} onChange={(_event, value: View) => setView(value)}>
          <Tab value="overview" label={t('environment.overview', 'Overview')} sx={{textTransform: 'none'}} />
          <Tab
            value="resources"
            label={
              missing > 0
                ? t('environment.resourcesMissing', 'Resources ({{count}} to set)', {count: missing})
                : t('environment.resources', 'Resources')
            }
            sx={{textTransform: 'none'}}
            data-testid="environment-resources-tab"
          />
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
        {view === 'overview' && overview}
        {view === 'resources' && resources()}
        {view !== 'overview' && view !== 'resources' && values(view)}
      </Stack>
      {panels.panels}
    </PageContent>
  );
}
