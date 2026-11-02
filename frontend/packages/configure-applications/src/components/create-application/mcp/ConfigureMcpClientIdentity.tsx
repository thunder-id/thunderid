// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  Alert,
  Box,
  Card,
  CardActionArea,
  CardContent,
  CircularProgress,
  FormControlLabel,
  Radio,
  RadioGroup,
  Stack,
  Typography,
  useTheme,
} from '@wso2/oxygen-ui';
import {FileBadge, Lightbulb, Wrench} from '@wso2/oxygen-ui-icons-react';
import type {ChangeEvent, JSX, ReactNode} from 'react';
import {useEffect, useState} from 'react';
import {useTranslation} from 'react-i18next';
import usePreviewCimdDocument from '../../../api/usePreviewCimdDocument';
import {McpClientIdentityModes} from '../../../models/cimd';
import type {CimdPreview, McpClientIdentityMode} from '../../../models/cimd';
import getCimdErrorMessage from '../../../utils/getCimdErrorMessage';
import ConfigureMetadataDocument from '../cimd/ConfigureMetadataDocument';

/**
 * Props for the {@link ConfigureMcpClientIdentity} component.
 *
 * @public
 */
export interface ConfigureMcpClientIdentityProps {
  /** How the client identifies itself, or null until the administrator picks one of the other options. */
  mode: McpClientIdentityMode | null;
  /** Invoked when the administrator picks one of the other options. */
  onModeChange: (mode: McpClientIdentityMode) => void;
  /** The approved document for "Another client", or null when none has been fetched for the current URL. */
  preview: CimdPreview | null;
  /** Invoked with the validated document for "Another client", or null when the URL changes. */
  onPreviewChange: (preview: CimdPreview | null) => void;
  /** Invoked with a known client's validated document, to register it without further steps. */
  onKnownClientSelect: (preview: CimdPreview) => void;
  /** Whether a known client's registration is in progress. */
  isRegistering?: boolean;
  /** Invoked whenever the step's readiness changes. */
  onReadyChange?: (isReady: boolean) => void;
  /** Known MCP clients shown as tiles, from the template's `metadataDocumentClients`. */
  knownClients?: {name: string; clientId: string}[];
}

/**
 * The mcp-client template's first step. Known clients come first as tiles: choosing one retrieves and
 * validates its Client ID Metadata Document and registers it straight away. Below them, the administrator
 * can register another client from its metadata document URL, or configure a client by hand.
 *
 * @public
 */
