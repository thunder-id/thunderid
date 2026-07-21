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
  Button,
  Chip,
  CircularProgress,
  Divider,
  IconButton,
  ListItemIcon,
  ListItemText,
  Menu,
  MenuItem,
  Paper,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import {Database, Plus, Wrench} from '@wso2/oxygen-ui-icons-react';
import {useMemo, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import AddNodeDialog, {type AddNodeMode} from './AddNodeDialog';
import {PANEL_HEADER_ROW_HEIGHT} from './constants';
import ResourceDetailPanel from './ResourceDetailPanel';
import type {KindFilter, SelectedNode} from './ResourceTree';
import {ActionNode} from './ResourceTreeNode';
import useGetServerActions from '../../api/useGetServerActions';
import type {ResourceServer} from '../../models/resource-server';

interface McpCapabilitiesPanelProps {
  resourceServer: ResourceServer;
  onRefresh: () => void;
}

export default function McpCapabilitiesPanel({resourceServer, onRefresh}: McpCapabilitiesPanelProps): JSX.Element {
  const {t} = useTranslation();
  const [selectedNode, setSelectedNode] = useState<SelectedNode | null>(null);
  const [addDialog, setAddDialog] = useState<{
    mode: AddNodeMode;
    parentResourceId?: string;
    parentPermission: string;
  } | null>(null);
  const [addMenuAnchor, setAddMenuAnchor] = useState<HTMLElement | null>(null);
  const [kindFilter, setKindFilter] = useState<KindFilter>('all');

  const {data: serverActionsData, isLoading: loadingActions} = useGetServerActions(resourceServer.id);

  const serverActions = useMemo(() => serverActionsData?.actions ?? [], [serverActionsData]);

  const serverTools = useMemo(() => serverActions.filter((a) => a.kind === 'tool'), [serverActions]);
  const serverResources = useMemo(() => serverActions.filter((a) => a.kind === 'resource'), [serverActions]);

  const openAdd = (mode: AddNodeMode, parentResourceId?: string, parentPermission?: string): void => {
    setAddDialog({
      mode,
      parentResourceId,
      parentPermission: parentPermission ?? '',
    });
  };

  const isLoading = loadingActions;
  const isEmpty = serverActions.length === 0;

  const effectiveSelectedNode = useMemo<SelectedNode | null>(() => {
    if (selectedNode) return selectedNode;
    if (serverTools.length > 0) return {type: 'server-action', id: serverTools[0].id, data: serverTools[0]};
    if (serverResources.length > 0) return {type: 'server-action', id: serverResources[0].id, data: serverResources[0]};
    return null;
  }, [selectedNode, serverTools, serverResources]);

  const filteredServerActions = useMemo(
    () =>
      serverActions.filter((a) => {
        if (kindFilter === 'tool') return a.kind === 'tool';
        if (kindFilter === 'resource') return a.kind === 'resource';
        return true;
      }),
    [serverActions, kindFilter],
  );

  return (
    <Box sx={{display: 'flex', flexDirection: {xs: 'column', md: 'row'}, gap: 2, height: {md: '100%'}}}>
      {/* Left: Capabilities panel */}
      <Paper
        variant="outlined"
        sx={{
          flex: 1,
          minWidth: {md: 300},
          minHeight: {xs: 320, md: 0},
          display: 'flex',
          flexDirection: 'column',
          overflow: 'hidden',
        }}
      >
        {/* Panel header */}
        <Box
          sx={{
            px: 1.5,
            height: PANEL_HEADER_ROW_HEIGHT,
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
          {(!isEmpty || isLoading) && (
            <>
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
                  <ListItemText>{t('resourceServers:mcp.addTool', 'Add tool permission')}</ListItemText>
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
                  <ListItemText>{t('resourceServers:mcp.addResource', 'Add resource permission')}</ListItemText>
                </MenuItem>
              </Menu>
            </>
          )}
        </Box>

        {/* Toolbar: filter chips */}
        {!isEmpty && !isLoading && (
          <Box
            sx={{
              px: 1.5,
              height: PANEL_HEADER_ROW_HEIGHT,
              display: 'flex',
              alignItems: 'center',
              borderBottom: '1px solid',
              borderColor: 'divider',
            }}
          >
            <Stack
              direction="row"
              flexWrap="wrap"
              useFlexGap
              gap={0.75}
              role="radiogroup"
              aria-label={t('resourceServers:mcp.filter.label', 'Filter capabilities')}
            >
              <Chip
                label={t('resourceServers:mcp.filter.all', 'All')}
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
                label={t('resourceServers:mcp.filter.tools', 'Tools')}
                icon={<Wrench size={14} />}
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
                label={t('resourceServers:mcp.filter.resources', 'Resources')}
                icon={<Database size={14} />}
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
          </Box>
        )}

        {/* Scroll body */}
        <Box role="tree" sx={{flex: 1, overflowY: 'auto', height: '100%', pt: 1}}>
          {isLoading ? (
            <Box sx={{display: 'flex', justifyContent: 'center', py: 4}}>
              <CircularProgress size={24} />
            </Box>
          ) : isEmpty ? (
            <Box
              sx={{
                flex: 1,
                display: 'flex',
                flexDirection: 'column',
                alignItems: 'center',
                justifyContent: 'center',
                height: '100%',
              }}
            >
              <Typography variant="body2" color="text.disabled" sx={{mb: 3, textAlign: 'center', maxWidth: 360}}>
                {t(
                  'resourceServers:mcp.empty',
                  'No capabilities have been added to this MCP server yet. Add tool permissions to control which tools can be invoked, or resource permissions to control access to data sources.',
                )}
              </Typography>
              <Stack spacing={1.5} alignItems="center" sx={{width: '100%', maxWidth: 280}}>
                <Button
                  variant="outlined"
                  fullWidth
                  startIcon={<Wrench size={16} />}
                  onClick={() => openAdd('mcp-server-tool')}
                >
                  {t('resourceServers:mcp.addTool', 'Add tool permission')}
                </Button>
                <Divider sx={{width: '100%'}}>
                  <Typography variant="caption" color="text.disabled">
                    {t('common:or', 'or')}
                  </Typography>
                </Divider>
                <Button
                  variant="outlined"
                  fullWidth
                  startIcon={<Database size={16} />}
                  onClick={() => openAdd('mcp-server-resource')}
                >
                  {t('resourceServers:mcp.addResource', 'Add resource permission')}
                </Button>
              </Stack>
            </Box>
          ) : (
            filteredServerActions.map((action) => (
              <ActionNode
                key={action.id}
                resourceServerId={resourceServer.id}
                action={action}
                depth={0}
                selectedNodeId={effectiveSelectedNode?.id ?? null}
                onSelect={setSelectedNode}
              />
            ))
          )}
        </Box>
      </Paper>

      {/* Right: Detail Panel */}
      {(!isEmpty || isLoading) && (
        <Paper variant="outlined" sx={{flex: 1, minWidth: {md: 280}, overflow: 'hidden'}}>
          <ResourceDetailPanel
            selectedNode={effectiveSelectedNode}
            resourceServer={resourceServer}
            onRefresh={onRefresh}
          />
        </Paper>
      )}

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
