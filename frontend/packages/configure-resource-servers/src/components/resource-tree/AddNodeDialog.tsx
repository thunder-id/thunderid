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
  Button,
  Chip,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControl,
  FormLabel,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import {type JSX, useState} from 'react';
import {useTranslation} from 'react-i18next';
import useCreateAction from '../../api/useCreateAction';
import useCreateResource from '../../api/useCreateResource';
import type {ActionKind} from '../../models/resource-server';
import {deriveHandle} from '../../utils/deriveHandle';

export type AddNodeMode =
  | 'resource'
  | 'sub-resource'
  | 'server-action'
  | 'resource-action'
  | 'mcp-server-tool'
  | 'mcp-server-resource';

export interface AddNodeDialogProps {
  /** Whether the dialog is open. */
  open: boolean;
  /** The mode that determines what kind of node to create. */
  mode: AddNodeMode;
  /** The ID of the resource server. */
  resourceServerId: string;
  /** The ID of the parent resource (for in-namespace creates). */
  parentResourceId?: string;
  /** The permission prefix used to derive the full permission string preview. */
  parentPermission: string;
  /** The permission delimiter character. */
  delimiter: string;
  /** Called when the dialog is closed without submitting. */
  onClose: () => void;
  /** Called after a successful create. */
  onSuccess: () => void;
}

interface ModeConfig {
  /** The action kind to send when creating an action, or undefined for resource modes. */
  kind: ActionKind | undefined;
  /** Whether this mode creates an action (vs. a resource). */
  isAction: boolean;
  /** Whether this mode nests the new node under `parentResourceId`. */
  usesParentResourceId: boolean;
}

const MODE_CONFIG: Record<AddNodeMode, ModeConfig> = {
  resource: {kind: undefined, isAction: false, usesParentResourceId: false},
  'sub-resource': {kind: undefined, isAction: false, usesParentResourceId: false},
  'server-action': {kind: undefined, isAction: true, usesParentResourceId: false},
  'resource-action': {kind: undefined, isAction: true, usesParentResourceId: true},
  'mcp-server-tool': {kind: 'tool', isAction: true, usesParentResourceId: false},
  'mcp-server-resource': {kind: 'resource', isAction: true, usesParentResourceId: false},
};

