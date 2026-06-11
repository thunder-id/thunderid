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
  CircularProgress,
  IconButton,
  ListItemIcon,
  ListItemText,
  Menu,
  MenuItem,
  Paper,
  Typography,
} from '@wso2/oxygen-ui';
import {Layers, Plus, Zap} from '@wso2/oxygen-ui-icons-react';
import {useMemo, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import AddNodeDialog, {type AddNodeMode} from './AddNodeDialog';
import ResourceDetailPanel from './ResourceDetailPanel';
import {ActionNode, ResourceNode} from './ResourceTreeNode';
import useGetResources from '../../api/useGetResources';
import useGetServerActions from '../../api/useGetServerActions';
import type {Action, Resource, ResourceServer} from '../../models/resource-server';

export type SelectedNode =
  | {type: 'server'; id: string; data: ResourceServer}
  | {type: 'resource'; id: string; data: Resource}
  | {type: 'server-action'; id: string; data: Action; parentResourceId?: string}
  | {type: 'resource-action'; id: string; data: Action; parentResourceId?: string};

interface ResourceTreeProps {
  resourceServer: ResourceServer;
  onRefresh: () => void;
}

export default function ResourceTree({resourceServer, onRefresh}: ResourceTreeProps): JSX.Element {
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
