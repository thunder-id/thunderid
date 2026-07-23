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
  Button,
  Chip,
  Divider,
  IconButton,
  LinearProgress,
  Paper,
  Stack,
  Tooltip,
  Typography,
  AppBreadcrumbs,
} from '@wso2/oxygen-ui';
import {
  Bot,
  Building,
  IdCard,
  Key,
  Languages,
  LayoutGrid,
  Layout as LayoutIcon,
  Layers,
  Palette,
  Server,
  Settings,
  ShieldCheck,
  Upload,
  UserRoundCog,
  Users,
  UsersRound,
  Workflow,
  X,
} from '@wso2/oxygen-ui-icons-react';
import type {TFunction} from 'i18next';
import type {ComponentType, JSX} from 'react';
import {useCallback, useEffect, useMemo, useRef, useState} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate, useLocation} from 'react-router';
import RouteConfig from '../../../configs/RouteConfig';
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

interface ResourceItem {
  id?: string;
  name?: string;
  handle?: string;
  description?: string;
  url?: string;
  type?: string;
  flowType?: string;
  locale?: string;
  namespace?: string;
  allow_self_registration?: boolean;
  displayName?: string;
  vct?: string;
  display?: {name?: string};
  attributes?: {username?: string; email?: string; name?: string};
  inbound_auth_config?: {type?: string; config?: {client_id?: string}}[];
}

interface ResourceView {
  type: string;
  id: string;
  icon: ComponentType<{size?: number}>;
  getLabel: (t: TFunction) => string;
  getKey: (item: ResourceItem, idx: number) => string;
  getName: (item: ResourceItem, t: TFunction, idx: number) => string;
  renderChip?: (item: ResourceItem, t: TFunction) => JSX.Element | null;
  renderDetails?: (item: ResourceItem, t: TFunction, envData: string | null) => JSX.Element | null;
}

function detailLine(text: string, mono = false): JSX.Element {
  return (
    <Typography variant="caption" color="text.secondary" sx={mono ? {pl: 2.5, fontFamily: 'monospace'} : {pl: 2.5}}>
      {text}
    </Typography>
  );
}

function smallChip(label: string): JSX.Element {
  return <Chip label={label} size="small" sx={{height: 18, fontSize: '0.65rem'}} />;
}

function getClientId(item: ResourceItem): string | undefined {
  return item.inbound_auth_config?.find((cfg) => cfg.type === 'oauth2')?.config?.client_id;
}

