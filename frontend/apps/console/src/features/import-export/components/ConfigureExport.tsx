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

import {useConfig} from '@thunderid/contexts';
import {useLogger} from '@thunderid/logger/react';
import {Box, Button, Chip, ColorSchemeSVG, IconButton, Paper, Stack, Typography} from '@wso2/oxygen-ui';
import {
  Bell,
  Building,
  Copy,
  FileDown,
  Key,
  Languages,
  Layers,
  LayoutGrid,
  Layout as LayoutIcon,
  Palette,
  Server,
  Terminal,
  UserRoundCog,
  Users,
  UsersRound,
  Workflow,
} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useMemo, useState} from 'react';
import {useTranslation} from 'react-i18next';
import yaml from 'yaml';
import EnvVariablesViewer from './EnvVariablesViewer';
import FileContentViewer from './FileContentViewer';
import HowProductRunInHostedIllustration from './HowProductRunInHostedIllustration';
import ResourceSummaryTable from './ResourceSummaryTable';
import TemplateVariableDisplay from './TemplateVariableDisplay';
import type {ConfigSummaryItem} from '../models/import-configuration';
import getConfigFileName from '../utils/getConfigFileName';
import getEnvFileName from '../utils/getEnvFileName';

/**
 * Props for the {@link ConfigureExport} component.
 *
 * @public
 */
export interface ConfigureExportProps {
  /**
   * YAML configuration resources content
   */
  resources?: string;
  /**
   * Environment variables (.env) content
   */
  environmentVariables?: string;
  /**
   * Resource counts by type from the export API
   */
  resourceCounts?: Record<string, number>;
  /**
   * Whether the export operation is in progress
   */
  isExporting?: boolean;
}

/**
 * Organization Export Summary step component showing a two-column layout
 * with resources table and downloads on the left and run product instructions on the right.
 *
 * @public
 */
