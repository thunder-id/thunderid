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

import {useToast} from '@thunderid/contexts';
import {useLogger} from '@thunderid/logger/react';
import {
  Box,
  CircularProgress,
  Collapse,
  IconButton,
  ListItemIcon,
  ListItemText,
  Menu,
  MenuItem,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import {
  Check,
  ChevronDown,
  ChevronRight,
  Copy,
  Database,
  Layers,
  Plus,
  Trash2,
  Wrench,
  Zap,
} from '@wso2/oxygen-ui-icons-react';
import {useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import type {AddNodeMode} from './AddNodeDialog';
import type {SelectedNode} from './ResourceTree';
import useDeleteAction from '../../api/useDeleteAction';
import useDeleteResource from '../../api/useDeleteResource';
import useGetResourceActions from '../../api/useGetResourceActions';
import useGetResources from '../../api/useGetResources';
import {getActionKindLabel} from '../../config/get-action-kind-label';
import type {Action, Resource} from '../../models/resource-server';

interface ResourceTreeNodeProps {
  resourceServerId: string;
  delimiter: string;
  node: Resource;
  depth: number;
  selectedNodeId: string | null;
  onSelect: (node: SelectedNode) => void;
  onAddChild: (mode: AddNodeMode, parentResourceId: string, parentPermission: string) => void;
}

export function ResourceNode({
  resourceServerId,
  delimiter,
  node,
  depth,
  selectedNodeId,
  onSelect,
  onAddChild,
}: ResourceTreeNodeProps): JSX.Element {
  const {t} = useTranslation();
  const {showToast} = useToast();
  const logger = useLogger('ResourceNode');
  const [expanded, setExpanded] = useState(false);
  const [hovered, setHovered] = useState(false);
  const [addMenuAnchor, setAddMenuAnchor] = useState<HTMLElement | null>(null);

  const deleteResource = useDeleteResource(resourceServerId);

  const {data: childResources} = useGetResources(resourceServerId, node.id);
  const {data: resourceActions} = useGetResourceActions(resourceServerId, node.id, expanded);

  const isSelected = selectedNodeId === node.id;
  const children = childResources?.resources ?? [];
  const actions = resourceActions?.actions ?? [];
  const [copiedPermission, setCopiedPermission] = useState(false);

  const handleCopyPermission = (e: React.MouseEvent): void => {
    e.stopPropagation();
    navigator.clipboard
      .writeText(node.permission)
      .then(() => {
        setCopiedPermission(true);
        setTimeout(() => setCopiedPermission(false), 1500);
      })
      .catch((err: unknown) => logger.error('Failed to copy permission', {error: err}));
  };

  const handleDelete = (e: React.MouseEvent): void => {
    e.stopPropagation();
    deleteResource.mutate(node.id, {
      onSuccess: () => {
        showToast(t('resourceServers:tree.deleteResource.success', 'Resource deleted.'), 'success');
      },
      onError: (err: Error) => {
        logger.error('Failed to delete resource', {error: err});
        showToast(
          t('resourceServers:tree.deleteResource.error', 'Cannot delete — remove child resources and actions first.'),
          'error',
        );
      },
    });
  };

  const nodeIcon = <Layers size={16} style={{flexShrink: 0, opacity: 0.7}} />;

  return (
    <Box>
      <Box
        onMouseEnter={() => setHovered(true)}
        onMouseLeave={() => setHovered(false)}
        onClick={() =>
          onSelect({
            type: 'resource',
            id: node.id,
            data: node,
          })
        }
        role="treeitem"
        aria-level={depth + 1}
        aria-label={node.name}
        sx={{
          display: 'flex',
          alignItems: 'center',
          gap: 0.75,
          pl: depth * 2 + 0.5,
          pr: 0.5,
          py: 0.5,
          borderRadius: 1,
          cursor: 'pointer',
          bgcolor: isSelected ? 'action.selected' : hovered ? 'action.hover' : 'transparent',
          '&:hover': {bgcolor: isSelected ? 'action.selected' : 'action.hover'},
        }}
      >
        <IconButton
          size="small"
          onClick={(e) => {
            e.stopPropagation();
            setExpanded((v) => !v);
          }}
          sx={{p: 0.25, flexShrink: 0}}
        >
          {expanded ? <ChevronDown size={16} /> : <ChevronRight size={16} />}
        </IconButton>

        {nodeIcon}

        <Box sx={{flex: 1, minWidth: 0, display: 'flex', alignItems: 'center', gap: 0.75}}>
          <Typography
            variant="body2"
            sx={{flexShrink: 1, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis', minWidth: 40}}
          >
            {node.name}
          </Typography>
          {(isSelected || hovered) && (
            <Tooltip
              title={
                copiedPermission
                  ? t('common:copied', 'Copied!')
                  : t('resourceServers:tree.copyPermission', 'Copy permission string')
              }
            >
              <IconButton size="small" sx={{p: 0.15, flexShrink: 0}} onClick={handleCopyPermission}>
                {copiedPermission ? <Check size={14} /> : <Copy size={14} />}
              </IconButton>
            </Tooltip>
          )}
        </Box>

        {(isSelected || hovered || Boolean(addMenuAnchor)) && (
          <Box sx={{display: 'flex', gap: 0.25, flexShrink: 0}} onClick={(e) => e.stopPropagation()}>
            <Tooltip title={t('resourceServers:tree.add', 'Add')}>
              <IconButton
                size="small"
                sx={{p: 0.25}}
                aria-label={t('resourceServers:tree.add', 'Add')}
                onClick={(e) => {
                  e.stopPropagation();
                  setAddMenuAnchor(e.currentTarget);
                }}
              >
                <Plus size={14} />
              </IconButton>
            </Tooltip>
            <Tooltip title={t('common:delete', 'Delete')}>
              <IconButton
                size="small"
                sx={{p: 0.25, color: 'error.main'}}
                onClick={handleDelete}
                disabled={deleteResource.isPending}
              >
                {deleteResource.isPending ? <CircularProgress size={12} /> : <Trash2 size={14} />}
              </IconButton>
            </Tooltip>
          </Box>
        )}
      </Box>

      <Menu
        anchorEl={addMenuAnchor}
        open={Boolean(addMenuAnchor)}
        onClose={() => setAddMenuAnchor(null)}
        slotProps={{paper: {sx: {minWidth: 160}}}}
      >
        <MenuItem
          key="sub-resource"
          onClick={() => {
            onAddChild('sub-resource', node.id, node.permission);
            setAddMenuAnchor(null);
          }}
        >
          <ListItemIcon>
            <Layers size={16} />
          </ListItemIcon>
          <ListItemText>{t('resourceServers:tree.addSubResource', 'Add sub-resource')}</ListItemText>
        </MenuItem>
        <MenuItem
          key="resource-action"
          onClick={() => {
            onAddChild('resource-action', node.id, node.permission);
            setAddMenuAnchor(null);
          }}
        >
          <ListItemIcon>
            <Zap size={16} />
          </ListItemIcon>
          <ListItemText>{t('resourceServers:tree.addAction', 'Add action')}</ListItemText>
        </MenuItem>
      </Menu>

      <Collapse in={expanded}>
        {actions.map((action) => (
          <ActionNode
            key={action.id}
            resourceServerId={resourceServerId}
            action={action}
            depth={depth + 1}
            parentResourceId={node.id}
            selectedNodeId={selectedNodeId}
            onSelect={onSelect}
          />
        ))}
        {children.map((child) => (
          <ResourceNode
            key={child.id}
            resourceServerId={resourceServerId}
            delimiter={delimiter}
            node={child}
            depth={depth + 1}
            selectedNodeId={selectedNodeId}
            onSelect={onSelect}
            onAddChild={onAddChild}
          />
        ))}
      </Collapse>
    </Box>
  );
}

interface ActionNodeProps {
  resourceServerId: string;
  action: Action;
  depth: number;
  parentResourceId?: string;
  selectedNodeId: string | null;
  onSelect: (node: SelectedNode) => void;
  breadcrumb?: string[];
}

export function ActionNode({
  resourceServerId,
  action,
  depth,
  parentResourceId = undefined,
  selectedNodeId,
  onSelect,
  breadcrumb = [],
}: ActionNodeProps): JSX.Element {
  const {t} = useTranslation();
  const {showToast} = useToast();
  const logger = useLogger('ActionNode');
  const [hovered, setHovered] = useState(false);

  const deleteAction = useDeleteAction(resourceServerId, parentResourceId);
  const isSelected = selectedNodeId === action.id;
  const nodeType: SelectedNode['type'] = parentResourceId ? 'resource-action' : 'server-action';
  const [copiedPermission, setCopiedPermission] = useState(false);

  const handleCopyPermission = (e: React.MouseEvent): void => {
    e.stopPropagation();
    navigator.clipboard
      .writeText(action.permission)
      .then(() => {
        setCopiedPermission(true);
        setTimeout(() => setCopiedPermission(false), 1500);
      })
      .catch((err: unknown) => logger.error('Failed to copy permission', {error: err}));
  };

  const resolveDeleteSuccessToast = (): string => {
    if (action.kind === 'tool') return t('resourceServers:mcp.deleteTool.success', 'Tool deleted.');
    if (action.kind === 'resource') return t('resourceServers:mcp.deleteResource.success', 'Resource deleted.');
    return t('resourceServers:tree.deleteAction.success', 'Action deleted.');
  };

  const handleDelete = (e: React.MouseEvent): void => {
    e.stopPropagation();
    deleteAction.mutate(action.id, {
      onSuccess: () => showToast(resolveDeleteSuccessToast(), 'success'),
      onError: (err: Error) => {
        logger.error('Failed to delete action', {error: err});
        showToast(t('resourceServers:tree.deleteAction.error', 'Failed to delete action.'), 'error');
      },
    });
  };

  const kindAriaLabel = getActionKindLabel(action.kind, t);

  const kindIcon =
    action.kind === 'tool' ? (
      <Wrench size={16} />
    ) : action.kind === 'resource' ? (
      <Database size={16} />
    ) : (
      <Zap size={16} />
    );

  return (
    <Box
      onMouseEnter={() => setHovered(true)}
      onMouseLeave={() => setHovered(false)}
      onClick={() => onSelect({type: nodeType, id: action.id, data: action, parentResourceId, breadcrumb})}
      role="treeitem"
      aria-level={depth + 1}
      aria-label={`${kindAriaLabel}: ${action.name}`}
      sx={{
        display: 'flex',
        alignItems: 'center',
        gap: 0.75,
        pl: depth * 2 + 2,
        pr: 0.5,
        py: 0.5,
        borderRadius: 1,
        cursor: 'pointer',
        bgcolor: isSelected ? 'action.selected' : hovered ? 'action.hover' : 'transparent',
      }}
    >
      <Box sx={{flexShrink: 0, opacity: 0.7, display: 'flex', alignItems: 'center'}}>{kindIcon}</Box>

      <Box sx={{flex: 1, minWidth: 0, display: 'flex', alignItems: 'center', gap: 0.75}}>
        <Typography
          variant="body2"
          sx={{flexShrink: 1, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis', minWidth: 40}}
        >
          {action.name}
        </Typography>
        {(isSelected || hovered) && (
          <Tooltip
            title={
              copiedPermission
                ? t('common:copied', 'Copied!')
                : t('resourceServers:tree.copyPermission', 'Copy permission string')
            }
          >
            <IconButton size="small" sx={{p: 0.15, flexShrink: 0}} onClick={handleCopyPermission}>
              {copiedPermission ? <Check size={10} /> : <Copy size={10} />}
            </IconButton>
          </Tooltip>
        )}
      </Box>

      {(isSelected || hovered) && (
        <Box sx={{display: 'flex', flexShrink: 0}} onClick={(e) => e.stopPropagation()}>
          <Tooltip title={t('common:delete', 'Delete')}>
            <IconButton
              size="small"
              sx={{p: 0.25, color: 'error.main'}}
              onClick={handleDelete}
              disabled={deleteAction.isPending}
            >
              {deleteAction.isPending ? <CircularProgress size={12} /> : <Trash2 size={14} />}
            </IconButton>
          </Tooltip>
        </Box>
      )}
    </Box>
  );
}