export default function ConfigureMcpClientIdentity({
  mode,
  onModeChange,
  preview,
  onPreviewChange,
  onKnownClientSelect,
  isRegistering = false,
  onReadyChange = undefined,
  knownClients = [],
}: ConfigureMcpClientIdentityProps): JSX.Element {
  const {t} = useTranslation();
  const theme = useTheme();
  const previewKnownClient = usePreviewCimdDocument();
  const [selectedClientId, setSelectedClientId] = useState<string | null>(null);

  // The embedded document form reports its own readiness; the manual mode is ready at once.
  useEffect((): void => {
    if (mode === null) onReadyChange?.(false);
    if (mode === McpClientIdentityModes.MANUAL) onReadyChange?.(true);
  }, [mode, onReadyChange]);

  const handleKnownClientClick = (clientId: string): void => {
    if (previewKnownClient.isPending || isRegistering) return;
    setSelectedClientId(clientId);
    previewKnownClient.mutate(clientId, {onSuccess: (data) => onKnownClientSelect(data)});
  };

  const options: {value: McpClientIdentityMode; icon: ReactNode; title: string; description: string}[] = [
    {
      value: McpClientIdentityModes.METADATA_DOCUMENT,
      icon: <FileBadge size={20} />,
      title: t('applications:cimd.identity.document.title', 'Another client with a CIMD'),
      description: t(
        'applications:cimd.identity.document.description',
        "Enter the client's metadata document URL. Its identity and sign-in details come from its vendor.",
      ),
    },
    {
      value: McpClientIdentityModes.MANUAL,
      icon: <Wrench size={20} />,
      title: t('applications:cimd.identity.manual.title', "I'll configure it myself"),
      description: t(
        'applications:cimd.identity.manual.description',
        'For your own MCP clients. You set the redirect URIs and credentials.',
      ),
    },
  ];

  const title = t('applications:cimd.identity.title', 'Which MCP client are you connecting?');
  const isBusy = previewKnownClient.isPending || isRegistering;

  return (
    <Stack direction="column" spacing={3} data-testid="application-configure-mcp-client-identity">
      <Stack direction="column" spacing={0.5}>
        <Typography variant="h1" gutterBottom>
          {title}
        </Typography>
        <Stack direction="row" alignItems="center" spacing={1}>
          <Lightbulb size={20} color={theme.vars?.palette.warning.main} />
          <Typography variant="body2" color="text.secondary">
            {t(
              'applications:cimd.identity.subtitle',
              'Known clients register in one step from the metadata document their vendor publishes.',
            )}
          </Typography>
        </Stack>
      </Stack>

      {knownClients.length > 0 && (
        <Stack spacing={1.5}>
          <Typography variant="h6">{t('applications:cimd.identity.knownClients', 'Known clients')}</Typography>
          <Box sx={{display: 'grid', gridTemplateColumns: {xs: '1fr', sm: 'repeat(3, minmax(0, 1fr))'}, gap: 2}}>
            {knownClients.map((client) => {
              const isLoading = isBusy && selectedClientId === client.clientId;
              return (
                <Card key={client.clientId} variant="outlined">
                  <CardActionArea
                    onClick={() => handleKnownClientClick(client.clientId)}
                    disabled={isBusy}
                    data-testid={`cimd-known-client-${client.name}`}
                    sx={{height: '100%', '&:hover': {bgcolor: 'action.hover'}}}
                  >
                    <CardContent>
                      <Stack direction="row" spacing={1} alignItems="center" justifyContent="space-between">
                        <Typography variant="h6">{client.name}</Typography>
                        {isLoading && <CircularProgress size={18} />}
                      </Stack>
                    </CardContent>
                  </CardActionArea>
                </Card>
              );
            })}
          </Box>
          {previewKnownClient.isError && (
            <Alert severity="error" data-testid="cimd-known-client-error">
              {getCimdErrorMessage(previewKnownClient.error, t)}
            </Alert>
          )}
        </Stack>
      )}

      <Stack spacing={1.5}>
        {knownClients.length > 0 && (
          <Typography variant="h6">{t('applications:cimd.identity.otherClients', 'Other clients')}</Typography>
        )}
        <RadioGroup
          aria-label={title}
          value={mode ?? ''}
          onChange={(event: ChangeEvent<HTMLInputElement>) => onModeChange(event.target.value as McpClientIdentityMode)}
        >
          <Stack direction="column" spacing={2}>
            {options.map((option) => {
              const isSelected = mode === option.value;
              return (
                <Card key={option.value} variant="outlined" onClick={() => onModeChange(option.value)}>
                  <CardActionArea
                    sx={{
                      border: 1,
                      borderColor: isSelected ? 'primary.main' : 'divider',
                      '&:hover': {
                        borderColor: 'primary.main',
                        bgcolor: isSelected ? 'action.selected' : 'action.hover',
                      },
                    }}
                  >
                    <CardContent>
                      <Stack direction="row" spacing={2} alignItems="flex-start">
                        <FormControlLabel
                          value={option.value}
                          control={<Radio />}
                          label=""
                          sx={{m: 0}}
                          onClick={(e) => e.stopPropagation()}
                        />
                        <Box sx={{flex: 1}}>
                          <Stack direction="row" spacing={1} alignItems="center" sx={{mb: 1}}>
                            {option.icon}
                            <Typography variant="h6">{option.title}</Typography>
                          </Stack>
                          <Typography variant="body2" color="text.secondary">
                            {option.description}
                          </Typography>
                        </Box>
                      </Stack>
                    </CardContent>
                  </CardActionArea>
                </Card>
              );
            })}
          </Stack>
        </RadioGroup>
      </Stack>

      {mode === McpClientIdentityModes.METADATA_DOCUMENT && (
        <ConfigureMetadataDocument
          embedded
          preview={preview}
          onPreviewChange={onPreviewChange}
          onReadyChange={onReadyChange}
        />
      )}
    </Stack>
  );
}