export default function ConfigureExport({
  resources = '',
  environmentVariables = '',
  resourceCounts = undefined,
  isExporting = false,
}: ConfigureExportProps): JSX.Element {
  const {t} = useTranslation(['applications', 'importExport']);
  const logger = useLogger('ConfigureExport');
  const {config} = useConfig();
  const productName = config.brand.product_name;
  const configFileName = getConfigFileName(productName);
  const envFileName = getEnvFileName(productName);

  // Expand/collapse state for each resource type
  const [expandedApplications, setExpandedApplications] = useState(false);
  const [expandedFlows, setExpandedFlows] = useState(false);
  const [expandedThemes, setExpandedThemes] = useState(false);
  const [expandedUsers, setExpandedUsers] = useState(false);
  const [expandedIdps, setExpandedIdps] = useState(false);
  const [expandedOrgUnits, setExpandedOrgUnits] = useState(false);
  const [expandedSenders, setExpandedSenders] = useState(false);
  const [expandedSchemas, setExpandedSchemas] = useState(false);
  const [expandedTranslations, setExpandedTranslations] = useState(false);
  const [expandedLayouts, setExpandedLayouts] = useState(false);
  const [expandedResourceServers, setExpandedResourceServers] = useState(false);
  const [expandedRoles, setExpandedRoles] = useState(false);
  const [expandedGroups, setExpandedGroups] = useState(false);

  const nextSteps = [
    t('importExport:configureExport.nextSteps.startWithConfig', {productName}),
    t('importExport:configureExport.nextSteps.resourcesAvailable'),
    t('importExport:configureExport.nextSteps.testFlows'),
  ];

  const handleCopyCommand = (): void => {
    navigator.clipboard.writeText(command).catch(() => {
      // Handle copy error silently
    });
  };

  const handleDownloadConfiguration = async (): Promise<void> => {
    if (!resources) return;

    try {
      // Try to use File System Access API for "Save As" dialog
      if ('showSaveFilePicker' in window) {
        const handle = await (
          window as Window & {
            showSaveFilePicker: (options?: {
              suggestedName?: string;
              types?: {description: string; accept: Record<string, string[]>}[];
            }) => Promise<{
              createWritable: () => Promise<{write: (data: string) => Promise<void>; close: () => Promise<void>}>;
            }>;
          }
        ).showSaveFilePicker({
          suggestedName: configFileName,
          types: [
            {
              description: 'YAML Configuration',
              accept: {'text/yaml': ['.yml', '.yaml']},
            },
          ],
        });
        const writable = await handle.createWritable();
        await writable.write(resources);
        await writable.close();
      } else {
        // Fallback to traditional download
        const blob = new Blob([resources], {type: 'text/plain;charset=utf-8'});
        const url = URL.createObjectURL(blob);
        const link = document.createElement('a');
        link.href = url;
        link.download = configFileName;
        document.body.appendChild(link);
        link.click();
        document.body.removeChild(link);
        URL.revokeObjectURL(url);
      }
    } catch {
      // User cancelled or error occurred, ignore silently
    }
  };

  // Parse YAML resources to extract configuration data (fallback if resourceCounts not provided)
  const configData = useMemo(() => {
    if (!resources) return null;

    try {
      const sections = resources.split(/^---$/m);
      const resourcesByType: Record<string, unknown[]> = {};

      sections.forEach((section, idx) => {
        const trimmedSection = section.trim();
        if (!trimmedSection) return;

        const lines = trimmedSection.split(/\r?\n|\r/);
        let resourceType = 'unknown';
        let fileName = 'unknown';

        // Extract resource type and file name from comments
        for (const line of lines) {
          if (line.startsWith('#')) {
            const fileNameRegex = /File:\s*(.+\.ya?ml)/i;
            const fileNameMatch = fileNameRegex.exec(line);
            if (fileNameMatch) {
              fileName = fileNameMatch[1];
            }

            const resourceTypeRegex = /resource_type:\s*(\w+)/;
            const resourceTypeMatch = resourceTypeRegex.exec(line);
            if (resourceTypeMatch) {
              resourceType = resourceTypeMatch[1];
            }
          }
        }

        // Extract non-comment lines for YAML parsing
        const yamlLines = lines.filter((line) => {
          const trimmed = line.trim();
          // Skip comments, empty lines, and template block directives
          return (
            trimmed !== '' &&
            !trimmed.startsWith('#') &&
            !trimmed.startsWith('{{-') && // Skip template block directives (e.g., {{- range}}, {{- end}})
            trimmed !== '{{- end}}' &&
            trimmed !== '{{- end }}'
          );
        });

        // Replace template variables with placeholder strings to prevent YAML parsing errors
        // Templates like {{ .VAR }} or {{ t(...) }} are interpreted as objects by YAML parser
        const yamlContent = yamlLines
          .map((line) => {
            let processedLine = line;

            // Quote template variables in two scenarios:
            // 1. Standalone values: key: {{.VAR}} → key: "{{.VAR}}"
            processedLine = processedLine.replace(/:\s*(\{\{[^}]+\}\})(\s*)$/g, ': "$1"$2');

            // 2. Array items: - {{.}} → - "{{.}}"
            processedLine = processedLine.replace(/^(\s*-\s+)(\{\{[^}]+\}\})(\s*)$/g, '$1"$2"$3');

            return processedLine;
          })
          .join('\n');

        if (!yamlContent) return;

        try {
          const resource = yaml.parse(yamlContent) as unknown;
          if (resource && typeof resource === 'object') {
            if (!resourcesByType[resourceType]) {
              resourcesByType[resourceType] = [];
            }
            resourcesByType[resourceType].push(resource);
          }
        } catch (error) {
          logger.warn('Failed to parse YAML section', {
            fileName,
            resourceType,
            sectionIndex: idx,
            error: error instanceof Error ? error.message : String(error),
            yamlPreview: yamlContent.substring(0, 200),
          });
        }
      });

      return resourcesByType;
    } catch (error) {
      logger.error('Failed to parse export resources', {error});
      return null;
    }
  }, [resources, logger]);

  const command = t('howSolutionWorksIllustration:commandProduction');

  // Use resourceCounts from API if available, otherwise fall back to parsing
  const applicationsCount =
    resourceCounts?.application ?? (Array.isArray(configData?.application) ? configData.application.length : 0);
  const usersCount = resourceCounts?.user ?? (Array.isArray(configData?.user) ? configData.user.length : 0);
  const flowsCount = resourceCounts?.flow ?? (Array.isArray(configData?.flow) ? configData.flow.length : 0);
  const themesCount = resourceCounts?.theme ?? (Array.isArray(configData?.theme) ? configData.theme.length : 0);
  const identityProvidersCount =
    resourceCounts?.identity_provider ??
    (Array.isArray(configData?.identity_provider) ? configData.identity_provider.length : 0);
  const orgUnitsCount =
    resourceCounts?.organization_unit ??
    (Array.isArray(configData?.organization_unit) ? configData.organization_unit.length : 0);
  const notificationSendersCount =
    resourceCounts?.notification_sender ??
    (Array.isArray(configData?.notification_sender) ? configData.notification_sender.length : 0);
  const userTypesCount =
    resourceCounts?.user_type ?? (Array.isArray(configData?.user_type) ? configData.user_type.length : 0);
  const translationsCount =
    resourceCounts?.translation ?? (Array.isArray(configData?.translation) ? configData.translation.length : 0);
  const layoutsCount = resourceCounts?.layout ?? (Array.isArray(configData?.layout) ? configData.layout.length : 0);
  const resourceServersCount =
    resourceCounts?.resource_server ??
    (Array.isArray(configData?.resource_server) ? configData.resource_server.length : 0);
  const rolesCount = resourceCounts?.role ?? (Array.isArray(configData?.role) ? configData.role.length : 0);
  const groupsCount = resourceCounts?.group ?? (Array.isArray(configData?.group) ? configData.group.length : 0);

  const items: ConfigSummaryItem[] = [];

  // Add applications if present
  if (applicationsCount > 0) {
    const apps =
      (configData?.application as {
        name?: string;
        handle?: string;
        description?: string;
        inbound_auth_config?: {type?: string; config?: {client_id?: string}}[];
        url?: string;
      }[]) ?? [];
    const displayedApps = expandedApplications ? apps : apps.slice(0, 5);
    const remainingCount = apps.length - 5;

    items.push({
      id: 'applications',
      label: t('export.table.applications'),
      icon: <LayoutGrid size={16} />,
      value: applicationsCount,
      status: 'ready',
      dependencyCount: flowsCount + themesCount + identityProvidersCount,
      content: (
        <Box sx={{px: 3, py: 2, bgcolor: 'background.default'}}>
          <Stack spacing={2}>
            <Stack spacing={2} divider={<Box sx={{borderBottom: 1, borderColor: 'divider'}} />}>
              {displayedApps.map((app, idx) => {
                const clientId = app.inbound_auth_config?.find((cfg) => cfg.type === 'oauth2')?.config?.client_id;
                const appKey = app.name ?? clientId ?? `app-${idx}`;
                return (
                  <Stack key={appKey} spacing={0.5}>
                    <Stack direction="row" spacing={1} sx={{alignItems: 'center'}}>
                      <LayoutGrid size={14} />
                      <Typography variant="body2" fontWeight={600}>
                        {app.name ?? t('importExport:configureExport.fallback.unnamedApplication')}
                      </Typography>
                    </Stack>
                    {app.description && (
                      <Typography variant="caption" color="text.secondary" sx={{pl: 2.5}}>
                        {app.description}
                      </Typography>
                    )}
                    {clientId && (
                      <Box sx={{pl: 2.5}}>
                        <TemplateVariableDisplay
                          text={clientId}
                          envData={environmentVariables}
                          label={t('importExport:configureExport.labels.clientId')}
                        />
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
                      ? t('importExport:configureExport.actions.showLess')
                      : t('importExport:configureExport.actions.more', {count: remainingCount})
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

  // Add identity providers if present
  if (identityProvidersCount > 0) {
    const idps = (configData?.identity_provider as {name?: string; handle?: string; type?: string}[]) ?? [];
    const displayedIdps = expandedIdps ? idps : idps.slice(0, 5);
    const remainingCount = idps.length - 5;

    items.push({
      id: 'integrations',
      label: t('export.table.integrations'),
      icon: <Layers size={16} />,
      value: identityProvidersCount,
      status: 'ready',
      dependencyCount: 0,
      content: (
        <Box sx={{px: 3, py: 2, bgcolor: 'background.default'}}>
          <Stack spacing={2}>
            <Stack spacing={2} divider={<Box sx={{borderBottom: 1, borderColor: 'divider'}} />}>
              {displayedIdps.map((idp, idx) => (
                <Stack key={idp.handle ?? idp.name ?? `idp-${idx}`} spacing={0.5}>
                  <Stack direction="row" spacing={1} sx={{alignItems: 'center'}}>
                    <Layers size={14} />
                    <Typography variant="body2" fontWeight={600}>
                      {idp.name ?? t('importExport:configureExport.fallback.unnamedProvider')}
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
                      ? t('importExport:configureExport.actions.showLess')
                      : t('importExport:configureExport.actions.more', {count: remainingCount})
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

  // Add flows if present
  if (flowsCount > 0) {
    const flows = (configData?.flow as {name?: string; handle?: string; flowType?: string}[]) ?? [];
    const displayedFlows = expandedFlows ? flows : flows.slice(0, 5);
    const remainingCount = flows.length - 5;

    items.push({
      id: 'flows',
      label: t('export.table.flows'),
      icon: <Workflow size={16} />,
      value: flowsCount,
      status: 'ready',
      dependencyCount: identityProvidersCount,
      content: (
        <Box sx={{px: 3, py: 2, bgcolor: 'background.default'}}>
          <Stack spacing={2}>
            <Stack spacing={2} divider={<Box sx={{borderBottom: 1, borderColor: 'divider'}} />}>
              {displayedFlows.map((flow, idx) => (
                <Stack key={flow.handle ?? flow.name ?? `flow-${idx}`} spacing={0.5}>
                  <Stack direction="row" spacing={1} sx={{alignItems: 'center'}}>
                    <Workflow size={14} />
                    <Typography variant="body2" fontWeight={600}>
                      {flow.name ?? flow.handle ?? t('importExport:configureExport.fallback.unnamedFlow')}
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
                      ? t('importExport:configureExport.actions.showLess')
                      : t('importExport:configureExport.actions.more', {count: remainingCount})
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

  // Add themes if present
  if (themesCount > 0) {
    const themes = (configData?.theme as {name?: string; handle?: string; description?: string}[]) ?? [];
    const displayedThemes = expandedThemes ? themes : themes.slice(0, 5);
    const remainingCount = themes.length - 5;

    items.push({
      id: 'themes',
      label: t('importExport:configureExport.labels.themes'),
      icon: <Palette size={16} />,
      value: themesCount,
      status: 'ready',
      dependencyCount: 0,
      content: (
        <Box sx={{px: 3, py: 2, bgcolor: 'background.default'}}>
          <Stack spacing={2}>
            <Stack spacing={2} divider={<Box sx={{borderBottom: 1, borderColor: 'divider'}} />}>
              {displayedThemes.map((theme, idx) => (
                <Stack key={theme.handle ?? theme.name ?? `theme-${idx}`} spacing={0.5}>
                  <Stack direction="row" spacing={1} sx={{alignItems: 'center'}}>
                    <Palette size={14} />
                    <Typography variant="body2" fontWeight={600}>
                      {theme.name ?? theme.handle ?? t('importExport:configureExport.fallback.unnamedTheme')}
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
                      ? t('importExport:configureExport.actions.showLess')
                      : t('importExport:configureExport.actions.more', {count: remainingCount})
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

  // Add users if present
  if (usersCount > 0) {
    const users =
      (configData?.user as {type?: string; attributes?: {name?: string; username?: string; email?: string}}[]) ?? [];
    const displayedUsers = expandedUsers ? users : users.slice(0, 5);
    const remainingCount = users.length - 5;

    items.push({
      id: 'users',
      label: t('importExport:configureExport.labels.users'),
      icon: <UsersRound size={16} />,
      value: usersCount,
      status: 'ready',
      dependencyCount: 0,
      content: (
        <Box sx={{px: 3, py: 2, bgcolor: 'background.default'}}>
          <Stack spacing={2}>
            <Stack spacing={2} divider={<Box sx={{borderBottom: 1, borderColor: 'divider'}} />}>
              {displayedUsers.map((user, idx) => (
                <Stack key={user.attributes?.username ?? user.attributes?.email ?? `user-${idx}`} spacing={0.5}>
                  <Stack direction="row" spacing={1} sx={{alignItems: 'center'}}>
                    <UsersRound size={14} />
                    <Typography variant="body2" fontWeight={600}>
                      {user.attributes?.name ??
                        user.attributes?.username ??
                        user.attributes?.email ??
                        t('importExport:configureExport.fallback.unnamedUser', {index: idx + 1})}
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
                      ? t('importExport:configureExport.actions.showLess')
                      : t('importExport:configureExport.actions.more', {count: remainingCount})
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

  // Add organization units if present
  if (orgUnitsCount > 0) {
    const orgUnits = (configData?.organization_unit as {name?: string; handle?: string; description?: string}[]) ?? [];
    const displayedOrgUnits = expandedOrgUnits ? orgUnits : orgUnits.slice(0, 5);
    const remainingCount = orgUnits.length - 5;

    items.push({
      id: 'organization-units',
      label: t('importExport:configureExport.labels.organizationUnits'),
      icon: <Building size={16} />,
      value: orgUnitsCount,
      status: 'ready',
      dependencyCount: 0,
      content: (
        <Box sx={{px: 3, py: 2, bgcolor: 'background.default'}}>
          <Stack spacing={2}>
            <Stack spacing={2} divider={<Box sx={{borderBottom: 1, borderColor: 'divider'}} />}>
              {displayedOrgUnits.map((org, idx) => (
                <Stack key={org.handle ?? org.name ?? `org-${idx}`} spacing={0.5}>
                  <Stack direction="row" spacing={1} sx={{alignItems: 'center'}}>
                    <Building size={14} />
                    <Typography variant="body2" fontWeight={600}>
                      {org.name ?? org.handle ?? t('importExport:configureExport.fallback.unnamedOrganization')}
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
                      ? t('importExport:configureExport.actions.showLess')
                      : t('importExport:configureExport.actions.more', {count: remainingCount})
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

  // Add notification senders if present
  if (notificationSendersCount > 0) {
    const senders = (configData?.notification_sender as {name?: string; handle?: string; type?: string}[]) ?? [];
    const displayedSenders = expandedSenders ? senders : senders.slice(0, 5);
    const remainingCount = senders.length - 5;

    items.push({
      id: 'notification-senders',
      label: t('importExport:configureExport.labels.notificationSenders'),
      icon: <Bell size={16} />,
      value: notificationSendersCount,
      status: 'ready',
      dependencyCount: 0,
      content: (
        <Box sx={{px: 3, py: 2, bgcolor: 'background.default'}}>
          <Stack spacing={2}>
            <Stack spacing={2} divider={<Box sx={{borderBottom: 1, borderColor: 'divider'}} />}>
              {displayedSenders.map((sender, idx) => (
                <Stack key={sender.handle ?? sender.name ?? `sender-${idx}`} spacing={0.5}>
                  <Stack direction="row" spacing={1} sx={{alignItems: 'center'}}>
                    <Bell size={14} />
                    <Typography variant="body2" fontWeight={600}>
                      {sender.name ?? t('importExport:configureExport.fallback.unnamedSender')}
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
                      ? t('importExport:configureExport.actions.showLess')
                      : t('importExport:configureExport.actions.more', {count: remainingCount})
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

  // Add user types if present
  if (userTypesCount > 0) {
    const schemas =
      (configData?.user_type as {name?: string; handle?: string; allow_self_registration?: boolean}[]) ?? [];
    const displayedSchemas = expandedSchemas ? schemas : schemas.slice(0, 5);
    const remainingCount = schemas.length - 5;

    items.push({
      id: 'user-types',
      label: t('importExport:configureExport.labels.userTypes'),
      icon: <UserRoundCog size={16} />,
      value: userTypesCount,
      status: 'ready',
      dependencyCount: 0,
      content: (
        <Box sx={{px: 3, py: 2, bgcolor: 'background.default'}}>
          <Stack spacing={2}>
            <Stack spacing={2} divider={<Box sx={{borderBottom: 1, borderColor: 'divider'}} />}>
              {displayedSchemas.map((schema, idx) => (
                <Stack key={schema.handle ?? schema.name ?? `schema-${idx}`} spacing={0.5}>
                  <Stack direction="row" spacing={1} sx={{alignItems: 'center'}}>
                    <UserRoundCog size={14} />
                    <Typography variant="body2" fontWeight={600}>
                      {schema.name ?? schema.handle ?? t('importExport:configureExport.fallback.unnamedSchema')}
                    </Typography>
                    {schema.allow_self_registration && (
                      <Chip
                        label={t('importExport:configureExport.labels.selfRegistration')}
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
                      ? t('importExport:configureExport.actions.showLess')
                      : t('importExport:configureExport.actions.more', {count: remainingCount})
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

  // Add translations if present
  if (translationsCount > 0) {
    const translations = (configData?.translation as {locale?: string; namespace?: string}[]) ?? [];
    const displayedTranslations = expandedTranslations ? translations : translations.slice(0, 5);
    const remainingCount = translations.length - 5;

    items.push({
      id: 'translations',
      label: t('importExport:configureExport.labels.translations'),
      icon: <Languages size={16} />,
      value: translationsCount,
      status: 'ready',
      dependencyCount: 0,
      content: (
        <Box sx={{px: 3, py: 2, bgcolor: 'background.default'}}>
          <Stack spacing={2}>
            <Stack spacing={2} divider={<Box sx={{borderBottom: 1, borderColor: 'divider'}} />}>
              {displayedTranslations.map((translation, idx) => (
                <Stack key={translation.locale ?? `translation-${idx}`} spacing={0.5}>
                  <Stack direction="row" spacing={1} sx={{alignItems: 'center'}}>
                    <Languages size={14} />
                    <Typography variant="body2" fontWeight={600}>
                      {translation.locale ?? t('importExport:configureExport.fallback.unnamedTranslation')}
                    </Typography>
                    {translation.namespace && (
                      <Chip
                        label={translation.namespace}
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
                      ? t('importExport:configureExport.actions.showLess')
                      : t('importExport:configureExport.actions.more', {count: remainingCount})
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

  // Add layouts if present
  if (layoutsCount > 0) {
    const layouts = (configData?.layout as {name?: string; handle?: string; description?: string}[]) ?? [];
    const displayedLayouts = expandedLayouts ? layouts : layouts.slice(0, 5);
    const remainingCount = layouts.length - 5;

    items.push({
      id: 'layouts',
      label: t('importExport:configureExport.labels.layouts'),
      icon: <LayoutIcon size={16} />,
      value: layoutsCount,
      status: 'ready',
      dependencyCount: 0,
      content: (
        <Box sx={{px: 3, py: 2, bgcolor: 'background.default'}}>
          <Stack spacing={2}>
            <Stack spacing={2} divider={<Box sx={{borderBottom: 1, borderColor: 'divider'}} />}>
              {displayedLayouts.map((layout, idx) => (
                <Stack key={layout.handle ?? layout.name ?? `layout-${idx}`} spacing={0.5}>
                  <Stack direction="row" spacing={1} sx={{alignItems: 'center'}}>
                    <LayoutIcon size={14} />
                    <Typography variant="body2" fontWeight={600}>
                      {layout.name ?? layout.handle ?? t('importExport:configureExport.fallback.unnamedLayout')}
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
                      ? t('importExport:configureExport.actions.showLess')
                      : t('importExport:configureExport.actions.more', {count: remainingCount})
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

  // Add resource servers if present
  if (resourceServersCount > 0) {
    const resourceServers =
      (configData?.resource_server as {name?: string; handle?: string; description?: string}[]) ?? [];
    const displayedResourceServers = expandedResourceServers ? resourceServers : resourceServers.slice(0, 5);
    const remainingCount = resourceServers.length - 5;

    items.push({
      id: 'resource-servers',
      label: t('importExport:configureExport.labels.resourceServers'),
      icon: <Server size={16} />,
      value: resourceServersCount,
      status: 'ready',
      dependencyCount: 0,
      content: (
        <Box sx={{px: 3, py: 2, bgcolor: 'background.default'}}>
          <Stack spacing={2}>
            <Stack spacing={2} divider={<Box sx={{borderBottom: 1, borderColor: 'divider'}} />}>
              {displayedResourceServers.map((rs, idx) => (
                <Stack key={rs.handle ?? rs.name ?? `rs-${idx}`} spacing={0.5}>
                  <Stack direction="row" spacing={1} sx={{alignItems: 'center'}}>
                    <Server size={14} />
                    <Typography variant="body2" fontWeight={600}>
                      {rs.name ?? rs.handle ?? t('importExport:configureExport.fallback.unnamedResourceServer')}
                    </Typography>
                  </Stack>
                  {rs.description && (
                    <Typography variant="caption" color="text.secondary" sx={{pl: 2.5}}>
                      {rs.description}
                    </Typography>
                  )}
                  {rs.handle && rs.name !== rs.handle && (
                    <Typography variant="caption" color="text.secondary" sx={{pl: 2.5, fontFamily: 'monospace'}}>
                      {rs.handle}
                    </Typography>
                  )}
                </Stack>
              ))}
            </Stack>
            {remainingCount > 0 && (
              <Box sx={{pt: 1, textAlign: 'center'}}>
                <Chip
                  label={
                    expandedResourceServers
                      ? t('importExport:configureExport.actions.showLess')
                      : t('importExport:configureExport.actions.more', {count: remainingCount})
                  }
                  size="small"
                  variant="outlined"
                  onClick={() => setExpandedResourceServers(!expandedResourceServers)}
                  sx={{cursor: 'pointer'}}
                />
              </Box>
            )}
          </Stack>
        </Box>
      ),
    });
  }

  // Add roles if present
  if (rolesCount > 0) {
    const roles = (configData?.role as {name?: string; handle?: string; description?: string}[]) ?? [];
    const displayedRoles = expandedRoles ? roles : roles.slice(0, 5);
    const remainingCount = roles.length - 5;

    items.push({
      id: 'roles',
      label: t('importExport:configureExport.labels.roles'),
      icon: <Key size={16} />,
      value: rolesCount,
      status: 'ready',
      dependencyCount: 0,
      content: (
        <Box sx={{px: 3, py: 2, bgcolor: 'background.default'}}>
          <Stack spacing={2}>
            <Stack spacing={2} divider={<Box sx={{borderBottom: 1, borderColor: 'divider'}} />}>
              {displayedRoles.map((role, idx) => (
                <Stack key={role.handle ?? role.name ?? `role-${idx}`} spacing={0.5}>
                  <Stack direction="row" spacing={1} sx={{alignItems: 'center'}}>
                    <Key size={14} />
                    <Typography variant="body2" fontWeight={600}>
                      {role.name ?? role.handle ?? t('importExport:configureExport.fallback.unnamedRole')}
                    </Typography>
                  </Stack>
                  {role.description && (
                    <Typography variant="caption" color="text.secondary" sx={{pl: 2.5}}>
                      {role.description}
                    </Typography>
                  )}
                  {role.handle && role.name !== role.handle && (
                    <Typography variant="caption" color="text.secondary" sx={{pl: 2.5, fontFamily: 'monospace'}}>
                      {role.handle}
                    </Typography>
                  )}
                </Stack>
              ))}
            </Stack>
            {remainingCount > 0 && (
              <Box sx={{pt: 1, textAlign: 'center'}}>
                <Chip
                  label={
                    expandedRoles
                      ? t('importExport:configureExport.actions.showLess')
                      : t('importExport:configureExport.actions.more', {count: remainingCount})
                  }
                  size="small"
                  variant="outlined"
                  onClick={() => setExpandedRoles(!expandedRoles)}
                  sx={{cursor: 'pointer'}}
                />
              </Box>
            )}
          </Stack>
        </Box>
      ),
    });
  }

  // Add groups if present
  if (groupsCount > 0) {
    const groups = (configData?.group as {id?: string; name?: string; description?: string}[]) ?? [];
    const displayedGroups = expandedGroups ? groups : groups.slice(0, 5);
    const remainingCount = groups.length - 5;

    items.push({
      id: 'groups',
      label: t('importExport:configureExport.labels.groups'),
      icon: <Users size={16} />,
      value: groupsCount,
      status: 'ready',
      dependencyCount: 0,
      content: (
        <Box sx={{px: 3, py: 2, bgcolor: 'background.default'}}>
          <Stack spacing={2}>
            <Stack spacing={2} divider={<Box sx={{borderBottom: 1, borderColor: 'divider'}} />}>
              {displayedGroups.map((group, idx) => (
                <Stack key={group.id ?? group.name ?? `group-${idx}`} spacing={0.5}>
                  <Stack direction="row" spacing={1} sx={{alignItems: 'center'}}>
                    <Users size={14} />
                    <Typography variant="body2" fontWeight={600}>
                      {group.name ?? t('importExport:configureExport.fallback.unnamedGroup')}
                    </Typography>
                  </Stack>
                  {group.description && (
                    <Typography variant="caption" color="text.secondary" sx={{pl: 2.5}}>
                      {group.description}
                    </Typography>
                  )}
                </Stack>
              ))}
            </Stack>
            {remainingCount > 0 && (
              <Box sx={{pt: 1, textAlign: 'center'}}>
                <Chip
                  label={
                    expandedGroups
                      ? t('importExport:configureExport.actions.showLess')
                      : t('importExport:configureExport.actions.more', {count: remainingCount})
                  }
                  size="small"
                  variant="outlined"
                  onClick={() => setExpandedGroups(!expandedGroups)}
                  sx={{cursor: 'pointer'}}
                />
              </Box>
            )}
          </Stack>
        </Box>
      ),
    });
  }

  return (
    <Stack direction="column" spacing={3} sx={{width: '100%'}}>
      {/* Page title */}
      <Stack direction="column" spacing={0.5}>
        <Typography variant="h3" component="h1">
          {t('export.title')}
        </Typography>
        <Typography variant="body1" color="text.secondary">
          {t('export.subtitle')}
        </Typography>
      </Stack>

      {/* Two-column layout: Main content and Right sidebar */}
      <Stack
        sx={{
          flexDirection: {xs: 'column', lg: 'row'},
          alignItems: {xs: 'stretch', lg: 'flex-start'},
          gap: 3,
        }}
      >
        {/* Left: Resource summary table and downloads */}
        <Box sx={{width: '100%', maxWidth: {lg: 850}, minWidth: 500, flexShrink: 1}}>
          <Stack spacing={2}>
            {/* Project Details */}
            <Paper variant="outlined" sx={{p: 3, borderRadius: 2}}>
              <Stack spacing={2}>
                <Typography variant="h6" fontWeight={600}>
                  {t('importExport:configureExport.labels.projectDetails')}
                </Typography>
                <Box>
                  <Typography variant="caption" color="text.secondary" sx={{mb: 0.5, display: 'block'}}>
                    {t('importExport:configureExport.labels.totalResources')}
                  </Typography>
                  <Typography variant="body2" fontWeight={600}>
                    {items.reduce((sum, item) => sum + (typeof item.value === 'number' ? item.value : 0), 0)}
                  </Typography>
                </Box>
              </Stack>
            </Paper>

            {/* Resources Table */}
            <ResourceSummaryTable items={items} />

            {/* Environment Variables Section */}
            {environmentVariables && (
              <Box>
                <EnvVariablesViewer content={environmentVariables} showDownload fileName={envFileName} />
              </Box>
            )}

            {/* Resources Configuration Section */}
            {resources && (
              <Box>
                <FileContentViewer
                  content={resources}
                  fileName={configFileName}
                  title={t('importExport:configureExport.labels.configurationResources')}
                  subtitle={t('importExport:configureExport.labels.downloadConfig', {
                    fileName: configFileName,
                  })}
                  icon={<FileDown size={18} />}
                  iconBgColor="primary.lighter"
                  iconColor="primary.main"
                  showDownload={false}
                />

                <Box sx={{mt: 3}}>
                  <Button
                    variant="contained"
                    startIcon={<FileDown size={16} />}
                    onClick={() => void handleDownloadConfiguration()}
                    disabled={isExporting}
                    fullWidth
                  >
                    {t('importExport:configureExport.actions.exportConfiguration')}
                  </Button>
                </Box>
              </Box>
            )}
          </Stack>
        </Box>
        {/* Right: Run Product Locally */}
        <Box sx={{width: '100%', flex: {lg: '1 1 0'}, minWidth: 350}}>
          <Paper variant="outlined" sx={{p: 3}}>
            <Stack spacing={3}>
              <Stack direction="row" spacing={1.5} sx={{alignItems: 'flex-start'}}>
                <Box sx={{flex: 1}}>
                  <Stack direction="row" spacing={1.5} sx={{alignItems: 'center', mb: 1}}>
                    <Box
                      sx={{
                        width: 32,
                        height: 32,
                        borderRadius: 1,
                        bgcolor: 'primary.main',
                        color: '#fff',
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'center',
                        flexShrink: 0,
                      }}
                    >
                      <Terminal size={18} />
                    </Box>
                    <Typography variant="h6" fontWeight={600}>
                      {t('importExport:configureExport.runProduct.title', {productName})}
                    </Typography>
                  </Stack>
                  <Typography variant="body2" color="text.secondary" mb={2}>
                    {t('importExport:configureExport.runProduct.subtitle', {productName})}
                  </Typography>

                  {/* Illustration */}
                  <Box sx={{mt: 5, mb: 6, display: 'flex', justifyContent: 'center', overflow: 'auto'}}>
                    <ColorSchemeSVG
                      svg={HowProductRunInHostedIllustration}
                      sx={{
                        width: '100%',
                        minWidth: '240px',
                        maxWidth: {xs: '280px', sm: '350px', md: '400px'},
                        height: 'auto',
                      }}
                    />
                  </Box>

                  <Box
                    sx={{
                      bgcolor: 'action.hover',
                      borderRadius: 1,
                      p: 2,
                      fontFamily: 'monospace',
                      position: 'relative',
                      border: '1px solid',
                      borderColor: 'divider',
                    }}
                  >
                    <Stack direction="row" sx={{alignItems: 'center', justifyContent: 'space-between'}}>
                      <Typography
                        variant="body2"
                        sx={{
                          fontFamily: 'monospace',
                          color: 'text.primary',
                        }}
                      >
                        {command}
                      </Typography>
                      <IconButton size="small" onClick={handleCopyCommand} sx={{ml: 2, flexShrink: 0}}>
                        <Copy size={16} />
                      </IconButton>
                    </Stack>
                  </Box>
                </Box>
              </Stack>

              <Stack spacing={1.5}>
                <Typography variant="subtitle2" fontWeight={600}>
                  {t('importExport:configureExport.runProduct.nextStepsTitle')}
                </Typography>
                <Stack spacing={1} sx={{pl: 2}}>
                  {nextSteps.map((step) => (
                    <Stack key={step} direction="row" spacing={1.5}>
                      <Typography variant="body2" color="text.secondary">
                        •
                      </Typography>
                      <Typography variant="body2" color="text.secondary">
                        {step}
                      </Typography>
                    </Stack>
                  ))}
                </Stack>
              </Stack>
            </Stack>
          </Paper>
        </Box>
      </Stack>
    </Stack>
  );
}