const RESOURCE_VIEWS: ResourceView[] = [
  {
    type: 'application',
    id: 'applications',
    icon: LayoutGrid,
    getLabel: (t) => t('export.table.applications'),
    getKey: (item, idx) => item.name ?? getClientId(item) ?? `app-${idx}`,
    getName: (item, t) => item.name ?? t('configureExport.fallback.unnamedApplication'),
    renderDetails: (item, t, envData) => {
      const clientId = getClientId(item);
      return (
        <>
          {item.description && detailLine(item.description)}
          {clientId && (
            <Box sx={{pl: 2.5}}>
              <TemplateVariableDisplay text={clientId} envData={envData} label={t('export.app.clientId')} />
            </Box>
          )}
          {item.url && (
            <Typography variant="caption" color="primary.main" sx={{pl: 2.5}}>
              {item.url}
            </Typography>
          )}
        </>
      );
    },
  },
  {
    type: 'credential_configuration',
    id: 'credential-configurations',
    icon: IdCard,
    getLabel: (t) => t('summary.labels.credentialConfigurations'),
    getKey: (item, idx) => item.handle ?? item.id ?? `credential-config-${idx}`,
    getName: (item, t) =>
      item.display?.name ?? item.handle ?? t('configureExport.fallback.unnamedCredentialConfiguration'),
    renderChip: (item) => (item.vct ? smallChip(item.vct) : null),
    renderDetails: (item) => (item.handle && item.display?.name !== item.handle ? detailLine(item.handle, true) : null),
  },
  {
    type: 'presentation_definition',
    id: 'presentation-definitions',
    icon: ShieldCheck,
    getLabel: (t) => t('summary.labels.presentationDefinitions'),
    getKey: (item, idx) => item.handle ?? item.id ?? `presentation-definition-${idx}`,
    getName: (item, t) =>
      item.displayName ?? item.handle ?? t('configureExport.fallback.unnamedPresentationDefinition'),
    renderChip: (item) => (item.vct ? smallChip(item.vct) : null),
    renderDetails: (item) => (item.handle && item.displayName !== item.handle ? detailLine(item.handle, true) : null),
  },
  {
    type: 'flow',
    id: 'flows',
    icon: Workflow,
    getLabel: (t) => t('export.table.flows'),
    getKey: (item, idx) => item.handle ?? item.name ?? `flow-${idx}`,
    getName: (item, t, idx) => item.name ?? item.handle ?? t('summary.fallback.flow', {index: idx + 1}),
    renderChip: (item) => (item.flowType ? smallChip(item.flowType) : null),
    renderDetails: (item) => (item.handle && item.name !== item.handle ? detailLine(item.handle, true) : null),
  },
  {
    type: 'connection',
    id: 'connections',
    icon: Layers,
    getLabel: (t) => t('importExport:configureExport.labels.connections'),
    getKey: (item, idx) => item.handle ?? item.name ?? `connection-${idx}`,
    getName: (item, t) => item.name ?? t('configureExport.fallback.unnamedProvider'),
    renderChip: (item) => (item.type ? smallChip(item.type) : null),
  },
  {
    type: 'layout',
    id: 'layouts',
    icon: LayoutIcon,
    getLabel: (t) => t('configureExport.labels.layouts'),
    getKey: (item, idx) => item.handle ?? item.name ?? `layout-${idx}`,
    getName: (item, t) => item.name ?? item.handle ?? t('configureExport.fallback.unnamedLayout'),
    renderDetails: (item) => (item.description ? detailLine(item.description) : null),
  },
  {
    type: 'organization_unit',
    id: 'organization-units',
    icon: Building,
    getLabel: (t) => t('configureExport.labels.organizationUnits'),
    getKey: (item, idx) => item.handle ?? item.name ?? `org-${idx}`,
    getName: (item, t) => item.name ?? item.handle ?? t('configureExport.fallback.unnamedOrganization'),
    renderDetails: (item) => (
      <>
        {item.description && detailLine(item.description)}
        {item.handle && item.name !== item.handle && detailLine(item.handle, true)}
      </>
    ),
  },
  {
    type: 'theme',
    id: 'themes',
    icon: Palette,
    getLabel: (t) => t('configureExport.labels.themes'),
    getKey: (item, idx) => item.handle ?? item.name ?? `theme-${idx}`,
    getName: (item, t, idx) => item.name ?? item.handle ?? t('summary.fallback.theme', {index: idx + 1}),
    renderDetails: (item) => (
      <>
        {item.description && detailLine(item.description)}
        {item.handle && item.name !== item.handle && detailLine(item.handle, true)}
      </>
    ),
  },
  {
    type: 'translation',
    id: 'translations',
    icon: Languages,
    getLabel: (t) => t('configureExport.labels.translations'),
    getKey: (item, idx) => item.locale ?? `translation-${idx}`,
    getName: (item, t) => item.locale ?? t('configureExport.fallback.unnamedTranslation'),
    renderChip: (item) =>
      item.namespace ? (
        <Chip label={item.namespace} size="small" variant="outlined" sx={{height: 18, fontSize: '0.65rem'}} />
      ) : null,
  },
  {
    type: 'user',
    id: 'users',
    icon: UsersRound,
    getLabel: (t) => t('configureExport.labels.users'),
    getKey: (item, idx) => item.attributes?.username ?? item.attributes?.email ?? `user-${idx}`,
    getName: (item, t, idx) =>
      item.attributes?.name ??
      item.attributes?.username ??
      item.attributes?.email ??
      t('summary.fallback.user', {index: idx + 1}),
    renderChip: (item) => (item.type ? smallChip(item.type) : null),
    renderDetails: (item) => (
      <>
        {item.attributes?.username && item.attributes.name !== item.attributes.username && (
          <Typography variant="caption" color="text.secondary" sx={{pl: 2.5}}>
            @{item.attributes.username}
          </Typography>
        )}
        {item.attributes?.email && detailLine(item.attributes.email)}
      </>
    ),
  },
  {
    type: 'user_type',
    id: 'user-types',
    icon: UserRoundCog,
    getLabel: (t) => t('configureExport.labels.userTypes'),
    getKey: (item, idx) => item.handle ?? item.name ?? `schema-${idx}`,
    getName: (item, t, idx) => item.name ?? item.handle ?? t('summary.fallback.schema', {index: idx + 1}),
    renderChip: (item, t) =>
      item.allow_self_registration ? (
        <Chip
          label={t('configureExport.labels.selfRegistration')}
          size="small"
          color="success"
          variant="outlined"
          sx={{height: 18, fontSize: '0.65rem'}}
        />
      ) : null,
  },
  {
    type: 'agent',
    id: 'agents',
    icon: Bot,
    getLabel: (t) => t('configureExport.labels.agents'),
    getKey: (item, idx) => item.id ?? item.name ?? `agent-${idx}`,
    getName: (item, t) => item.name ?? t('configureExport.fallback.unnamedAgent'),
    renderDetails: (item, t, envData) => {
      const clientId = getClientId(item);
      return (
        <>
          {item.description && detailLine(item.description)}
          {clientId && (
            <Box sx={{pl: 2.5}}>
              <TemplateVariableDisplay text={clientId} envData={envData} label={t('export.app.clientId')} />
            </Box>
          )}
        </>
      );
    },
  },
  {
    type: 'resource_server',
    id: 'resource-servers',
    icon: Server,
    getLabel: (t) => t('configureExport.labels.resourceServers'),
    getKey: (item, idx) => item.handle ?? item.name ?? `rs-${idx}`,
    getName: (item, t) => item.name ?? item.handle ?? t('configureExport.fallback.unnamedResourceServer'),
    renderDetails: (item) => (
      <>
        {item.description && detailLine(item.description)}
        {item.handle && item.name !== item.handle && detailLine(item.handle, true)}
      </>
    ),
  },
  {
    type: 'role',
    id: 'roles',
    icon: Key,
    getLabel: (t) => t('configureExport.labels.roles'),
    getKey: (item, idx) => item.handle ?? item.name ?? `role-${idx}`,
    getName: (item, t) => item.name ?? item.handle ?? t('configureExport.fallback.unnamedRole'),
    renderDetails: (item) => (
      <>
        {item.description && detailLine(item.description)}
        {item.handle && item.name !== item.handle && detailLine(item.handle, true)}
      </>
    ),
  },
  {
    type: 'group',
    id: 'groups',
    icon: Users,
    getLabel: (t) => t('configureExport.labels.groups'),
    getKey: (item, idx) => item.id ?? item.name ?? `group-${idx}`,
    getName: (item, t) => item.name ?? t('configureExport.fallback.unnamedGroup'),
    renderDetails: (item) => (item.description ? detailLine(item.description) : null),
  },
  {
    type: 'server_config',
    id: 'server-configs',
    icon: Settings,
    getLabel: (t) => t('configureExport.labels.serverConfigs'),
    getKey: (item, idx) => item.name ?? `server-config-${idx}`,
    getName: (item, t) => item.name ?? t('configureExport.fallback.unnamedServerConfig'),
  },
];