export default function AddNodeDialog({
  open,
  mode,
  resourceServerId,
  parentResourceId = undefined,
  parentPermission,
  delimiter,
  onClose,
  onSuccess,
}: AddNodeDialogProps): JSX.Element {
  const {t} = useTranslation();
  const {showToast} = useToast();
  const logger = useLogger('AddNodeDialog');

  const [name, setName] = useState('');
  const [handle, setHandle] = useState('');
  const [description, setDescription] = useState('');
  const [handleEdited, setHandleEdited] = useState(false);

  const isAction = MODE_CONFIG[mode].isAction;
  const resourceId = MODE_CONFIG[mode].usesParentResourceId ? parentResourceId : undefined;
  const kind = MODE_CONFIG[mode].kind;

  const createResource = useCreateResource(resourceServerId);
  const createAction = useCreateAction(resourceServerId, resourceId);

  const permissionPrefix = parentPermission ? `${parentPermission}${delimiter}` : '';
  const derivedPermission = handle.trim() ? `${permissionPrefix}${handle.trim()}` : `${permissionPrefix}…`;

  const handleClose = (): void => {
    setName('');
    setHandle('');
    setDescription('');
    setHandleEdited(false);
    onClose();
  };

  const resolveSuccessToast = (): string => {
    if (mode === 'mcp-server-tool') {
      return t('resourceServers:mcp.addTool.success', 'Tool added.');
    }
    if (mode === 'mcp-server-resource') {
      return t('resourceServers:mcp.addResource.success', 'Resource added.');
    }
    if (isAction) {
      return t('resourceServers:tree.addAction.success', 'Action added.');
    }
    return t('resourceServers:tree.addResource.success', 'Resource added.');
  };

  const resolveErrorToast = (): string => {
    if (mode === 'mcp-server-tool') {
      return t('resourceServers:mcp.addTool.error', 'Failed to add tool.');
    }
    if (mode === 'mcp-server-resource') {
      return t('resourceServers:mcp.addResource.error', 'Failed to add resource.');
    }
    if (isAction) {
      return t('resourceServers:tree.addAction.error', 'Failed to add action.');
    }
    return t('resourceServers:tree.addResource.error', 'Failed to add resource.');
  };

  const resolveNamePlaceholder = (): string => {
    if (kind === 'tool') return t('resourceServers:tree.fields.namePlaceholder.tool', 'e.g. Send message');
    if (kind === 'resource') return t('resourceServers:tree.fields.namePlaceholder.resource', 'e.g. User profile');
    if (isAction) return t('resourceServers:tree.fields.namePlaceholder.action', 'e.g. Read');
    return t('resourceServers:tree.fields.namePlaceholder.resourceGeneric', 'e.g. Orders');
  };

  const resolveHandlePlaceholder = (): string => {
    if (kind === 'tool') return t('resourceServers:tree.fields.handlePlaceholder.tool', 'e.g. send-message');
    if (kind === 'resource') return t('resourceServers:tree.fields.handlePlaceholder.resource', 'e.g. user-profile');
    if (isAction) return t('resourceServers:tree.fields.handlePlaceholder.action', 'e.g. read');
    return t('resourceServers:tree.fields.handlePlaceholder.resourceGeneric', 'e.g. orders');
  };

  const resolveDescriptionPlaceholder = (): string => {
    if (kind === 'tool') {
      return t(
        'resourceServers:tree.fields.descriptionPlaceholder.tool',
        'e.g. Sends a message to the specified channel',
      );
    }
    if (kind === 'resource') {
      return t('resourceServers:tree.fields.descriptionPlaceholder.resource', 'e.g. User profile data and preferences');
    }
    if (isAction) {
      return t('resourceServers:tree.fields.descriptionPlaceholder.action', 'e.g. Grants read access to the resource');
    }
    return t(
      'resourceServers:tree.fields.descriptionPlaceholder.resourceGeneric',
      'e.g. Manages order data and lifecycle',
    );
  };

  const handleSubmit = (): void => {
    const trimmedName = name.trim();
    const trimmedHandle = handle.trim();
    if (!trimmedName || !trimmedHandle) return;

    const baseData = {name: trimmedName, handle: trimmedHandle, description: description.trim() || undefined};

    if (isAction) {
      createAction.mutate(
        {...baseData, kind},
        {
          onSuccess: () => {
            showToast(resolveSuccessToast(), 'success');
            handleClose();
            onSuccess();
          },
          onError: (err: Error) => {
            logger.error('Failed to create action', {error: err});
            showToast(resolveErrorToast(), 'error');
          },
        },
      );
    } else {
      const parent = mode === 'sub-resource' ? parentResourceId : undefined;
      createResource.mutate(
        {...baseData, parent},
        {
          onSuccess: () => {
            showToast(resolveSuccessToast(), 'success');
            handleClose();
            onSuccess();
          },
          onError: (err: Error) => {
            logger.error('Failed to create resource', {error: err});
            showToast(resolveErrorToast(), 'error');
          },
        },
      );
    }
  };

  const titleMap: Record<AddNodeMode, string> = {
    resource: t('resourceServers:tree.addResource.title', 'Add Resource'),
    'sub-resource': t('resourceServers:tree.addSubResource.title', 'Add Sub-resource'),
    'server-action': t('resourceServers:tree.addAction.title', 'Add Action'),
    'resource-action': t('resourceServers:tree.addAction.title', 'Add Action'),
    'mcp-server-tool': t('resourceServers:mcp.addTool.title', 'Add tool permission'),
    'mcp-server-resource': t('resourceServers:mcp.addResource.title', 'Add resource permission'),
  };

  const isPending = createResource.isPending || createAction.isPending;
  const handleContainsDelimiter = handle.includes(delimiter);

  return (
    <Dialog open={open} onClose={handleClose} maxWidth="sm" fullWidth>
      <DialogTitle>{titleMap[mode]}</DialogTitle>
      <DialogContent>
        <Box sx={{display: 'flex', flexDirection: 'column', gap: 2, pt: 1}}>
          <FormControl fullWidth required>
            <FormLabel>{t('resourceServers:tree.fields.name', 'Name')}</FormLabel>
            <TextField
              value={name}
              onChange={(e) => {
                const newName = e.target.value;
                setName(newName);
                if (!handleEdited) {
                  setHandle(deriveHandle(newName, delimiter));
                }
              }}
              fullWidth
              size="small"
              placeholder={resolveNamePlaceholder()}
              // eslint-disable-next-line jsx-a11y/no-autofocus
              autoFocus
            />
          </FormControl>
          <FormControl fullWidth required>
            <FormLabel>{t('resourceServers:tree.fields.handle', 'Handle')}</FormLabel>
            <TextField
              value={handle}
              onChange={(e) => {
                setHandleEdited(true);
                const sanitized = e.target.value.toLowerCase().replace(/[^a-z0-9._\-:/]/g, '');
                setHandle(sanitized);
              }}
              fullWidth
              size="small"
              placeholder={resolveHandlePlaceholder()}
              error={handleContainsDelimiter}
              helperText={
                handleContainsDelimiter
                  ? t(
                      'resourceServers:tree.fields.handleDelimiterError',
                      `Handle cannot contain the delimiter character "${delimiter}".`,
                    )
                  : t(
                      'resourceServers:tree.fields.handleHint',
                      'Lowercase, alphanumeric, and . _ - : / characters. Cannot be changed after creation.',
                    )
              }
            />
          </FormControl>
          <FormControl fullWidth>
            <FormLabel>{t('resourceServers:tree.fields.description', 'Description')}</FormLabel>
            <TextField
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              fullWidth
              size="small"
              multiline
              rows={2}
              placeholder={resolveDescriptionPlaceholder()}
            />
          </FormControl>

          {handle.trim() && (
            <Box sx={{bgcolor: 'action.hover', borderRadius: 1, px: 1.5, py: 1}}>
              <Typography variant="caption" color="text.secondary">
                {t('resourceServers:tree.fields.permissionPreview', 'Permission string')}
              </Typography>
              <Box sx={{mt: 0.5}}>
                <Chip
                  label={derivedPermission}
                  size="small"
                  variant="outlined"
                  sx={{fontFamily: 'monospace', fontSize: '0.8rem'}}
                />
              </Box>
            </Box>
          )}
        </Box>
      </DialogContent>
      <DialogActions>
        <Button variant="outlined" onClick={handleClose} disabled={isPending}>
          {t('common:cancel', 'Cancel')}
        </Button>
        <Button
          variant="contained"
          onClick={handleSubmit}
          disabled={isPending || !name.trim() || !handle.trim() || handleContainsDelimiter}
        >
          {isPending ? t('common:adding', 'Adding…') : t('common:add', 'Add')}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
