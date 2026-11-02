// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  Alert,
  Box,
  Button,
  FormControl,
  FormLabel,
  IconButton,
  Stack,
  TextField,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import {Plus, Trash} from '@wso2/oxygen-ui-icons-react';
import type {ChangeEvent, JSX} from 'react';
import {useEffect, useState} from 'react';
import {useTranslation} from 'react-i18next';
import McpInspectorLogo from './McpInspectorLogo';
import validateMcpRedirectUri from '../../../utils/validateMcpRedirectUri';

const MCP_INSPECTOR_CALLBACK_URI = 'http://localhost:6274/oauth/callback';

/**
 * Props for the {@link ConfigureMcpConnection} component.
 *
 * @public
 */
export interface ConfigureMcpConnectionProps {
  /**
   * The currently configured redirect URIs
   */
  redirectUris: string[];

  /**
   * Callback function invoked when the redirect URIs change
   */
  onRedirectUrisChange: (uris: string[]) => void;

  /**
   * Callback function to broadcast whether this step is ready to proceed
   */
  onReadyChange?: (isReady: boolean) => void;

  /**
   * When true, hides the title and subtitle so only the redirect URI editor is rendered —
   * used when this component is embedded inline within another step
   */
  compact?: boolean;
}

/**
 * React component that renders the Connection step of the mcp-client template's
 * creation flow, shown only for the user-delegated client type.
 *
 * Presents a redirect URI editor (add/remove/edit) validated against the MCP redirect URI
 * rule: each URI must be loopback (`http://localhost`, `http://127.0.0.1`, or `http://[::1]`)
 * or use HTTPS — wildcards are rejected anywhere in the URI, since the backend's create-time
 * validation rejects them too. A banner above the editor surfaces the MCP Inspector's default
 * callback URI with a quick-add affordance, mirroring the dev-server banner shown on the
 * standard Configuration step's URLs section. The step is ready only when at least one redirect
 * URI is present and every non-empty URI is valid.
 *
 * @param props - The component props
 * @param props.redirectUris - The currently configured redirect URIs
 * @param props.onRedirectUrisChange - Callback invoked when the redirect URIs change
 * @param props.onReadyChange - Optional callback to notify parent of step readiness
 * @param props.compact - When true, hides the title and subtitle for inline embedding
 *
 * @returns JSX element displaying the Connection step
 *
 * @example
 * ```tsx
 * import ConfigureMcpConnection from './ConfigureMcpConnection';
 *
 * function ConnectionStep() {
 *   const [uris, setUris] = useState<string[]>([]);
 *
 *   return (
 *     <ConfigureMcpConnection
 *       redirectUris={uris}
 *       onRedirectUrisChange={setUris}
 *       onReadyChange={(isReady) => console.log('Step ready:', isReady)}
 *     />
 *   );
 * }
 * ```
 *
 * @public
 */
