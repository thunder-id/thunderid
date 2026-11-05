// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {DocumentTitle} from '@thunderid/components';
import {
  Alert,
  Box,
  Button,
  Card,
  Chip,
  CircularProgress,
  PageContent,
  PageTitle,
  Stack,
  Tab,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
  Tabs,
  Typography,
} from '@wso2/oxygen-ui';
import {ExternalLink} from '@wso2/oxygen-ui-icons-react';
import {useMemo, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useParams} from 'react-router';
import {useAppliedVersion, useRunningReferences} from '../api/useGatewayVersions';
import {useMaster, useMasterHistory} from '../api/useMaster';
import {usePipeline} from '../api/usePipeline';
import {useStoredValues} from '../api/useStoredValues';
import {useVersions} from '../api/useVersions';
import ActorName from '../components/ActorName';
import AutoApplySwitch from '../components/AutoApplySwitch';
import CopyableUrl from '../components/CopyableUrl';
import Timestamp from '../components/Timestamp';
import ValuesTable, {type ValueRow} from '../components/ValuesTable';
import {gatewayEndpoints, type Endpoint} from '../explorer/endpoints';
import useDeployPanels from '../hooks/useDeployPanels';
import type {Master, MasterEntry, ValueKind, Version} from '../models/deploy';
import type {PipelineStage} from '../models/pipeline';
import {versionLabel} from '../utils/format';
import valueRowsOf from '../utils/valueRows';

type View = 'overview' | ValueKind;

/** How many of an environment's deploys the overview lists. */
const RECENT = 5;

/**
 * One environment's own page: the version it runs and how it compares with the latest, where clients
 * reach it, what it holds resource by resource, its recent deploys, and its variables and secrets.
 */
export default function EnvironmentPage(): JSX.Element {
  const {t} = useTranslation('deploy');
  const {gatewayId = ''} = useParams<{gatewayId: string}>();
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
  const entries: MasterEntry[] = master.data?.entries ?? [];

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
        <Typography variant="h6">{t('environment.deployed', 'Deployed configuration')}</Typography>
        <Typography variant="body2" color="text.secondary" sx={{mb: 2}}>
          {t('environment.deployedHint', 'Each resource {{name}} holds, as it last accepted it.', {name: stage.name})}
        </Typography>
        {master.isError && (
          <Alert severity="error">
            {t('environment.deployedFailed', 'What {{name}} holds could not be read.', {name: stage.name})}
          </Alert>
        )}
        {master.isSuccess && entries.length === 0 && (
          <Typography variant="body2" color="text.secondary">
            {t('stage.nothingDeployed', 'Nothing deployed yet')}
          </Typography>
        )}
        {entries.length > 0 && (
          <Box sx={{overflowX: 'auto'}}>
            <Table size="small" data-testid="environment-resources">
              <TableHead>
                <TableRow>
                  <TableCell>{t('environment.resource', 'Resource')}</TableCell>
                  <TableCell>{t('environment.type', 'Type')}</TableCell>
                  <TableCell>{t('environment.version', 'Version')}</TableCell>
                  <TableCell>{t('environment.status', 'Status')}</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {entries.map((entry: MasterEntry) => (
                  <TableRow key={entry.key}>
                    <TableCell>{entry.name ?? entry.id ?? entry.key}</TableCell>
                    <TableCell>{entry.resourceType}</TableCell>
                    <TableCell>{entry.sourceVersion ? label(entry.sourceVersion) : ''}</TableCell>
                    <TableCell>
                      {entry.state === 'failed' ? (
                        <Chip
                          size="small"
                          color="error"
                          label={t('environment.refused', 'Refused')}
                          title={entry.failure?.message}
                        />
                      ) : (
                        <Chip
                          size="small"
                          color={entry.state === 'removed' ? 'default' : 'success'}
                          label={
                            entry.state === 'removed'
                              ? t('environment.removed', 'Removed')
                              : t('environment.running', 'Running')
                          }
                        />
                      )}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
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

  const values = (kind: ValueKind): JSX.Element => {
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
    return (
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
  };

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
        {view === 'overview' ? overview : values(view)}
      </Stack>
      {panels.panels}
    </PageContent>
  );
}