export default function ImportConfigurationSummaryPage(): JSX.Element {
  const {t} = useTranslation('importExport');
  const navigate = useNavigate();
  const location = useLocation();
  const isWelcomeFlow = location.pathname.startsWith(RouteConfig.welcome.root());
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

  // Expand/collapse state keyed by resource type
  const [expandedTypes, setExpandedTypes] = useState<Record<string, boolean>>({});

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

  const buildSummaryItem = (view: ResourceView, items: ResourceItem[]): ConfigSummaryItem => {
    const isExpanded = expandedTypes[view.type] ?? false;
    const displayed = isExpanded ? items : items.slice(0, 5);
    const remainingCount = items.length - 5;
    const Icon = view.icon;

    return {
      id: view.id,
      icon: <Icon size={16} />,
      label: view.getLabel(t),
      value: items.length,
      content: (
        <Box sx={{px: 3, py: 2, bgcolor: 'background.default'}}>
          <Stack spacing={2}>
            <Stack spacing={2} divider={<Box sx={{borderBottom: 1, borderColor: 'divider'}} />}>
              {displayed.map((item, idx) => (
                <Stack key={view.getKey(item, idx)} spacing={0.5}>
                  <Stack direction="row" alignItems="center" spacing={1}>
                    <Icon size={14} />
                    <Typography variant="body2" fontWeight={600}>
                      {view.getName(item, t, idx)}
                    </Typography>
                    {view.renderChip?.(item, t)}
                  </Stack>
                  {view.renderDetails?.(item, t, envData)}
                </Stack>
              ))}
            </Stack>
            {remainingCount > 0 && (
              <Box sx={{pt: 1, textAlign: 'center'}}>
                <Chip
                  label={
                    isExpanded
                      ? t('configureExport.actions.showLess')
                      : t('configureExport.actions.more', {count: remainingCount})
                  }
                  size="small"
                  variant="outlined"
                  onClick={() => setExpandedTypes((prev) => ({...prev, [view.type]: !isExpanded}))}
                  sx={{cursor: 'pointer'}}
                />
              </Box>
            )}
          </Stack>
        </Box>
      ),
    };
  };

  const toResourceItems = (value: unknown): ResourceItem[] => (Array.isArray(value) ? (value as ResourceItem[]) : []);

  // Render every known resource type with its tailored view, in declaration order.
  RESOURCE_VIEWS.forEach((view) => {
    const items = toResourceItems(configData?.[view.type]);
    if (items.length > 0) {
      summaryItems.push(buildSummaryItem(view, items));
    }
  });

  const handleClose = (): void => {
    void navigate(RouteConfig.home.list());
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

      await navigate(RouteConfig.home.list());
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
          <AppBreadcrumbs
            items={[
              ...(isWelcomeFlow
                ? [
                    {
                      key: 'welcome',
                      label: t('common:welcome.header'),
                      onClick: () => void navigate(RouteConfig.welcome.root()),
                    },
                  ]
                : []),
              {
                key: 'import-configuration',
                label: t('upload.breadcrumb.openProject'),
                onClick: () =>
                  void navigate(
                    isWelcomeFlow
                      ? RouteConfig.welcome.importConfigurationUpload()
                      : RouteConfig.importConfiguration.upload(),
                  ),
              },
              {key: 'summary', label: t('summary.breadcrumb')},
            ]}
          />
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
