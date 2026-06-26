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
  Box,
  Chip,
  CircularProgress,
  IconButton,
  InputAdornment,
  ListItemIcon,
  ListItemText,
  Menu,
  MenuItem,
  Paper,
  Stack,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import {Database, Folder, Layers, Plus, Search, Wrench, X, Zap} from '@wso2/oxygen-ui-icons-react';
import {useMemo, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import AddNodeDialog, {type AddNodeMode} from './AddNodeDialog';
import ResourceDetailPanel from './ResourceDetailPanel';
import {ActionNode, ResourceNode} from './ResourceTreeNode';
import useGetResources from '../../api/useGetResources';
import useGetServerActions from '../../api/useGetServerActions';
import type {Action, Resource, ResourceServer} from '../../models/resource-server';

export type KindFilter = 'all' | 'tool' | 'resource';

export type SelectedNode =
  | {type: 'server'; id: string; data: ResourceServer}
  | {type: 'resource'; id: string; data: Resource; breadcrumb?: string[]}
  | {type: 'server-action'; id: string; data: Action; parentResourceId?: string; breadcrumb?: string[]}
  | {type: 'resource-action'; id: string; data: Action; parentResourceId?: string; breadcrumb?: string[]};

interface ResourceTreeProps {
  resourceServer: ResourceServer;
  onRefresh: () => void;
}

/* -------------------------------------------------------------------------- */
/*  MCP Capabilities Panel                                                      */
/* -------------------------------------------------------------------------- */

function McpCapabilitiesPanel({resourceServer, onRefresh}: ResourceTreeProps): JSX.Element {
  const {t} = useTranslation();
  const [selectedNode, setSelectedNode] = useState<SelectedNode | null>(null);
  const [addDialog, setAddDialog] = useState<{
    mode: AddNodeMode;
    parentResourceId?: string;
    parentPermission: string;
  } | null>(null);
  const [addMenuAnchor, setAddMenuAnchor] = useState<HTMLElement | null>(null);
  const [kindFilter, setKindFilter] = useState<KindFilter>('all');
  const [searchQuery, setSearchQuery] = useState('');

  const {data: topLevelResources, isLoading: loadingResources} = useGetResources(resourceServer.id);
  const {data: serverActionsData, isLoading: loadingActions} = useGetServerActions(resourceServer.id);

  const namespaces = useMemo(() => topLevelResources?.resources ?? [], [topLevelResources]);
  const serverActions = useMemo(() => serverActionsData?.actions ?? [], [serverActionsData]);

  const serverTools = useMemo(() => serverActions.filter((a) => a.kind === 'tool'), [serverActions]);
  const serverResources = useMemo(() => serverActions.filter((a) => a.kind === 'resource'), [serverActions]);

  const toolCount = serverTools.length;
  const resourceCount = serverResources.length;
  const allCount = toolCount + resourceCount;

  const searchActive = searchQuery.trim().length > 0;
  const searchMatcher = useMemo(() => {
    if (!searchActive) return undefined;
    const q = searchQuery.trim().toLowerCase();
    return (node: {name: string; handle: string}): boolean =>
      node.name.toLowerCase().includes(q) || node.handle.toLowerCase().includes(q);
  }, [searchActive, searchQuery]);

  const openAdd = (mode: AddNodeMode, parentResourceId?: string, parentPermission?: string): void => {
    setAddDialog({
      mode,
      parentResourceId,
      parentPermission: parentPermission ?? resourceServer.handle,
    });
  };

  const isLoading = loadingResources || loadingActions;
  const isEmpty = serverActions.length === 0 && namespaces.length === 0;

  const effectiveSelectedNode = useMemo<SelectedNode | null>(() => {
    if (selectedNode) return selectedNode;
    if (serverTools.length > 0) return {type: 'server-action', id: serverTools[0].id, data: serverTools[0]};
    if (serverResources.length > 0) return {type: 'server-action', id: serverResources[0].id, data: serverResources[0]};
    if (namespaces.length > 0) return {type: 'resource', id: namespaces[0].id, data: namespaces[0]};
    return null;
  }, [selectedNode, serverTools, serverResources, namespaces]);

  const filteredServerActions = useMemo(() => {
    const kindFiltered = serverActions.filter((a) => {
      if (kindFilter === 'tool') return a.kind === 'tool';
      if (kindFilter === 'resource') return a.kind === 'resource';
      return true;
    });
    if (!searchActive || !searchMatcher) return kindFiltered;
    return kindFiltered.filter(searchMatcher);
  }, [serverActions, kindFilter, searchActive, searchMatcher]);

  const noSearchResults = searchActive && filteredServerActions.length === 0 && namespaces.length === 0;

  const formatCount = (count: number, hasMore: boolean): string => `${count}${hasMore ? '+' : ''}`;

  return (
    <Box sx={{display: 'flex', gap: 2, height: '100%'}}>
      {/* Left: Capabilities panel */}
      <Paper
        variant="outlined"
        sx={{
          flex: 1,
          minWidth: 300,
          display: 'flex',
          flexDirection: 'column',
          overflow: 'hidden',
        }}
      >
        {/* Panel header */}
        <Box
          sx={{
            px: 1.5,
            py: 1,
            bgcolor: 'background.default',
            borderBottom: '1px solid',
            borderColor: 'divider',
            display: 'flex',
            justifyContent: 'space-between',
            alignItems: 'center',
          }}
        >
          <Typography
            variant="caption"
            color="text.secondary"
            sx={{textTransform: 'uppercase', letterSpacing: 0.5, fontWeight: 600}}
          >
            {t('resourceServers:mcp.panel.title', 'Capabilities')}
          </Typography>
          <IconButton
            size="small"
            aria-label={t('resourceServers:mcp.add', 'Add')}
            onClick={(e) => setAddMenuAnchor(e.currentTarget)}
          >
            <Plus size={16} />
          </IconButton>
          <Menu anchorEl={addMenuAnchor} open={Boolean(addMenuAnchor)} onClose={() => setAddMenuAnchor(null)}>
            <MenuItem
              onClick={() => {
                openAdd('mcp-server-tool');
                setAddMenuAnchor(null);
              }}
            >
              <ListItemIcon>
                <Wrench size={16} />
              </ListItemIcon>
              <ListItemText>{t('resourceServers:mcp.addTool', 'Add tool')}</ListItemText>
            </MenuItem>
            <MenuItem
              onClick={() => {
                openAdd('mcp-server-resource');
                setAddMenuAnchor(null);
              }}
            >
              <ListItemIcon>
                <Database size={16} />
              </ListItemIcon>
              <ListItemText>{t('resourceServers:mcp.addResource', 'Add resource')}</ListItemText>
            </MenuItem>
            <MenuItem
              onClick={() => {
                openAdd('mcp-namespace');
                setAddMenuAnchor(null);
              }}
            >
              <ListItemIcon>
                <Folder size={16} />
              </ListItemIcon>
              <ListItemText>{t('resourceServers:mcp.addNamespace', 'Add namespace')}</ListItemText>
            </MenuItem>
          </Menu>
        </Box>

        {/* Toolbar: filter chips + search */}
        <Box sx={{px: 1.5, pt: 1, pb: 0.5, borderBottom: '1px solid', borderColor: 'divider'}}>
          <Stack
            direction="row"
            flexWrap="wrap"
            useFlexGap
            gap={0.75}
            role="radiogroup"
            aria-label={t('resourceServers:mcp.filter.label', 'Filter capabilities')}
            sx={{mb: 1}}
          >
            <Chip
              label={`${t('resourceServers:mcp.filter.all', 'All')} · ${isLoading ? '—' : formatCount(allCount, namespaces.length > 0)}`}
              size="small"
              color={kindFilter === 'all' ? 'primary' : 'default'}
              variant={kindFilter === 'all' ? 'filled' : 'outlined'}
              onClick={() => setKindFilter('all')}
              role="radio"
              aria-checked={kindFilter === 'all'}
              disabled={isLoading}
              sx={{cursor: 'pointer'}}
            />
            <Chip
              label={`${t('resourceServers:mcp.filter.tools', 'Tools')} · ${isLoading ? '—' : formatCount(toolCount, namespaces.length > 0)}`}
              size="small"
              color={kindFilter === 'tool' ? 'primary' : 'default'}
              variant={kindFilter === 'tool' ? 'filled' : 'outlined'}
              onClick={() => setKindFilter('tool')}
              role="radio"
              aria-checked={kindFilter === 'tool'}
              disabled={isLoading}
              sx={{cursor: 'pointer'}}
            />
            <Chip
              label={`${t('resourceServers:mcp.filter.resources', 'Resources')} · ${isLoading ? '—' : formatCount(resourceCount, namespaces.length > 0)}`}
              size="small"
              color={kindFilter === 'resource' ? 'primary' : 'default'}
              variant={kindFilter === 'resource' ? 'filled' : 'outlined'}
              onClick={() => setKindFilter('resource')}
              role="radio"
              aria-checked={kindFilter === 'resource'}
              disabled={isLoading}
              sx={{cursor: 'pointer'}}
            />
          </Stack>
          <TextField
            size="small"
            fullWidth
            placeholder={t('resourceServers:mcp.search.placeholder', 'Search capabilities')}
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            disabled={isLoading || isEmpty}
            aria-label={t('resourceServers:mcp.search.placeholder', 'Search capabilities')}
            slotProps={{
              input: {
                startAdornment: (
                  <InputAdornment position="start">
                    <Search size={14} />
                  </InputAdornment>
                ),
                endAdornment: searchQuery ? (
                  <InputAdornment position="end">
                    <IconButton
                      size="small"
                      aria-label={t('resourceServers:mcp.search.clear', 'Clear search')}
                      onClick={() => setSearchQuery('')}
                      edge="end"
                    >
                      <X size={14} />
                    </IconButton>
                  </InputAdornment>
                ) : undefined,
              },
            }}
            sx={{mb: 0.5}}
          />
        </Box>

        {/* Scroll body */}
        <Box role="tree" sx={{flex: 1, overflowY: 'auto', height: '100%'}}>
          {isLoading ? (
            <Box sx={{display: 'flex', justifyContent: 'center', py: 4}}>
              <CircularProgress size={24} />
            </Box>
          ) : isEmpty ? (
            <Box sx={{px: 2, py: 2}}>
              <Typography variant="body2" color="text.disabled" sx={{textAlign: 'center'}}>
                {t('resourceServers:mcp.empty', 'No capabilities yet. Use + to add a tool, resource, or namespace.')}
              </Typography>
            </Box>
          ) : noSearchResults ? (
            <Box sx={{px: 2, py: 2}}>
              <Typography variant="body2" color="text.disabled" sx={{textAlign: 'center'}}>
                {t('resourceServers:mcp.search.noResults', 'No capabilities match "{{query}}".', {
                  query: searchQuery,
                })}
              </Typography>
            </Box>
          ) : (
            <>
              {namespaces.map((ns) => (
                <ResourceNode
                  key={ns.id}
                  resourceServerId={resourceServer.id}
                  delimiter={resourceServer.delimiter}
                  node={ns}
                  depth={0}
                  selectedNodeId={effectiveSelectedNode?.id ?? null}
                  onSelect={setSelectedNode}
                  onAddChild={(mode, parentResourceId, parentPermission) =>
                    openAdd(mode, parentResourceId, parentPermission)
                  }
                  isMcp
                  kindFilter={kindFilter}
                  searchMatcher={searchMatcher}
                  searchActive={searchActive}
                />
              ))}
              {filteredServerActions.map((action) => (
                <ActionNode
                  key={action.id}
                  resourceServerId={resourceServer.id}
                  action={action}
                  depth={0}
                  selectedNodeId={effectiveSelectedNode?.id ?? null}
                  onSelect={setSelectedNode}
                />
              ))}
            </>
          )}
        </Box>
      </Paper>

      {/* Right: Detail Panel */}
      <Paper variant="outlined" sx={{flex: 1, minWidth: 280, overflow: 'hidden'}}>
        <ResourceDetailPanel
          selectedNode={effectiveSelectedNode}
          resourceServer={resourceServer}
          onRefresh={onRefresh}
        />
      </Paper>

      {addDialog && (
        <AddNodeDialog
          open={true}
          mode={addDialog.mode}
          resourceServerId={resourceServer.id}
          parentResourceId={addDialog.parentResourceId}
          parentPermission={addDialog.parentPermission}
          delimiter={resourceServer.delimiter}
          onClose={() => setAddDialog(null)}
          onSuccess={() => setAddDialog(null)}
        />
      )}
    </Box>
  );
}

/* -------------------------------------------------------------------------- */
/*  Generic Resource Tree (API / CUSTOM — unchanged)                           */
/* -------------------------------------------------------------------------- */

function GenericResourceTree({resourceServer, onRefresh}: ResourceTreeProps): JSX.Element {
  const {t} = useTranslation();
  const [selectedNode, setSelectedNode] = useState<SelectedNode | null>(null);
  const [addDialog, setAddDialog] = useState<{
    mode: AddNodeMode;
    parentResourceId?: string;
    parentPermission: string;
  } | null>(null);
  const [addMenuAnchor, setAddMenuAnchor] = useState<HTMLElement | null>(null);

  const {data: topLevelResources, isLoading: loadingResources} = useGetResources(resourceServer.id);
  const {data: serverActionsData, isLoading: loadingActions} = useGetServerActions(resourceServer.id);

  const resources = useMemo(() => topLevelResources?.resources ?? [], [topLevelResources]);
  const serverActions = useMemo(() => serverActionsData?.actions ?? [], [serverActionsData]);

  const openAdd = (mode: AddNodeMode, parentResourceId?: string, parentPermission?: string): void => {
    setAddDialog({
      mode,
      parentResourceId,
      parentPermission: parentPermission ?? resourceServer.handle,
    });
  };

  const isLoading = loadingResources || loadingActions;
  const isEmpty = resources.length === 0 && serverActions.length === 0;

  const effectiveSelectedNode = useMemo<SelectedNode | null>(() => {
    if (selectedNode) return selectedNode;
    if (serverActions.length > 0) return {type: 'server-action', id: serverActions[0].id, data: serverActions[0]};
    if (resources.length > 0) return {type: 'resource', id: resources[0].id, data: resources[0]};
    return null;
  }, [selectedNode, serverActions, resources]);

  return (
    <Box sx={{display: 'flex', gap: 2, height: '100%'}}>
      {/* Left: Tree */}
      <Paper
        variant="outlined"
        sx={{
          flex: 1,
          minWidth: 300,
          display: 'flex',
          flexDirection: 'column',
          overflow: 'hidden',
        }}
      >
        <Box
          sx={{
            px: 1.5,
            py: 1,
            bgcolor: 'background.default',
            borderBottom: '1px solid',
            borderColor: 'divider',
            display: 'flex',
            justifyContent: 'space-between',
            alignItems: 'center',
          }}
        >
          <Typography
            variant="caption"
            color="text.secondary"
            sx={{textTransform: 'uppercase', letterSpacing: 0.5, fontWeight: 600}}
          >
            {t('resourceServers:tree.title', 'Resource Hierarchy')}
          </Typography>
          <IconButton
            size="small"
            onClick={(e) => setAddMenuAnchor(e.currentTarget)}
            aria-label={t('resourceServers:tree.add', 'Add')}
          >
            <Plus size={16} />
          </IconButton>
          <Menu anchorEl={addMenuAnchor} open={Boolean(addMenuAnchor)} onClose={() => setAddMenuAnchor(null)}>
            <MenuItem
              onClick={() => {
                openAdd('resource');
                setAddMenuAnchor(null);
              }}
            >
              <ListItemIcon>
                <Layers size={16} />
              </ListItemIcon>
              <ListItemText>{t('resourceServers:tree.addResource', 'Add resource')}</ListItemText>
            </MenuItem>
            <MenuItem
              onClick={() => {
                openAdd('server-action');
                setAddMenuAnchor(null);
              }}
            >
              <ListItemIcon>
                <Zap size={16} />
              </ListItemIcon>
              <ListItemText>{t('resourceServers:tree.addAction', 'Add action')}</ListItemText>
            </MenuItem>
          </Menu>
        </Box>

        <Box sx={{flex: 1, overflowY: 'auto', p: 0.5, height: '100%'}}>
          {isLoading ? (
            <Box sx={{display: 'flex', justifyContent: 'center', py: 4}}>
              <CircularProgress size={24} />
            </Box>
          ) : (
            <>
              {/* Server-level actions */}
              {serverActions.map((action) => (
                <ActionNode
                  key={action.id}
                  resourceServerId={resourceServer.id}
                  action={action}
                  depth={0}
                  selectedNodeId={effectiveSelectedNode?.id ?? null}
                  onSelect={setSelectedNode}
                />
              ))}

              {/* Top-level resources */}
              {resources.map((resource) => (
                <ResourceNode
                  key={resource.id}
                  resourceServerId={resourceServer.id}
                  delimiter={resourceServer.delimiter}
                  node={resource}
                  depth={0}
                  selectedNodeId={effectiveSelectedNode?.id ?? null}
                  onSelect={setSelectedNode}
                  onAddChild={(mode, parentResourceId, parentPermission) =>
                    openAdd(mode, parentResourceId, parentPermission)
                  }
                />
              ))}

              {isEmpty && (
                <Box
                  sx={{
                    height: '100%',
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    px: 2,
                    textAlign: 'center',
                  }}
                >
                  <Typography variant="body2" color="text.disabled">
                    {t('resourceServers:tree.empty', 'No resources yet — add a resource or action to get started.')}
                  </Typography>
                </Box>
              )}
            </>
          )}
        </Box>
      </Paper>

      {/* Right: Detail Panel */}
      <Paper variant="outlined" sx={{flex: 1, minWidth: 280, overflow: 'hidden'}}>
        <ResourceDetailPanel
          selectedNode={effectiveSelectedNode}
          resourceServer={resourceServer}
          onRefresh={onRefresh}
        />
      </Paper>

      {addDialog && (
        <AddNodeDialog
          open={true}
          mode={addDialog.mode}
          resourceServerId={resourceServer.id}
          parentResourceId={addDialog.parentResourceId}
          parentPermission={addDialog.parentPermission}
          delimiter={resourceServer.delimiter}
          onClose={() => setAddDialog(null)}
          onSuccess={() => setAddDialog(null)}
        />
      )}
    </Box>
  );
}

/* -------------------------------------------------------------------------- */
/*  Root component — branches on type                                          */
/* -------------------------------------------------------------------------- */

export default function ResourceTree({resourceServer, onRefresh}: ResourceTreeProps): JSX.Element {
  if (resourceServer.type === 'MCP') {
    return <McpCapabilitiesPanel resourceServer={resourceServer} onRefresh={onRefresh} />;
  }
  return <GenericResourceTree resourceServer={resourceServer} onRefresh={onRefresh} />;
}
