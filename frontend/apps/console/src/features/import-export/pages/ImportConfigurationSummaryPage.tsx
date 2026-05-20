/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {useToast, useConfig} from '@thunderid/contexts';
import {useLogger} from '@thunderid/logger/react';
import {
  Alert,
  Box,
  Breadcrumbs,
  Button,
  Chip,
  Divider,
  IconButton,
  LinearProgress,
  Paper,
  Stack,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import {
  Bell,
  Building,
  ChevronRight,
  Languages,
  LayoutGrid,
  Layout as LayoutIcon,
  Layers,
  Palette,
  Upload,
  UserRoundCog,
  UsersRound,
  Workflow,
  X,
} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useCallback, useEffect, useMemo, useRef, useState} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate, useLocation} from 'react-router';
import useImportConfiguration from '../api/useImportConfiguration';
import EnvVariablesViewer from '../components/EnvVariablesViewer';
import ResourceSummaryTable from '../components/ResourceSummaryTable';
import TemplateVariableDisplay from '../components/TemplateVariableDisplay';
import type {ConfigSummaryItem, ImportItemOutcome, ProductConfig} from '../models/import-configuration';
import getEnvFileName from '../utils/getEnvFileName';

function parseEnvData(envData: string | null): Map<string, string> {
  const entries = new Map<string, string>();

  if (!envData) {
    return entries;
  }

  envData.split(/\r?\n|\r/).forEach((line) => {
    const trimmed = line.trim();
    if (!trimmed || trimmed.startsWith('#')) {
      return;
    }

    const separatorIndex = trimmed.indexOf('=');
    if (separatorIndex <= 0) {
      return;
    }

    const key = trimmed.slice(0, separatorIndex).trim();
    const value = trimmed.slice(separatorIndex + 1).trim();

    entries.set(key, value);
  });

  return entries;
}

