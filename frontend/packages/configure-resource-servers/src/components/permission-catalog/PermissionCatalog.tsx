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

import {
  Alert,
  Box,
  Checkbox,
  Chip,
  CircularProgress,
  Collapse,
  IconButton,
  Paper,
  Stack,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import {ChevronDown, ChevronRight} from '@wso2/oxygen-ui-icons-react';
import {useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import McpServerSectionContent from './McpServerSectionContent';
import useGetResourceActions from '../../api/useGetResourceActions';
import useGetResources from '../../api/useGetResources';
import useGetResourceServers from '../../api/useGetResourceServers';
import useGetServerActions from '../../api/useGetServerActions';
import useSubtreePermissions from '../../api/useSubtreePermissions';
import type {Resource, ResourcePermissions, ResourceServer} from '../../models/resource-server';
import {
  getSubtreeSelectionState,
  isPermissionSelected,
  mergePermissions,
  removePermissions,
  togglePermission,
  type SelectionState,
} from '../../utils/permissionSelection';

export interface PermissionCatalogProps {
  selected: ResourcePermissions[];
  onChange: (selected: ResourcePermissions[]) => void;
  readOnly?: boolean;
}

/* ---------- Row ---------- */

export interface CatalogRowProps {
  name: string;
  permission: string;
  depth: number;
  checked: boolean;
  indeterminate?: boolean;
  disabled: boolean;
  loading?: boolean;
  onToggle: () => void;
  expandControl?: JSX.Element;
}

export function CatalogRow({
  name,
  permission,
  depth,
  checked,
  indeterminate = false,
  disabled,
  loading = false,
  onToggle,
  expandControl = undefined,
}: CatalogRowProps): JSX.Element {
  return (
    <Stack direction="row" alignItems="center" spacing={1} sx={{pl: depth * 3, py: 0.25}}>
      <Box sx={{width: 28, display: 'flex', justifyContent: 'center'}}>{expandControl}</Box>
      {loading ? (
        <Box sx={{width: 38, display: 'flex', justifyContent: 'center'}}>
          <CircularProgress size={16} />
        </Box>
      ) : (
        <Checkbox
          size="small"
          checked={checked}
          indeterminate={indeterminate}
          disabled={disabled}
          onChange={onToggle}
          inputProps={{'aria-label': permission}}
        />
      )}
      <Typography variant="body2">{name}</Typography>
      <Chip label={permission} size="small" sx={{fontFamily: 'monospace'}} />
    </Stack>
  );
}

/* ---------- Resource node (recursive, tri-state) ---------- */

export interface CatalogResourceNodeProps {
  resourceServerId: string;
  resource: Resource;
  depth: number;
  selected: ResourcePermissions[];
  readOnly: boolean;
  delimiter: string;
  onChange: (selected: ResourcePermissions[]) => void;
  collectSubtree: (resource: Resource) => Promise<string[]>;
  getCachedSubtree: (resource: Resource) => string[] | null;
}

export function CatalogResourceNode({
  resourceServerId,
  resource,
  depth,
  selected,
  readOnly,
  delimiter,
  onChange,
  collectSubtree,
  getCachedSubtree,
}: CatalogResourceNodeProps): JSX.Element | null {
  const {t} = useTranslation();
  const [expanded, setExpanded] = useState(false);
  const [cascading, setCascading] = useState(false);
  const [cascadeError, setCascadeError] = useState(false);
  const isOpen = expanded;
  const {data: childResourcesData} = useGetResources(resourceServerId, resource.id, isOpen);
  const {data: resourceActionsData} = useGetResourceActions(resourceServerId, resource.id, isOpen);

  const childResources = childResourcesData?.resources ?? [];
  const resourceActions = resourceActionsData?.actions ?? [];

  const cached = getCachedSubtree(resource);
  let state: SelectionState;
  if (cached !== null) {
    state = getSubtreeSelectionState(selected, resourceServerId, cached);
  } else {
    const serverEntry = selected.find((e) => e.resourceServerId === resourceServerId);
    const anyUnder =
      serverEntry?.permissions.some(
        (p) => p === resource.permission || p.startsWith(`${resource.permission}${delimiter}`),
      ) ?? false;
    state = anyUnder ? 'some' : 'none';
  }

  const handleCascadeToggle = (): void => {
    if (state === 'all' && cached !== null) {
      onChange(removePermissions(selected, resourceServerId, cached));
      return;
    }
    setCascading(true);
    setCascadeError(false);
    collectSubtree(resource)
      .then((all) => {
        onChange(mergePermissions(selected, [{resourceServerId, permissions: all}]));
        setCascading(false);
      })
      .catch(() => {
        setCascadeError(true);
        setCascading(false);
      });
  };

  return (
    <>
      <CatalogRow
        name={resource.name}
        permission={resource.permission}
        depth={depth}
        checked={state === 'all'}
        indeterminate={state === 'some'}
        disabled={readOnly}
        loading={cascading}
        onToggle={handleCascadeToggle}
        expandControl={
          <IconButton size="small" onClick={() => setExpanded((v) => !v)} aria-label={resource.handle}>
            {isOpen ? <ChevronDown size={16} /> : <ChevronRight size={16} />}
          </IconButton>
        }
      />
      {cascadeError && (
        <Alert severity="error" sx={{mx: 2, my: 0.5}}>
          {t('resourceServers:permissionCatalog.loadError', 'Failed to load permissions for this resource server.')}
        </Alert>
      )}
      <Collapse in={isOpen}>
        {resourceActions.map((action) => (
          <CatalogRow
            key={action.id}
            name={action.name}
            permission={action.permission}
            depth={depth + 1}
            checked={isPermissionSelected(selected, resourceServerId, action.permission)}
            disabled={readOnly}
            onToggle={() => onChange(togglePermission(selected, resourceServerId, action.permission))}
          />
        ))}
        {childResources.map((child) => (
          <CatalogResourceNode
            key={child.id}
            resourceServerId={resourceServerId}
            resource={child}
            depth={depth + 1}
            selected={selected}
            readOnly={readOnly}
            delimiter={delimiter}
            onChange={onChange}
            collectSubtree={collectSubtree}
            getCachedSubtree={getCachedSubtree}
          />
        ))}
      </Collapse>
    </>
  );
}

/* ---------- Server section ---------- */

interface ServerSectionProps {
  server: ResourceServer;
  selected: ResourcePermissions[];
  readOnly: boolean;
  onChange: (selected: ResourcePermissions[]) => void;
}

export interface ServerSectionContentProps extends ServerSectionProps {
  collectSubtree: (resource: Resource) => Promise<string[]>;
  getCachedSubtree: (resource: Resource) => string[] | null;
}

function DefaultServerSectionContent({
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

  return (
    <>
      {serverActions.map((action) => (
        <CatalogRow
          key={action.id}
          name={action.name}
          permission={action.permission}
          depth={1}
          checked={isPermissionSelected(selected, server.id, action.permission)}
          disabled={readOnly}
          onToggle={() => onChange(togglePermission(selected, server.id, action.permission))}
        />
      ))}
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

const SERVER_SECTION_CONTENT_BY_TYPE: Record<
  ResourceServer['type'],
  (props: ServerSectionContentProps) => JSX.Element
> = {
  API: DefaultServerSectionContent,
  CUSTOM: DefaultServerSectionContent,
  MCP: McpServerSectionContent,
};

function ServerSectionContent(props: ServerSectionContentProps): JSX.Element {
  const Content = SERVER_SECTION_CONTENT_BY_TYPE[props.server.type];
  return <Content {...props} />;
}

function ServerSection({server, selected, readOnly, onChange}: ServerSectionProps): JSX.Element {
  const {t} = useTranslation();
  const [expanded, setExpanded] = useState(false);
  const [hasExpanded, setHasExpanded] = useState(false);
  const [cascading, setCascading] = useState(false);
  const [cascadeError, setCascadeError] = useState(false);

  const {collectSubtreePermissions, getCachedSubtreePermissions, collectServerPermissions, getCachedServerPermissions} =
    useSubtreePermissions(server.id);

  const serverEntry = selected.find((e) => e.resourceServerId === server.id);
  const selectedCount = serverEntry?.permissions.length ?? 0;

  const cachedAll = getCachedServerPermissions();
  const isEmpty = cachedAll !== null && cachedAll.length === 0 && selectedCount === 0;
  let state: SelectionState;
  if (cachedAll !== null) {
    state = getSubtreeSelectionState(selected, server.id, cachedAll);
    if (state === 'all' && serverEntry?.permissions.some((p) => !cachedAll.includes(p))) {
      state = 'some';
    }
    if (cachedAll.length === 0 && selectedCount > 0) state = 'some';
  } else {
    state = selectedCount > 0 ? 'some' : 'none';
  }

  const handleToggleExpand = (): void => {
    setExpanded((v) => !v);
    setHasExpanded(true);
  };

  const handleCascadeToggle = (): void => {
    if (state === 'all') {
      onChange(selected.filter((e) => e.resourceServerId !== server.id));
      return;
    }
    setCascading(true);
    setCascadeError(false);
    collectServerPermissions()
      .then((all) => {
        onChange(mergePermissions(selected, [{resourceServerId: server.id, permissions: all}]));
        setCascading(false);
      })
      .catch(() => {
        setCascadeError(true);
        setCascading(false);
      });
  };

  return (
    <Box sx={{borderBottom: '1px solid', borderColor: 'divider'}}>
      <Stack direction="row" alignItems="center" spacing={1} sx={{py: 0.5}}>
        <IconButton size="small" onClick={handleToggleExpand} aria-label={server.name}>
          {expanded ? <ChevronDown size={16} /> : <ChevronRight size={16} />}
        </IconButton>
        {cascading ? (
          <Box sx={{width: 38, display: 'flex', justifyContent: 'center'}}>
            <CircularProgress size={16} />
          </Box>
        ) : (
          <Tooltip
            title={
              isEmpty
                ? t(
                    'resourceServers:permissionCatalog.noPermissions',
                    'No permissions defined for this resource server.',
                  )
                : ''
            }
          >
            <Checkbox
              size="small"
              checked={state === 'all'}
              indeterminate={state === 'some'}
              disabled={readOnly || isEmpty}
              onChange={handleCascadeToggle}
              inputProps={{'aria-label': server.name}}
            />
          </Tooltip>
        )}
        <Typography variant="subtitle2">{server.name}</Typography>
        {selectedCount > 0 && <Chip size="small" label={selectedCount} />}
      </Stack>
      {cascadeError && (
        <Alert severity="error" sx={{mx: 2, my: 0.5}}>
          {t('resourceServers:permissionCatalog.loadError', 'Failed to load permissions for this resource server.')}
        </Alert>
      )}
      <Collapse in={expanded}>
        {hasExpanded && (
          <ServerSectionContent
            server={server}
            selected={selected}
            readOnly={readOnly}
            onChange={onChange}
            collectSubtree={collectSubtreePermissions}
            getCachedSubtree={getCachedSubtreePermissions}
          />
        )}
      </Collapse>
    </Box>
  );
}

/* ---------- Unknown servers ---------- */

function UnknownServerGroups({
  entries,
  readOnly,
  selected,
  onChange,
}: {
  entries: ResourcePermissions[];
  readOnly: boolean;
  selected: ResourcePermissions[];
  onChange: (selected: ResourcePermissions[]) => void;
}): JSX.Element {
  const {t} = useTranslation();
  return (
    <>
      {entries.map((entry) => (
        <Box key={entry.resourceServerId} sx={{borderBottom: '1px solid', borderColor: 'divider', py: 0.5}}>
          <Stack direction="row" alignItems="center" spacing={1} sx={{pl: 1}}>
            <Typography variant="subtitle2">{entry.resourceServerId}</Typography>
            <Chip
              size="small"
              color="warning"
              label={t('resourceServers:permissionCatalog.serverNotFound', 'Resource server not found')}
            />
          </Stack>
          {entry.permissions.map((permission) => (
            <CatalogRow
              key={permission}
              name={permission}
              permission={permission}
              depth={1}
              checked
              disabled={readOnly}
              onToggle={() => onChange(togglePermission(selected, entry.resourceServerId, permission))}
            />
          ))}
        </Box>
      ))}
    </>
  );
}

/* ---------- Catalog ---------- */

export default function PermissionCatalog({selected, onChange, readOnly = false}: PermissionCatalogProps): JSX.Element {
  const {t} = useTranslation();

  const {data: serversData, isLoading: loadingServers, error: serversError} = useGetResourceServers({limit: 100});
  const servers = serversData?.resourceServers ?? [];

  if (serversError) {
    return (
      <Alert severity="error">
        {t('resourceServers:permissionCatalog.loadServersError', 'Failed to load resource servers.')}
      </Alert>
    );
  }

  if (loadingServers) {
    return (
      <Box sx={{display: 'flex', justifyContent: 'center', py: 4}}>
        <CircularProgress size={24} />
      </Box>
    );
  }

  const unknownEntries = selected.filter((entry) => !servers.some((s) => s.id === entry.resourceServerId));

  if (servers.length === 0 && unknownEntries.length === 0) {
    return (
      <Alert severity="info">
        {t(
          'resourceServers:permissionCatalog.noResourceServers',
          'No resource servers found. Create a resource server first.',
        )}
      </Alert>
    );
  }

  return (
    <Paper variant="outlined" sx={{p: 2}}>
      {servers.map((server) => (
        <ServerSection key={server.id} server={server} selected={selected} readOnly={readOnly} onChange={onChange} />
      ))}
      <UnknownServerGroups entries={unknownEntries} readOnly={readOnly} selected={selected} onChange={onChange} />
    </Paper>
  );
}
