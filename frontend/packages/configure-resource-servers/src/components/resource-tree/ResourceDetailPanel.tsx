// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useToast} from '@thunderid/contexts';
import {useLogger} from '@thunderid/logger/react';
import {getErrorMessage} from '@thunderid/utils';
import {
  Alert,
  Box,
  Button,
  Chip,
  Divider,
  FormControl,
  FormLabel,
  IconButton,
  InputAdornment,
  Stack,
  TextField,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import {Check, Copy} from '@wso2/oxygen-ui-icons-react';
import {useCallback, useMemo, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {PANEL_HEADER_ROW_HEIGHT} from './constants';
import type {SelectedNode} from './ResourceTree';
import useUpdateAction from '../../api/useUpdateAction';
import useUpdateResource from '../../api/useUpdateResource';
import useUpdateResourceServer from '../../api/useUpdateResourceServer';
import {getActionKindIcon} from '../../config/get-action-kind-icon';
import {getActionKindLabel} from '../../config/get-action-kind-label';
import type {ResourceServer} from '../../models/resource-server';

interface ResourceDetailPanelProps {
  selectedNode: SelectedNode | null;
  resourceServer: ResourceServer;
  onRefresh: () => void;
}

function deriveInitialValues(node: SelectedNode | null): {name: string; description: string; identifier: string} {
  if (!node) return {name: '', description: '', identifier: ''};
  if (node.type === 'server') {
    const rs = node.data;
    return {name: rs.name, description: rs.description ?? '', identifier: rs.identifier ?? ''};
  }
  const item = node.data;
  return {name: item.name, description: item.description ?? '', identifier: ''};
}

interface DetailFormProps {
  selectedNode: SelectedNode;
  resourceServer: ResourceServer;
  onRefresh: () => void;
}

function DetailForm({selectedNode, resourceServer, onRefresh}: DetailFormProps): JSX.Element {
  const {t} = useTranslation();
  const {showToast} = useToast();
  const logger = useLogger('ResourceDetailPanel');

  const initial = deriveInitialValues(selectedNode);
  const [name, setName] = useState(initial.name);
  const [description, setDescription] = useState(initial.description);
  const [identifier, setIdentifier] = useState(initial.identifier);
  // Snapshot to compare current field values against — advanced on mount, Save, and Discard —
  // so typing a value back to its original clears the bar instead of a one-way "touched" flag.
  const [baseline, setBaseline] = useState(initial);
  const [copiedPermission, setCopiedPermission] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);

  // Resolves an error through the `resourceServers` catalog. `t` defaults to the `common`
  // namespace, so this forwards explicit `ns:` prefixes unchanged and prefixes bare keys with
  // `resourceServers:`, per getErrorMessage's namespace-resolution contract.
  const tForErrors = useCallback(
    (key: string, options?: Record<string, unknown>): string =>
      t(key.includes(':') ? key : `resourceServers:${key}`, options),
    [t],
  );

  const dirty = useMemo(() => {
    const norm = (v: string): string => v.trim();
    return (
      norm(name) !== norm(baseline.name) ||
      norm(description) !== norm(baseline.description) ||
      norm(identifier) !== norm(baseline.identifier)
    );
  }, [name, description, identifier, baseline]);

  const updateRs = useUpdateResourceServer();
  const updateResource = useUpdateResource(resourceServer.id);
  const updateServerAction = useUpdateAction(resourceServer.id);
  const updateResourceAction = useUpdateAction(
    resourceServer.id,
    selectedNode.type === 'resource-action' ? (selectedNode.parentResourceId ?? undefined) : undefined,
  );

  const resetForm = useCallback(() => {
    setSaveError(null);
    setName(baseline.name);
    setDescription(baseline.description);
    setIdentifier(baseline.identifier);
  }, [baseline]);

  const handleSave = (): void => {
    if (selectedNode.type === 'server') {
      const nextIdentifier = identifier.trim();
      if (!nextIdentifier) {
        showToast(t('resourceServers:detail.identifierRequired', 'Identifier is required.'), 'error');
        return;
      }

      updateRs.mutate(
        {
          id: resourceServer.id,
          data: {name, description: description || null, identifier: nextIdentifier, ouId: resourceServer.ouId},
        },
        {
          onSuccess: () => {
            setSaveError(null);
            showToast(t('resourceServers:detail.saved', 'Changes saved.'), 'success');
            setBaseline({name, description, identifier: nextIdentifier});
            onRefresh();
          },
          onError: (err: Error) => {
            logger.error('Failed to update resource server', {error: err});
            setSaveError(getErrorMessage(err, tForErrors, 'detail.saveError', 'Failed to save.'));
          },
        },
      );
    } else if (selectedNode.type === 'resource') {
      updateResource.mutate(
        {resourceId: selectedNode.id, data: {name, description: description || null}},
        {
          onSuccess: () => {
            setSaveError(null);
            showToast(t('resourceServers:detail.saved', 'Changes saved.'), 'success');
            setBaseline((prev) => ({...prev, name, description}));
            onRefresh();
          },
          onError: (err: Error) => {
            logger.error('Failed to update resource', {error: err});
            setSaveError(getErrorMessage(err, tForErrors, 'detail.saveError', 'Failed to save.'));
          },
        },
      );
    } else {
      const updater = selectedNode.type === 'resource-action' ? updateResourceAction : updateServerAction;
      updater.mutate(
        {actionId: selectedNode.id, data: {name, description: description || null}},
        {
          onSuccess: () => {
            setSaveError(null);
            showToast(t('resourceServers:detail.saved', 'Changes saved.'), 'success');
            setBaseline((prev) => ({...prev, name, description}));
            onRefresh();
          },
          onError: (err: Error) => {
            logger.error('Failed to update action', {error: err});
            setSaveError(getErrorMessage(err, tForErrors, 'detail.saveError', 'Failed to save.'));
          },
        },
      );
    }
  };

  const isReadOnly = Boolean(resourceServer.isReadOnly);
  const isPending =
    updateRs.isPending || updateResource.isPending || updateServerAction.isPending || updateResourceAction.isPending;

  const permission = selectedNode.type === 'server' ? '' : selectedNode.data.permission;

  const handleCopyPermission = (): void => {
    navigator.clipboard
      .writeText(permission)
      .then(() => {
        setCopiedPermission(true);
        setTimeout(() => setCopiedPermission(false), 1500);
      })
      .catch((err: unknown) => logger.error('Failed to copy permission', {error: err}));
  };

  const resolveNodeTypeLabel = (): string => {
    if (selectedNode.type === 'server') return t('resourceServers:detail.types.resourceServer', 'Resource server');
    if (selectedNode.type === 'resource') {
      if (resourceServer.type === 'MCP') return getActionKindLabel(undefined, t);
      return t('resourceServers:detail.types.resource', 'Resource');
    }
    const action = selectedNode.data;
    if (action.kind) return getActionKindLabel(action.kind, t);
    return t('resourceServers:detail.types.action', 'Action');
  };

  const handleField = (setter: (v: string) => void) => (e: React.ChangeEvent<HTMLInputElement>) => {
    if (saveError) setSaveError(null);
    setter(e.target.value);
  };

  const isMcpNonServer = resourceServer.type === 'MCP' && selectedNode.type !== 'server';

  const resolveMcpKindLabel = (): string => {
    if (selectedNode.type === 'server') return '';

    if (selectedNode.type === 'resource') {
      return t('resourceServers:mcp.types.namespace', 'Namespace');
    }

    const action = selectedNode.data;
    if (action.kind === 'tool') {
      return t('resourceServers:mcp.types.tool', 'Tool');
    }
    if (action.kind === 'resource') {
      return t('resourceServers:mcp.types.resource', 'Resource');
    }
    return '';
  };

  const kindNoun = resolveMcpKindLabel().toLowerCase() || 'item';

  const formContent = (
    <>
      {!isMcpNonServer && (
        <Typography variant="caption" color="text.secondary" sx={{textTransform: 'uppercase', letterSpacing: 0.5}}>
          {resolveNodeTypeLabel()}
        </Typography>
      )}

      {isReadOnly && selectedNode.type === 'server' && (
        <Alert severity="info">
          {t('resourceServers:detail.readOnlyWarning', 'This is a system resource server and cannot be modified.')}
        </Alert>
      )}

      {saveError && <Alert severity="error">{saveError}</Alert>}

      <Stack spacing={2}>
        <FormControl fullWidth>
          <FormLabel>{t('resourceServers:detail.fields.name', 'Name')}</FormLabel>
          <TextField
            value={name}
            onChange={handleField(setName)}
            fullWidth
            size="small"
            disabled={isReadOnly}
            helperText={
              isMcpNonServer
                ? t('resourceServers:mcp.detail.nameHint', 'A human-readable name for this {{kind}}.', {
                    kind: kindNoun,
                  })
                : undefined
            }
          />
        </FormControl>

        {isMcpNonServer && (
          <>
            <FormControl fullWidth>
              <FormLabel htmlFor="mcp-detail-handle">
                {t('resourceServers:detail.fields.handle', 'Handle (immutable)')}
              </FormLabel>
              <TextField
                fullWidth
                id="mcp-detail-handle"
                value={selectedNode.data.handle}
                InputProps={{readOnly: true}}
                size="small"
                helperText={t(
                  'resourceServers:mcp.detail.handleHint',
                  'Stable identifier for this {{kind}}, used to build the permission scope.',
                  {kind: kindNoun},
                )}
                sx={{'& input': {fontFamily: 'monospace', fontSize: '0.875rem'}}}
              />
            </FormControl>

            <FormControl fullWidth>
              <FormLabel htmlFor="mcp-detail-delimiter">
                {t('resourceServers:detail.fields.delimiter', 'Delimiter (immutable)')}
              </FormLabel>
              <TextField
                fullWidth
                id="mcp-detail-delimiter"
                value={resourceServer.delimiter}
                InputProps={{readOnly: true}}
                size="small"
                helperText={t(
                  'resourceServers:mcp.detail.delimiterHint',
                  'Separates segments in the permission scope. Defined by the resource server.',
                )}
                sx={{'& input': {fontFamily: 'monospace', fontSize: '0.875rem'}}}
              />
            </FormControl>
          </>
        )}

        <FormControl fullWidth>
          <FormLabel>{t('resourceServers:detail.fields.description', 'Description')}</FormLabel>
          <TextField
            value={description}
            onChange={handleField(setDescription)}
            fullWidth
            size="small"
            multiline
            rows={3}
            disabled={isReadOnly}
            placeholder={
              isMcpNonServer
                ? t('resourceServers:mcp.detail.descriptionPlaceholder', 'Describe what this {{kind}} is for.', {
                    kind: kindNoun,
                  })
                : undefined
            }
          />
        </FormControl>
      </Stack>
      <Divider sx={{my: 1, borderColor: 'divider', borderBottomWidth: 2}} />

      {/* MCP non-server: read-only Permission field */}
      {isMcpNonServer ? (
        <FormControl fullWidth>
          <FormLabel htmlFor="mcp-detail-permission">{t('resourceServers:detail.permission', 'Permission')}</FormLabel>
          <TextField
            fullWidth
            id="mcp-detail-permission"
            value={permission}
            InputProps={{
              readOnly: true,
              endAdornment: (
                <InputAdornment position="end">
                  <Tooltip title={copiedPermission ? t('common:copied', 'Copied!') : t('common:copy', 'Copy')}>
                    <IconButton
                      edge="end"
                      size="small"
                      onClick={handleCopyPermission}
                      aria-label={t('common:copy', 'Copy')}
                    >
                      {copiedPermission ? <Check size={16} /> : <Copy size={16} />}
                    </IconButton>
                  </Tooltip>
                </InputAdornment>
              ),
            }}
            size="small"
            helperText={t(
              'resourceServers:mcp.detail.permissionScopeHelp',
              'Built from the resource path and the name, joined by the delimiter.',
            )}
            sx={{'& input': {fontFamily: 'monospace', fontSize: '0.875rem'}}}
          />
        </FormControl>
      ) : (
        <>
          {selectedNode.type !== 'server' && (
            <>
              <Box sx={{display: 'flex', alignItems: 'center', gap: 1}}>
                <Typography variant="body2" color="text.secondary">
                  {t('resourceServers:detail.permission', 'Permission')}
                </Typography>
                <Chip
                  label={permission}
                  size="small"
                  variant="outlined"
                  sx={{fontFamily: 'monospace', fontSize: '0.78rem'}}
                />
                <Tooltip title={copiedPermission ? t('common:copied', 'Copied!') : t('common:copy', 'Copy permission')}>
                  <IconButton size="small" sx={{p: 0.25}} onClick={handleCopyPermission}>
                    {copiedPermission ? <Check size={14} /> : <Copy size={14} />}
                  </IconButton>
                </Tooltip>
              </Box>

              <Box>
                <Typography variant="caption" color="text.secondary">
                  {t('resourceServers:detail.fields.handle', 'Handle (immutable)')}
                </Typography>
                <Box>
                  <Chip
                    label={selectedNode.data.handle}
                    size="small"
                    sx={{fontFamily: 'monospace', fontSize: '0.75rem', mt: 0.5}}
                  />
                </Box>
              </Box>
            </>
          )}

          {selectedNode.type === 'server' && (
            <Box>
              <Typography variant="caption" color="text.secondary">
                {t('resourceServers:detail.fields.delimiter', 'Delimiter (immutable)')}
              </Typography>
              <Box>
                <Chip
                  label={selectedNode.data.delimiter}
                  size="small"
                  sx={{fontFamily: 'monospace', fontSize: '0.75rem', mt: 0.5}}
                />
              </Box>
            </Box>
          )}

          {selectedNode.type === 'server' && (
            <FormControl fullWidth>
              <FormLabel>{t('resourceServers:detail.fields.identifier', 'Identifier')}</FormLabel>
              <TextField
                value={identifier}
                onChange={handleField(setIdentifier)}
                fullWidth
                size="small"
                helperText={t(
                  'resourceServers:detail.fields.identifierHint',
                  'Used as audience parameter in OAuth2 flows.',
                )}
                disabled={isReadOnly}
              />
            </FormControl>
          )}
        </>
      )}

      {!isReadOnly && dirty && (
        <Box sx={{mt: 'auto', pt: 2, display: 'flex', gap: 1, justifyContent: 'flex-end'}}>
          <Button variant="outlined" size="small" onClick={resetForm} disabled={isPending}>
            {t('common:actions.reset', 'Reset')}
          </Button>
          <Button variant="contained" size="small" onClick={handleSave} disabled={isPending}>
            {isPending ? t('common:saving', 'Saving…') : t('common:save', 'Save')}
          </Button>
        </Box>
      )}
    </>
  );

  if (isMcpNonServer) {
    return (
      <Box sx={{display: 'flex', flexDirection: 'column', height: '100%'}}>
        <Box
          sx={{
            height: PANEL_HEADER_ROW_HEIGHT * 2,
            px: 2,
            display: 'flex',
            flexDirection: 'column',
            justifyContent: 'center',
            flexShrink: 0,
            borderBottom: '1px solid',
            borderColor: 'divider',
          }}
        >
          <Typography variant="h5">{name}</Typography>
          {resolveMcpKindLabel() && (
            <Stack direction="row" alignItems="center" spacing={0.5} sx={{mt: 0.5, color: 'text.disabled'}}>
              {getActionKindIcon(selectedNode.type === 'resource' ? undefined : selectedNode.data.kind, 14)}
              <Typography variant="body2" color="inherit">
                {resolveMcpKindLabel()}
              </Typography>
            </Stack>
          )}
        </Box>
        <Box sx={{p: 2, display: 'flex', flexDirection: 'column', gap: 2, flex: 1, overflowY: 'auto'}}>
          {formContent}
        </Box>
      </Box>
    );
  }

  return (
    <Box sx={{display: 'flex', flexDirection: 'column', gap: 2, p: 2, height: '100%', overflowY: 'auto'}}>
      {formContent}
    </Box>
  );
}

export default function ResourceDetailPanel({
  selectedNode,
  resourceServer,
  onRefresh,
}: ResourceDetailPanelProps): JSX.Element {
  const {t} = useTranslation();

  if (!selectedNode) {
    return (
      <Box
        sx={{
          height: '100%',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          color: 'text.disabled',
        }}
      >
        <Typography variant="body2">
          {t('resourceServers:detail.selectNode', 'Select a node from the tree to view its details.')}
        </Typography>
      </Box>
    );
  }

  return (
    <DetailForm
      key={selectedNode.id}
      selectedNode={selectedNode}
      resourceServer={resourceServer}
      onRefresh={onRefresh}
    />
  );
}