function parseArrayEnvValue(value: string): string[] {
  const trimmed = value.trim();
  if (!trimmed) {
    return [];
  }

  // Support list-like env formats such as ['a', 'b'] or ["a", "b"].
  const listMatch = /^\[(.*)\]$/s.exec(trimmed);
  if (listMatch) {
    const inner = listMatch[1].trim();
    if (!inner) {
      return [];
    }

    const items: string[] = [];
    const pattern = /'([^']*)'|"([^"]*)"|([^,\s]+)/g;
    let match: RegExpExecArray | null;

    while ((match = pattern.exec(inner)) !== null) {
      const item = (match[1] ?? match[2] ?? match[3] ?? '').trim();
      if (item) {
        items.push(item);
      }
    }

    if (items.length > 0) {
      return items;
    }
  }

  return trimmed
    .split(/[\s,]+/)
    .map((item) => item.trim().replace(/^['"]|['"]$/g, ''))
    .filter((item) => item.length > 0);
}

function collectTemplateVariables(value: unknown, variables: Set<string>): void {
  if (typeof value === 'string') {
    const matches = value.matchAll(/\{\{\s*\.\s*([A-Z_][A-Z0-9_]*)\s*\}\}/g);
    for (const match of matches) {
      if (match[1]) {
        variables.add(match[1]);
      }
    }
    return;
  }

  if (Array.isArray(value)) {
    value.forEach((item) => collectTemplateVariables(item, variables));
    return;
  }

  if (value && typeof value === 'object') {
    Object.values(value).forEach((item) => collectTemplateVariables(item, variables));
  }
}

export default function ImportConfigurationSummaryPage(): JSX.Element {
  const {t} = useTranslation('importExport');
  const navigate = useNavigate();
  const location = useLocation();
  const logger = useLogger('ImportConfigurationSummaryPage');
  const {showToast} = useToast();
  const {config} = useConfig();
  const productName = config.brand.product_name;
  const dryRunMutation = useImportConfiguration();
  const importMutation = useImportConfiguration();

  const configData = useMemo(() => {
    const state = location.state as {configData?: ProductConfig; envData?: string} | null;
    return state?.configData ?? null;
  }, [location.state]);

  // Use state for envData to allow editing
  const initialEnvData = useMemo(() => {
    const state = location.state as {configData?: ProductConfig; envData?: string} | null;
    return state?.envData ?? null;
  }, [location.state]);

  const configContent = useMemo(() => {
    const state = location.state as {configContent?: string} | null;
    return state?.configContent ?? null;
  }, [location.state]);

  const [envData, setEnvData] = useState<string | null>(initialEnvData);
  const envFileInputRef = useRef<HTMLInputElement>(null);
  const [dryRunStatus, setDryRunStatus] = useState<'idle' | 'running' | 'passed' | 'failed'>('idle');
  const [dryRunMessage, setDryRunMessage] = useState(t('summary.importTest.runToValidate'));
  const [dryRunFailedResults, setDryRunFailedResults] = useState<ImportItemOutcome[]>([]);
  const [lastDryRunSignature, setLastDryRunSignature] = useState<string | null>(null);

  // Expand/collapse state for each resource type
  const [expandedApplications, setExpandedApplications] = useState(false);
  const [expandedFlows, setExpandedFlows] = useState(false);
  const [expandedIdps, setExpandedIdps] = useState(false);
  const [expandedLayouts, setExpandedLayouts] = useState(false);
  const [expandedSenders, setExpandedSenders] = useState(false);
  const [expandedOrgUnits, setExpandedOrgUnits] = useState(false);
  const [expandedThemes, setExpandedThemes] = useState(false);
  const [expandedTranslations, setExpandedTranslations] = useState(false);
  const [expandedUsers, setExpandedUsers] = useState(false);
  const [expandedSchemas, setExpandedSchemas] = useState(false);

  // Extract data from configuration
  const applicationsCount = Array.isArray(configData?.application) ? configData.application.length : 0;
  const usersCount = Array.isArray(configData?.user) ? configData.user.length : 0;
  const flowsCount = Array.isArray(configData?.flow) ? configData.flow.length : 0;
  const themesCount = Array.isArray(configData?.theme) ? configData.theme.length : 0;
  const orgUnitsCount = Array.isArray(configData?.organization_unit) ? configData.organization_unit.length : 0;
  const userTypesCount = Array.isArray(configData?.user_type) ? configData.user_type.length : 0;
  const translationsCount = Array.isArray(configData?.translation) ? configData.translation.length : 0;
  const identityProvidersCount = Array.isArray(configData?.identity_provider) ? configData.identity_provider.length : 0;
  const notificationSendersCount = Array.isArray(configData?.notification_sender)
    ? configData.notification_sender.length
    : 0;
  const layoutsCount = Array.isArray(configData?.layout) ? configData.layout.length : 0;

  const envVariables = useMemo(() => parseEnvData(envData), [envData]);

  const arrayVariableNames = useMemo(() => {
    const names = new Set<string>();
    if (configContent) {
      const rangePattern = /\{\{-?\s*range\s+\.([A-Z0-9_]+)\s*-?\}\}/g;
      let match: RegExpExecArray | null;
      while ((match = rangePattern.exec(configContent)) !== null) {
        names.add(match[1]);
      }
    }
    return names;
  }, [configContent]);

  const envVariablesObject = useMemo(() => {
    const variables: Record<string, string | string[]> = {};
    envVariables.forEach((value, key) => {
      if (arrayVariableNames.has(key)) {
        variables[key] = parseArrayEnvValue(value);
      } else {
        variables[key] = value;
      }
    });
    return variables;
  }, [envVariables, arrayVariableNames]);

  const dryRunSignature = useMemo(
    () => JSON.stringify({content: configContent ?? '', envData: envData ?? ''}),
    [configContent, envData],
  );

  const isDryRunFresh = lastDryRunSignature === dryRunSignature;
  const effectiveDryRunStatus = isDryRunFresh ? dryRunStatus : 'idle';

  const requiredEnvVariables = useMemo(() => {
    if (!configData) {
      return [];
    }

    const variables = new Set<string>();
    collectTemplateVariables(configData, variables);

    return Array.from(variables).sort();
  }, [configData]);

  const missingEnvVariables = useMemo(
    () => requiredEnvVariables.filter((name) => !envVariables.get(name)?.trim()),
    [envVariables, requiredEnvVariables],
  );

  const resolvedEnvVariablesCount = requiredEnvVariables.length - missingEnvVariables.length;

  const effectiveDryRunMessage = useMemo(() => {
    if (!configContent) {
      return t('summary.importTest.configUnavailable');
    }

    if (missingEnvVariables.length > 0) {
      return t('summary.importTest.fixMissingThenRun');
    }

    if (!isDryRunFresh || dryRunStatus === 'idle') {
      return t('summary.importTest.runToValidate');
    }

    return dryRunMessage;
  }, [configContent, missingEnvVariables.length, isDryRunFresh, dryRunStatus, dryRunMessage, t]);

  const handleEnvFileUpload = (e: React.ChangeEvent<HTMLInputElement>): void => {
    const file = e.target.files?.[0];
    if (!file) return;

    if (!file.name.endsWith('.env') && file.name !== '.env') {
      logger.warn('Invalid file type', {fileName: file.name});
      return;
    }

    const reader = new FileReader();
    reader.onload = (event): void => {
      const content = event.target?.result as string;
      setEnvData(content);
      logger.info('Environment file re-uploaded successfully', {fileName: file.name});
    };
    reader.readAsText(file);

    // Reset input so same file can be uploaded again
    if (envFileInputRef.current) {
      envFileInputRef.current.value = '';
    }
  };

  const handleUploadClick = (): void => {
    envFileInputRef.current?.click();
  };

  const runDryRun = useCallback(async (): Promise<void> => {
    if (!configContent || missingEnvVariables.length > 0) {
      return;
    }

    setDryRunStatus('running');
    setDryRunMessage(t('summary.importTest.running'));
    setDryRunFailedResults([]);

    try {
      setLastDryRunSignature(dryRunSignature);
      const response = await dryRunMutation.mutateAsync({
        content: configContent,
        variables: envVariablesObject,
        dryRun: true,
        options: {
          upsert: true,
          continueOnError: true,
          target: 'runtime',
        },
      });

      const failedResults = response.results.filter((result) => result.status === 'failed');

      if (failedResults.length === 0) {
        setDryRunStatus('passed');
        setDryRunMessage(
          t('summary.importTest.passed', {
            imported: response.summary.imported,
            totalDocuments: response.summary.totalDocuments,
          }),
        );
        return;
      }

      setDryRunStatus('failed');
      setDryRunFailedResults(failedResults);
      setDryRunMessage(t('summary.importTest.failedCount', {count: failedResults.length}));
    } catch (error) {
      setDryRunStatus('failed');
      setDryRunMessage(
        t('summary.importTest.failedWithMessage', {
          message: error instanceof Error ? error.message : t('common:dictionary.unknown'),
        }),
      );
      logger.error('Dry-run import failed', {error});
    }
  }, [configContent, dryRunMutation, dryRunSignature, envVariablesObject, logger, missingEnvVariables.length, t]);

  const handleRunDryRun = (): void => {
    void runDryRun();
  };

  useEffect(() => {
    if (
      lastDryRunSignature === null &&
      configContent &&
      missingEnvVariables.length === 0 &&
      dryRunStatus === 'idle' &&
      !dryRunMutation.isPending
    ) {
      const timer = setTimeout(() => {
        void runDryRun();
      }, 0);

      return () => clearTimeout(timer);
    }

    return undefined;
  }, [
    configContent,
    dryRunMutation.isPending,
    dryRunStatus,
    lastDryRunSignature,
    missingEnvVariables.length,
    runDryRun,
  ]);

  const summaryItems: ConfigSummaryItem[] = [];

  // Resources in alphabetical order

  // 1. Applications
  if (applicationsCount > 0) {
    const apps =
      (configData?.application as {
        name?: string;
        description?: string;
        url?: string;
        inbound_auth_config?: {type?: string; config?: {client_id?: string}}[];
      }[]) ?? [];
    const displayedApps = expandedApplications ? apps : apps.slice(0, 5);
    const remainingCount = apps.length - 5;

    summaryItems.push({
      id: 'applications',
      icon: <LayoutGrid size={16} />,
      label: t('export.table.applications'),
      value: applicationsCount,
      content: (
        <Box sx={{px: 3, py: 2, bgcolor: 'background.default'}}>
          <Stack spacing={2}>
            <Stack spacing={2} divider={<Box sx={{borderBottom: 1, borderColor: 'divider'}} />}>
              {displayedApps.map((app, idx) => {
                const clientId = app.inbound_auth_config?.find((cfg) => cfg.type === 'oauth2')?.config?.client_id;
                const appKey = app.name ?? clientId ?? `app-${idx}`;
                return (
                  <Stack key={appKey} spacing={0.5}>
                    <Stack direction="row" alignItems="center" spacing={1}>
                      <LayoutGrid size={14} />
                      <Typography variant="body2" fontWeight={600}>
                        {app.name ?? t('configureExport.fallback.unnamedApplication')}
                      </Typography>
                    </Stack>
                    {app.description && (
                      <Typography variant="caption" color="text.secondary" sx={{pl: 2.5}}>
                        {app.description}
                      </Typography>
                    )}
                    {clientId && (
                      <Box sx={{pl: 2.5}}>
                        <TemplateVariableDisplay text={clientId} envData={envData} label={t('export.app.clientId')} />
                      </Box>
                    )}
                    {app.url && (
                      <Typography variant="caption" color="primary.main" sx={{pl: 2.5}}>
                        {app.url}
                      </Typography>
                    )}
                  </Stack>
                );
              })}
            </Stack>
            {remainingCount > 0 && (
              <Box sx={{pt: 1, textAlign: 'center'}}>
                <Chip
                  label={
                    expandedApplications
                      ? t('configureExport.actions.showLess')
                      : t('configureExport.actions.more', {count: remainingCount})
                  }
                  size="small"
                  variant="outlined"
                  onClick={() => setExpandedApplications(!expandedApplications)}
                  sx={{cursor: 'pointer'}}
                />
              </Box>
            )}
          </Stack>
        </Box>
      ),
    });
  }

  // 2. Flows
  if (flowsCount > 0) {
    const flows = (configData?.flow as {name?: string; flowType?: string; handle?: string}[]) ?? [];
    const displayedFlows = expandedFlows ? flows : flows.slice(0, 5);
    const remainingCount = flows.length - 5;

    summaryItems.push({
      id: 'flows',
      icon: <Workflow size={16} />,
      label: t('export.table.flows'),
      value: flowsCount,
      content: (
        <Box sx={{px: 3, py: 2, bgcolor: 'background.default'}}>
          <Stack spacing={2}>
            <Stack spacing={2} divider={<Box sx={{borderBottom: 1, borderColor: 'divider'}} />}>
              {displayedFlows.map((flow, idx) => (
                <Stack key={flow.handle ?? flow.name ?? `flow-${idx}`} spacing={0.5}>
                  <Stack direction="row" alignItems="center" spacing={1}>
                    <Workflow size={14} />
                    <Typography variant="body2" fontWeight={600}>
                      {flow.name ?? flow.handle ?? t('summary.fallback.flow', {index: idx + 1})}
                    </Typography>
                    {flow.flowType && (
                      <Chip label={flow.flowType} size="small" sx={{height: 18, fontSize: '0.65rem'}} />
                    )}
                  </Stack>
                  {flow.handle && flow.name !== flow.handle && (
                    <Typography variant="caption" color="text.secondary" sx={{pl: 2.5, fontFamily: 'monospace'}}>
                      {flow.handle}
                    </Typography>
                  )}
                </Stack>
              ))}
            </Stack>
            {remainingCount > 0 && (
              <Box sx={{pt: 1, textAlign: 'center'}}>
                <Chip
                  label={
                    expandedFlows
                      ? t('configureExport.actions.showLess')
                      : t('configureExport.actions.more', {count: remainingCount})
                  }
                  size="small"
                  variant="outlined"
                  onClick={() => setExpandedFlows(!expandedFlows)}
                  sx={{cursor: 'pointer'}}
                />
              </Box>
            )}
          </Stack>
        </Box>
      ),
    });
  }

  // 3. Identity Providers (Integrations)
  if (identityProvidersCount > 0) {
    const idps = (configData?.identity_provider as {name?: string; handle?: string; type?: string}[]) ?? [];
    const displayedIdps = expandedIdps ? idps : idps.slice(0, 5);
    const remainingCount = idps.length - 5;

    summaryItems.push({
      id: 'integrations',
      icon: <Layers size={16} />,
      label: t('summary.labels.identityProviders'),
      value: identityProvidersCount,
      content: (
        <Box sx={{px: 3, py: 2, bgcolor: 'background.default'}}>
          <Stack spacing={2}>
            <Stack spacing={2} divider={<Box sx={{borderBottom: 1, borderColor: 'divider'}} />}>
              {displayedIdps.map((idp, idx) => (
                <Stack key={idp.handle ?? idp.name ?? `idp-${idx}`} spacing={0.5}>
                  <Stack direction="row" alignItems="center" spacing={1}>
                    <Layers size={14} />
                    <Typography variant="body2" fontWeight={600}>
                      {idp.name ?? t('configureExport.fallback.unnamedProvider')}
                    </Typography>
                    {idp.type && <Chip label={idp.type} size="small" sx={{height: 18, fontSize: '0.65rem'}} />}
                  </Stack>
                </Stack>
              ))}
            </Stack>
            {remainingCount > 0 && (
              <Box sx={{pt: 1, textAlign: 'center'}}>
                <Chip
                  label={
                    expandedIdps
                      ? t('configureExport.actions.showLess')
                      : t('configureExport.actions.more', {count: remainingCount})
                  }
                  size="small"
                  variant="outlined"
                  onClick={() => setExpandedIdps(!expandedIdps)}
                  sx={{cursor: 'pointer'}}
                />
              </Box>
            )}
          </Stack>
        </Box>
      ),
    });
  }

  // 4. Layouts
  if (layoutsCount > 0) {
    const layouts = (configData?.layout as {name?: string; handle?: string; description?: string}[]) ?? [];
    const displayedLayouts = expandedLayouts ? layouts : layouts.slice(0, 5);
    const remainingCount = layouts.length - 5;

    summaryItems.push({
      id: 'layouts',
      icon: <LayoutIcon size={16} />,
      label: t('configureExport.labels.layouts'),
      value: layoutsCount,
      content: (
        <Box sx={{px: 3, py: 2, bgcolor: 'background.default'}}>
          <Stack spacing={2}>
            <Stack spacing={2} divider={<Box sx={{borderBottom: 1, borderColor: 'divider'}} />}>
              {displayedLayouts.map((layout, idx) => (
                <Stack key={layout.handle ?? layout.name ?? `layout-${idx}`} spacing={0.5}>
                  <Stack direction="row" alignItems="center" spacing={1}>
                    <LayoutIcon size={14} />
                    <Typography variant="body2" fontWeight={600}>
                      {layout.name ?? layout.handle ?? t('configureExport.fallback.unnamedLayout')}
                    </Typography>
                  </Stack>
                  {layout.description && (
                    <Typography variant="caption" color="text.secondary" sx={{pl: 2.5}}>
                      {layout.description}
                    </Typography>
                  )}
                </Stack>
              ))}
            </Stack>
            {remainingCount > 0 && (
              <Box sx={{pt: 1, textAlign: 'center'}}>
                <Chip
                  label={
                    expandedLayouts
                      ? t('configureExport.actions.showLess')
                      : t('configureExport.actions.more', {count: remainingCount})
                  }
                  size="small"
                  variant="outlined"
                  onClick={() => setExpandedLayouts(!expandedLayouts)}
                  sx={{cursor: 'pointer'}}
                />
              </Box>
            )}
          </Stack>
        </Box>
      ),
    });
  }

  // 5. Notification Senders
  if (notificationSendersCount > 0) {
    const senders = (configData?.notification_sender as {name?: string; handle?: string; type?: string}[]) ?? [];
    const displayedSenders = expandedSenders ? senders : senders.slice(0, 5);
    const remainingCount = senders.length - 5;

    summaryItems.push({
      id: 'notification-senders',
      icon: <Bell size={16} />,
      label: t('configureExport.labels.notificationSenders'),
      value: notificationSendersCount,
      content: (
        <Box sx={{px: 3, py: 2, bgcolor: 'background.default'}}>
          <Stack spacing={2}>
            <Stack spacing={2} divider={<Box sx={{borderBottom: 1, borderColor: 'divider'}} />}>
              {displayedSenders.map((sender, idx) => (
                <Stack key={sender.handle ?? sender.name ?? `sender-${idx}`} spacing={0.5}>
                  <Stack direction="row" alignItems="center" spacing={1}>
                    <Bell size={14} />
                    <Typography variant="body2" fontWeight={600}>
                      {sender.name ?? t('configureExport.fallback.unnamedSender')}
                    </Typography>
                    {sender.type && <Chip label={sender.type} size="small" sx={{height: 18, fontSize: '0.65rem'}} />}
                  </Stack>
                </Stack>
              ))}
            </Stack>
            {remainingCount > 0 && (
              <Box sx={{pt: 1, textAlign: 'center'}}>
                <Chip
                  label={
                    expandedSenders
                      ? t('configureExport.actions.showLess')
                      : t('configureExport.actions.more', {count: remainingCount})
                  }
                  size="small"
                  variant="outlined"
                  onClick={() => setExpandedSenders(!expandedSenders)}
                  sx={{cursor: 'pointer'}}
                />
              </Box>
            )}
          </Stack>
        </Box>
      ),
    });
  }

  // 6. Organization Units
  if (orgUnitsCount > 0) {
    const orgUnits = (configData?.organization_unit as {name?: string; handle?: string; description?: string}[]) ?? [];
    const displayedOrgUnits = expandedOrgUnits ? orgUnits : orgUnits.slice(0, 5);
    const remainingCount = orgUnits.length - 5;

    summaryItems.push({
      id: 'organization-units',
      icon: <Building size={16} />,
      label: t('configureExport.labels.organizationUnits'),
      value: orgUnitsCount,
      content: (
        <Box sx={{px: 3, py: 2, bgcolor: 'background.default'}}>
          <Stack spacing={2}>
            <Stack spacing={2} divider={<Box sx={{borderBottom: 1, borderColor: 'divider'}} />}>
              {displayedOrgUnits.map((org, idx) => (
                <Stack key={org.handle ?? org.name ?? `org-${idx}`} spacing={0.5}>
                  <Stack direction="row" alignItems="center" spacing={1}>
                    <Building size={14} />
                    <Typography variant="body2" fontWeight={600}>
                      {org.name ?? org.handle ?? t('configureExport.fallback.unnamedOrganization')}
                    </Typography>
                  </Stack>
                  {org.description && (
                    <Typography variant="caption" color="text.secondary" sx={{pl: 2.5}}>
                      {org.description}
                    </Typography>
                  )}
                  {org.handle && org.name !== org.handle && (
                    <Typography variant="caption" color="text.secondary" sx={{pl: 2.5, fontFamily: 'monospace'}}>
                      {org.handle}
                    </Typography>
                  )}
                </Stack>
              ))}
            </Stack>
            {remainingCount > 0 && (
              <Box sx={{pt: 1, textAlign: 'center'}}>
                <Chip
                  label={
                    expandedOrgUnits
                      ? t('configureExport.actions.showLess')
                      : t('configureExport.actions.more', {count: remainingCount})
                  }
                  size="small"
                  variant="outlined"
                  onClick={() => setExpandedOrgUnits(!expandedOrgUnits)}
                  sx={{cursor: 'pointer'}}
                />
              </Box>
            )}
          </Stack>
        </Box>
      ),
    });
  }

  // 7. Themes
  if (themesCount > 0) {
    const themes = (configData?.theme as {name?: string; handle?: string; description?: string}[]) ?? [];
    const displayedThemes = expandedThemes ? themes : themes.slice(0, 5);
    const remainingCount = themes.length - 5;

    summaryItems.push({
      id: 'themes',
      icon: <Palette size={16} />,
      label: t('configureExport.labels.themes'),
      value: themesCount,
      content: (
        <Box sx={{px: 3, py: 2, bgcolor: 'background.default'}}>
          <Stack spacing={2}>
            <Stack spacing={2} divider={<Box sx={{borderBottom: 1, borderColor: 'divider'}} />}>
              {displayedThemes.map((theme, idx) => (
                <Stack key={theme.handle ?? theme.name ?? `theme-${idx}`} spacing={0.5}>
                  <Stack direction="row" alignItems="center" spacing={1}>
                    <Palette size={14} />
                    <Typography variant="body2" fontWeight={600}>
                      {theme.name ?? theme.handle ?? t('summary.fallback.theme', {index: idx + 1})}
                    </Typography>
                  </Stack>
                  {theme.description && (
                    <Typography variant="caption" color="text.secondary" sx={{pl: 2.5}}>
                      {theme.description}
                    </Typography>
                  )}
                  {theme.handle && theme.name !== theme.handle && (
                    <Typography variant="caption" color="text.secondary" sx={{pl: 2.5, fontFamily: 'monospace'}}>
                      {theme.handle}
                    </Typography>
                  )}
                </Stack>
              ))}
            </Stack>
            {remainingCount > 0 && (
              <Box sx={{pt: 1, textAlign: 'center'}}>
                <Chip
                  label={
                    expandedThemes
                      ? t('configureExport.actions.showLess')
                      : t('configureExport.actions.more', {count: remainingCount})
                  }
                  size="small"
                  variant="outlined"
                  onClick={() => setExpandedThemes(!expandedThemes)}
                  sx={{cursor: 'pointer'}}
                />
              </Box>
            )}
          </Stack>
        </Box>
      ),
    });
  }

  // 8. Translations
  if (translationsCount > 0) {
    const translations = (configData?.translation as {locale?: string; namespace?: string}[]) ?? [];
    const displayedTranslations = expandedTranslations ? translations : translations.slice(0, 5);
    const remainingCount = translations.length - 5;

    summaryItems.push({
      id: 'translations',
      icon: <Languages size={16} />,
      label: t('configureExport.labels.translations'),
      value: translationsCount,
      content: (
        <Box sx={{px: 3, py: 2, bgcolor: 'background.default'}}>
          <Stack spacing={2}>
            <Stack spacing={2} divider={<Box sx={{borderBottom: 1, borderColor: 'divider'}} />}>
              {displayedTranslations.map((trans, idx) => (
                <Stack key={trans.locale ?? `translation-${idx}`} spacing={0.5}>
                  <Stack direction="row" alignItems="center" spacing={1}>
                    <Languages size={14} />
                    <Typography variant="body2" fontWeight={600}>
                      {trans.locale ?? t('configureExport.fallback.unnamedTranslation')}
                    </Typography>
                    {trans.namespace && (
                      <Chip
                        label={trans.namespace}
                        size="small"
                        variant="outlined"
                        sx={{height: 18, fontSize: '0.65rem'}}
                      />
                    )}
                  </Stack>
                </Stack>
              ))}
            </Stack>
            {remainingCount > 0 && (
              <Box sx={{pt: 1, textAlign: 'center'}}>
                <Chip
                  label={
                    expandedTranslations
                      ? t('configureExport.actions.showLess')
                      : t('configureExport.actions.more', {count: remainingCount})
                  }
                  size="small"
                  variant="outlined"
                  onClick={() => setExpandedTranslations(!expandedTranslations)}
                  sx={{cursor: 'pointer'}}
                />
              </Box>
            )}
          </Stack>
        </Box>
      ),
    });
  }

  // 9. Users
  if (usersCount > 0) {
    const users =
      (configData?.user as {
        type?: string;
        attributes?: {
          username?: string;
          email?: string;
          name?: string;
          given_name?: string;
          family_name?: string;
        };
      }[]) ?? [];
    const displayedUsers = expandedUsers ? users : users.slice(0, 5);
    const remainingCount = users.length - 5;

    summaryItems.push({
      id: 'users',
      icon: <UsersRound size={16} />,
      label: t('configureExport.labels.users'),
      value: usersCount,
      content: (
        <Box sx={{px: 3, py: 2, bgcolor: 'background.default'}}>
          <Stack spacing={2}>
            <Stack spacing={2} divider={<Box sx={{borderBottom: 1, borderColor: 'divider'}} />}>
              {displayedUsers.map((user, idx) => (
                <Stack key={user.attributes?.username ?? user.attributes?.email ?? `user-${idx}`} spacing={0.5}>
                  <Stack direction="row" alignItems="center" spacing={1}>
                    <UsersRound size={14} />
                    <Typography variant="body2" fontWeight={600}>
                      {user.attributes?.name ??
                        user.attributes?.username ??
                        user.attributes?.email ??
                        t('summary.fallback.user', {index: idx + 1})}
                    </Typography>
                    {user.type && <Chip label={user.type} size="small" sx={{height: 18, fontSize: '0.65rem'}} />}
                  </Stack>
                  {user.attributes?.username && user.attributes.name !== user.attributes.username && (
                    <Typography variant="caption" color="text.secondary" sx={{pl: 2.5}}>
                      @{user.attributes.username}
                    </Typography>
                  )}
                  {user.attributes?.email && (
                    <Typography variant="caption" color="text.secondary" sx={{pl: 2.5}}>
                      {user.attributes.email}
                    </Typography>
                  )}
                </Stack>
              ))}
            </Stack>
            {remainingCount > 0 && (
              <Box sx={{pt: 1, textAlign: 'center'}}>
                <Chip
                  label={
                    expandedUsers
                      ? t('configureExport.actions.showLess')
                      : t('configureExport.actions.more', {count: remainingCount})
                  }
                  size="small"
                  variant="outlined"
                  onClick={() => setExpandedUsers(!expandedUsers)}
                  sx={{cursor: 'pointer'}}
                />
              </Box>
            )}
          </Stack>
        </Box>
      ),
    });
  }

  // 10. User Types
  if (userTypesCount > 0) {
    const schemas =
      (configData?.user_type as {name?: string; handle?: string; allow_self_registration?: boolean}[]) ?? [];
    const displayedSchemas = expandedSchemas ? schemas : schemas.slice(0, 5);
    const remainingCount = schemas.length - 5;

    summaryItems.push({
      id: 'user-types',
      icon: <UserRoundCog size={16} />,
      label: t('configureExport.labels.userTypes'),
      value: userTypesCount,
      content: (
        <Box sx={{px: 3, py: 2, bgcolor: 'background.default'}}>
          <Stack spacing={2}>
            <Stack spacing={2} divider={<Box sx={{borderBottom: 1, borderColor: 'divider'}} />}>
              {displayedSchemas.map((schema, idx) => (
                <Stack key={schema.handle ?? schema.name ?? `schema-${idx}`} spacing={0.5}>
                  <Stack direction="row" alignItems="center" spacing={1}>
                    <UserRoundCog size={14} />
                    <Typography variant="body2" fontWeight={600}>
                      {schema.name ?? schema.handle ?? t('summary.fallback.schema', {index: idx + 1})}
                    </Typography>
                    {schema.allow_self_registration && (
                      <Chip
                        label={t('configureExport.labels.selfRegistration')}
                        size="small"
                        color="success"
                        variant="outlined"
                        sx={{height: 18, fontSize: '0.65rem'}}
                      />
                    )}
                  </Stack>
                </Stack>
              ))}
            </Stack>
            {remainingCount > 0 && (
              <Box sx={{pt: 1, textAlign: 'center'}}>
                <Chip
                  label={
                    expandedSchemas
                      ? t('configureExport.actions.showLess')
                      : t('configureExport.actions.more', {count: remainingCount})
                  }
                  size="small"
                  variant="outlined"
                  onClick={() => setExpandedSchemas(!expandedSchemas)}
                  sx={{cursor: 'pointer'}}
                />
              </Box>
            )}
          </Stack>
        </Box>
      ),
    });
  }

  const handleClose = (): void => {
    void navigate('/home');
  };

  const handleProceed = (): void => {
    (async () => {
      if (!configContent || missingEnvVariables.length > 0 || effectiveDryRunStatus !== 'passed') {
        return;
      }

      const response = await importMutation.mutateAsync({
        content: configContent,
        variables: envVariablesObject,
        dryRun: false,
        options: {
          upsert: true,
          continueOnError: false,
          target: 'runtime',
        },
      });

      if (response.summary.failed > 0) {
        logger.warn('Import completed with failures', {summary: response.summary});
        showToast(t('summary.import.completedWithFailures', {count: response.summary.failed}), 'warning');
      } else {
        showToast(t('summary.import.completedSuccessfully', {count: response.summary.imported}), 'success');
      }

      await navigate('/home');
    })().catch((_error: unknown) => {
      logger.error('Failed to import configuration', {error: _error});
      showToast(t('summary.import.failedRetry'), 'error');
    });
  };

  return (
    <Box sx={{minHeight: '100vh', display: 'flex', flexDirection: 'column'}}>
      <LinearProgress variant="determinate" value={100} sx={{height: 6}} />

      <Box
        sx={{
          p: 4,
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
          flexShrink: 0,
        }}
      >
        <Stack direction="row" alignItems="center" spacing={2}>
          <IconButton
            aria-label={t('common:actions.close')}
            onClick={handleClose}
            sx={{bgcolor: 'background.paper', '&:hover': {bgcolor: 'action.hover'}, boxShadow: 1}}
          >
            <X size={24} />
          </IconButton>
          <Breadcrumbs separator={<ChevronRight size={16} />} aria-label="breadcrumb">
            <Typography
              variant="h5"
              onClick={() => void navigate('/welcome')}
              sx={{cursor: 'pointer', '&:hover': {textDecoration: 'underline'}}}
            >
              {t('common:welcome.header')}
            </Typography>
            <Typography
              variant="h5"
              onClick={() => void navigate('/welcome/open-project')}
              sx={{cursor: 'pointer', '&:hover': {textDecoration: 'underline'}}}
            >
              {t('upload.breadcrumb.openProject')}
            </Typography>
            <Typography variant="h5" color="text.primary">
              {t('summary.breadcrumb')}
            </Typography>
          </Breadcrumbs>
        </Stack>
      </Box>

      <Box
        sx={{
          flex: 1,
          display: 'flex',
          flexDirection: 'column',
          py: 8,
          px: {xs: 2, sm: 3, md: 8, lg: 20},
          alignItems: 'flex-start',
        }}
      >
        <Box
          sx={{
            width: '100%',
            maxWidth: 1600,
            display: 'flex',
            flexDirection: 'column',
          }}
        >
          <Stack spacing={1} mb={4}>
            <Stack direction="row" alignItems="center" spacing={2}>
              <Typography variant="h2" fontWeight={600}>
                {t('summary.title')}
              </Typography>
              <Chip label={t('summary.valid')} color="success" size="small" />
            </Stack>
            <Typography variant="body1" color="text.secondary">
              {t('summary.subtitle')}
            </Typography>
          </Stack>

          {/* Two-column layout: Main content and Pre-flight Check */}
          <Stack
            mb={4}
            sx={{
              flexDirection: {xs: 'column', lg: 'row'},
              alignItems: {xs: 'stretch', lg: 'flex-start'},
              gap: 3,
            }}
          >
            {/* Left: Main content */}
            <Box sx={{width: '100%', flex: {lg: '1 1 0'}, minWidth: 0}}>
              <Stack spacing={3}>
                {/* Project Details */}
                <Paper variant="outlined" sx={{p: 3, borderRadius: 2}}>
                  <Stack spacing={2}>
                    <Typography variant="h6" fontWeight={600}>
                      {t('summary.projectDetails')}
                    </Typography>
                    <Box>
                      <Typography variant="caption" color="text.secondary" sx={{mb: 0.5, display: 'block'}}>
                        {t('summary.totalResources')}
                      </Typography>
                      <Typography variant="body2" fontWeight={600}>
                        {summaryItems.reduce((sum, item) => sum + (typeof item.value === 'number' ? item.value : 0), 0)}
                      </Typography>
                    </Box>
                  </Stack>
                </Paper>

                {/* Resources Table */}
                <ResourceSummaryTable items={summaryItems} />

                {/* Environment Variables Section */}
                {envData && (
                  <Stack spacing={2}>
                    <Stack direction="row" justifyContent="space-between" alignItems="center">
                      <Typography variant="subtitle2">{t('envViewer.title')}</Typography>
                      <Button
                        size="small"
                        variant="outlined"
                        startIcon={<Upload size={16} />}
                        onClick={handleUploadClick}
                      >
                        {t('summary.actions.reuploadEnv')}
                      </Button>
                      <input
                        ref={envFileInputRef}
                        type="file"
                        accept=".env"
                        style={{display: 'none'}}
                        onChange={handleEnvFileUpload}
                      />
                    </Stack>
                    {missingEnvVariables.length > 0 && (
                      <Alert severity="info">
                        <Typography variant="caption">{t('summary.env.editInfo')}</Typography>
                      </Alert>
                    )}
                    <EnvVariablesViewer
                      content={envData}
                      editable={true}
                      onChange={setEnvData}
                      showDownload={false}
                      maxHeight={300}
                      fileName={getEnvFileName(productName)}
                    />
                  </Stack>
                )}
              </Stack>
            </Box>

            {/* Right: Pre-flight Check */}
            <Box sx={{width: '100%', maxWidth: {lg: 480}, flexShrink: 0}}>
              <Paper variant="outlined" sx={{p: 2.5}}>
                <Stack direction="column" spacing={2}>
                  <Typography variant="subtitle2">{t('summary.preImportValidation')}</Typography>
                  <Divider />
                  {requiredEnvVariables.length === 0 ? (
                    <Alert severity="success">
                      <Typography variant="caption">{t('summary.precheck.readyNoEnvRequired')}</Typography>
                    </Alert>
                  ) : missingEnvVariables.length === 0 ? (
                    <Alert severity="success">
                      <Typography variant="caption">
                        {t('summary.precheck.readyAllEnvAvailable', {count: requiredEnvVariables.length})}
                      </Typography>
                    </Alert>
                  ) : (
                    <Stack spacing={1.5}>
                      <Alert severity="error">
                        <Typography variant="caption">
                          {t('summary.precheck.missingEnvValues', {count: missingEnvVariables.length})}
                        </Typography>
                      </Alert>
                      {resolvedEnvVariablesCount > 0 && (
                        <Alert severity="info">
                          <Typography variant="caption">
                            {t('summary.precheck.availableEnvValues', {
                              resolved: resolvedEnvVariablesCount,
                              total: requiredEnvVariables.length,
                            })}
                          </Typography>
                        </Alert>
                      )}
                      <Box>
                        <Typography variant="caption" color="text.secondary">
                          {t('summary.precheck.missingVariables')}
                        </Typography>
                        <Stack spacing={1} sx={{mt: 1}}>
                          {missingEnvVariables.map((name) => (
                            <TemplateVariableDisplay key={name} text={`{{.${name}}}`} envData={envData} />
                          ))}
                        </Stack>
                      </Box>
                    </Stack>
                  )}

                  <Stack spacing={1.5}>
                    <Typography variant="subtitle2">{t('summary.importTest.status')}</Typography>
                    <Divider />

                    {effectiveDryRunStatus === 'idle' && (
                      <Alert
                        severity="info"
                        action={
                          <Button
                            size="small"
                            variant="contained"
                            onClick={handleRunDryRun}
                            disabled={!configContent || missingEnvVariables.length > 0 || dryRunMutation.isPending}
                          >
                            {dryRunMutation.isPending
                              ? t('summary.importTest.runningShort')
                              : t('summary.importTest.test')}
                          </Button>
                        }
                      >
                        {effectiveDryRunMessage}
                      </Alert>
                    )}
                    {effectiveDryRunStatus === 'running' && <Alert severity="warning">{effectiveDryRunMessage}</Alert>}
                    {effectiveDryRunStatus === 'passed' && <Alert severity="success">{effectiveDryRunMessage}</Alert>}
                    {effectiveDryRunStatus === 'failed' && (
                      <Alert
                        severity="error"
                        action={
                          <Button
                            size="small"
                            variant="contained"
                            onClick={handleRunDryRun}
                            disabled={!configContent || missingEnvVariables.length > 0 || dryRunMutation.isPending}
                          >
                            {dryRunMutation.isPending
                              ? t('summary.importTest.runningShort')
                              : t('summary.importTest.retry')}
                          </Button>
                        }
                      >
                        {effectiveDryRunMessage}
                      </Alert>
                    )}

                    {effectiveDryRunStatus === 'failed' && dryRunFailedResults.length > 0 && (
                      <Box>
                        <Typography variant="caption" color="text.secondary">
                          {t('summary.importTest.failures')}
                        </Typography>
                        <Stack spacing={1} sx={{mt: 1}}>
                          {dryRunFailedResults.map((result) => (
                            <Typography key={JSON.stringify(result)} variant="caption" color="error.main">
                              {result.resourceType}
                              {result.resourceName ? ` (${result.resourceName})` : ''}:{' '}
                              {result.message ?? t('summary.importTest.failed')}
                            </Typography>
                          ))}
                        </Stack>
                      </Box>
                    )}
                  </Stack>
                </Stack>
              </Paper>
            </Box>
          </Stack>

          <Stack direction="row" spacing={2} justifyContent="flex-start">
            <Tooltip
              title={
                missingEnvVariables.length > 0
                  ? t('summary.import.tooltip.missingVariables', {count: missingEnvVariables.length})
                  : !configContent
                    ? t('summary.import.tooltip.configUnavailable')
                    : effectiveDryRunStatus !== 'passed'
                      ? t('summary.import.tooltip.runTestFirst')
                      : ''
              }
              arrow
              placement="top"
            >
              <span>
                <Button
                  variant="contained"
                  onClick={handleProceed}
                  disabled={
                    missingEnvVariables.length > 0 ||
                    !configContent ||
                    effectiveDryRunStatus !== 'passed' ||
                    importMutation.isPending
                  }
                >
                  {importMutation.isPending ? t('summary.import.importing') : t('summary.import.action')}
                </Button>
              </span>
            </Tooltip>
          </Stack>
        </Box>
      </Box>
    </Box>
  );
}
