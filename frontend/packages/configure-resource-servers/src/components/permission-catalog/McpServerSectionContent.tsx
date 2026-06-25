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

import {Alert, Box, CircularProgress, Typography} from '@wso2/oxygen-ui';
import {type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {CatalogResourceNode, CatalogRow, type ServerSectionContentProps} from './PermissionCatalog';
import useGetResources from '../../api/useGetResources';
import useGetServerActions from '../../api/useGetServerActions';
import type {Action, ResourcePermissions} from '../../models/resource-server';
import {isPermissionSelected, togglePermission} from '../../utils/permissionSelection';

interface CatalogSubsectionProps {
  label: string;
  actions: Action[];
  resourceServerId: string;
  selected: ResourcePermissions[];
  readOnly: boolean;
  onChange: (selected: ResourcePermissions[]) => void;
}

function CatalogSubsection({
  label,
  actions,
  resourceServerId,
  selected,
  readOnly,
  onChange,
}: CatalogSubsectionProps): JSX.Element | null {
  if (actions.length === 0) return null;

  return (
    <>
      <Typography
        variant="caption"
        color="text.secondary"
        sx={{
          display: 'block',
          pl: 3,
          py: 0.25,
          textTransform: 'uppercase',
          letterSpacing: 0.5,
        }}
      >
        {label}
      </Typography>
      {actions.map((action) => (
        <CatalogRow
          key={action.id}
          name={action.name}
          permission={action.permission}
          depth={1}
          checked={isPermissionSelected(selected, resourceServerId, action.permission)}
          disabled={readOnly}
          onToggle={() => onChange(togglePermission(selected, resourceServerId, action.permission))}
        />
      ))}
    </>
  );
}

export default function McpServerSectionContent({
  server,
  selected,
  readOnly,
  onChange,
  collectSubtree,
  getCachedSubtree,
}: ServerSectionContentProps): JSX.Element {
  const delimiter = server.delimiter;
  const {t} = useTranslation();
  const {data: resourcesData, isLoading: loadingResources, error: resourcesError} = useGetResources(server.id);
  const {data: actionsData, isLoading: loadingActions, error: actionsError} = useGetServerActions(server.id);

  if (loadingResources || loadingActions) {
    return (
      <Box sx={{display: 'flex', justifyContent: 'center', py: 3}}>
        <CircularProgress size={20} />
      </Box>
    );
  }

  if (resourcesError ?? actionsError) {
    return (
      <Alert severity="error" sx={{my: 1}}>
        {t('resourceServers:permissionCatalog.loadError', 'Failed to load permissions for this resource server.')}
      </Alert>
    );
  }

  const resources = resourcesData?.resources ?? [];
  const serverActions = actionsData?.actions ?? [];

  if (resources.length === 0 && serverActions.length === 0) {
    return (
      <Alert severity="info" sx={{my: 1}}>
        {t('resourceServers:permissionCatalog.noPermissions', 'No permissions defined for this resource server.')}
      </Alert>
    );
  }

  const tools = serverActions.filter((a) => a.kind === 'tool');
  const mcpResources = serverActions.filter((a) => a.kind === 'resource');

  return (
    <>
      <CatalogSubsection
        label={t('resourceServers:permissionCatalog.mcp.tools', 'Tools')}
        actions={tools}
        resourceServerId={server.id}
        selected={selected}
        readOnly={readOnly}
        onChange={onChange}
      />
      <CatalogSubsection
        label={t('resourceServers:permissionCatalog.mcp.resources', 'Resources')}
        actions={mcpResources}
        resourceServerId={server.id}
        selected={selected}
        readOnly={readOnly}
        onChange={onChange}
      />
      {resources.map((resource) => (
        <CatalogResourceNode
          key={resource.id}
          resourceServerId={server.id}
          resource={resource}
          depth={1}
          selected={selected}
          readOnly={readOnly}
          delimiter={delimiter}
          onChange={onChange}
          collectSubtree={collectSubtree}
          getCachedSubtree={getCachedSubtree}
        />
      ))}
    </>
  );
}