export default function ConfigureMcpConnection({
  redirectUris,
  onRedirectUrisChange,
  onReadyChange = undefined,
  compact = false,
}: ConfigureMcpConnectionProps): JSX.Element {
  const {t} = useTranslation();

  const removeUriLabel = t('applications:onboarding.mcp.connection.redirectUris.remove', 'Remove redirect URI');

  const [rows, setRows] = useState<string[]>(() => (redirectUris.length > 0 ? redirectUris : ['']));
  const [uriErrors, setUriErrors] = useState<Record<number, string>>({});

  useEffect((): void => {
    const nonEmptyUris = rows.filter((uri) => uri.trim() !== '');
    const isReady = nonEmptyUris.length > 0 && nonEmptyUris.every((uri) => validateMcpRedirectUri(uri).valid);
    onReadyChange?.(isReady);
  }, [rows, onReadyChange]);

  // Rows keep the raw, untrimmed value so the input doesn't jump around while the user is
  // typing; the value handed back to the parent (and eventually submitted) is trimmed so
  // whitespace never leaks into the persisted redirect URIs.
  const emitChange = (newRows: string[]): void => {
    onRedirectUrisChange(newRows.map((uri) => uri.trim()));
  };

  const handleUriChange = (index: number, value: string): void => {
    const newRows = [...rows];
    newRows[index] = value;
    setRows(newRows);
    emitChange(newRows);

    setUriErrors((prev) => {
      if (!(index in prev)) return prev;
      const newErrors = {...prev};
      delete newErrors[index];
      return newErrors;
    });
  };

  const handleUriBlur = (index: number): void => {
    const result = validateMcpRedirectUri(rows[index]);
    setUriErrors((prev) => {
      const newErrors = {...prev};
      if (result.valid) {
        delete newErrors[index];
      } else {
        newErrors[index] = t(result.errorKey ?? 'applications:onboarding.mcp.connection.redirectUris.error.invalid');
      }
      return newErrors;
    });
  };

  const handleAddUri = (): void => {
    const newRows = [...rows, ''];
    setRows(newRows);
    emitChange(newRows);
  };

  const handleRemoveUri = (index: number): void => {
    const filteredRows = rows.filter((_, i) => i !== index);
    const newRows = filteredRows.length > 0 ? filteredRows : [''];
    setRows(newRows);
    emitChange(newRows);

    setUriErrors((prev) => {
      const reindexed: Record<number, string> = {};
      Object.entries(prev).forEach(([key, value]) => {
        const oldIndex = parseInt(key, 10);
        if (oldIndex > index) {
          reindexed[oldIndex - 1] = value;
        } else if (oldIndex < index) {
          reindexed[oldIndex] = value;
        }
      });
      return reindexed;
    });
  };

  // Fills the first empty row if one is present instead of appending, so quick-adding into an
  // otherwise-blank editor doesn't leave a dangling empty field above the newly added URI.
  const handleAddInspectorUri = (): void => {
    if (rows.some((uri) => uri.trim() === MCP_INSPECTOR_CALLBACK_URI)) {
      return;
    }
    const emptyIndex = rows.findIndex((uri) => uri.trim() === '');
    const newRows =
      emptyIndex !== -1
        ? rows.map((uri, i) => (i === emptyIndex ? MCP_INSPECTOR_CALLBACK_URI : uri))
        : [...rows, MCP_INSPECTOR_CALLBACK_URI];
    setRows(newRows);
    emitChange(newRows);
  };

  return (
    <Stack spacing={4} data-testid="application-configure-mcp-connection">
      {!compact && (
        <Stack spacing={0.5}>
          <Typography variant="h1">{t('applications:onboarding.mcp.connection.title')}</Typography>
          <Typography variant="body1" color="text.secondary">
            {t('applications:onboarding.mcp.connection.subtitle')}
          </Typography>
        </Stack>
      )}

      <Alert severity="info" icon={<McpInspectorLogo />} data-testid="application-mcp-inspector-hint">
        {t('applications:onboarding.mcp.connection.inspectorHint.prefix', 'Testing with MCP Inspector? Use')}{' '}
        <code>{MCP_INSPECTOR_CALLBACK_URI}</code>{' '}
        <Box
          component="span"
          onClick={handleAddInspectorUri}
          sx={{color: 'primary.main', fontWeight: 600, cursor: 'pointer', textDecoration: 'underline'}}
        >
          {t('applications:onboarding.configure.details.devServer.addToRedirect', 'Add it to redirect URIs')}
        </Box>
      </Alert>

      <FormControl fullWidth required>
        <FormLabel htmlFor="mcp-redirect-uris-section">
          {t('applications:onboarding.mcp.connection.redirectUris.label')}
        </FormLabel>
        <Typography variant="caption" color="text.secondary" sx={{display: 'block', mb: 2}}>
          {t('applications:onboarding.mcp.connection.redirectUris.hint')}
        </Typography>

        <Stack spacing={2} id="mcp-redirect-uris-section">
          {rows.map((uri, index) => (
            // IMPORTANT: Do not remove the suppression since it affects functionality.
            // eslint-disable-next-line react/no-array-index-key
            <Stack key={index} direction="row" spacing={1} alignItems="flex-start">
              <FormControl fullWidth sx={{flex: 1}}>
                <TextField
                  fullWidth
                  id={`mcp-redirect-uri-${index}-input`}
                  value={uri}
                  onChange={(e: ChangeEvent<HTMLInputElement>) => handleUriChange(index, e.target.value)}
                  onBlur={() => handleUriBlur(index)}
                  error={!!uriErrors[index]}
                  helperText={uriErrors[index]}
                  placeholder="http://localhost:8080/callback"
                  sx={{'& input': {fontFamily: 'monospace', fontSize: '0.875rem'}}}
                />
              </FormControl>
              <Tooltip title={removeUriLabel}>
                <IconButton
                  aria-label={removeUriLabel}
                  onClick={() => handleRemoveUri(index)}
                  color="error"
                  sx={{mt: 1}}
                >
                  <Trash size={20} />
                </IconButton>
              </Tooltip>
            </Stack>
          ))}

          <Box>
            <Button variant="text" color="primary" size="small" startIcon={<Plus size={16} />} onClick={handleAddUri}>
              {t('applications:onboarding.mcp.connection.redirectUris.addUri')}
            </Button>
          </Box>
        </Stack>
      </FormControl>
    </Stack>
  );
}
