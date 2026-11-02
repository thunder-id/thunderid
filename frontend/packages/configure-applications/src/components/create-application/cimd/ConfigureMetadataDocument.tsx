// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  Alert,
  Button,
  Chip,
  CircularProgress,
  FormControl,
  FormLabel,
  Stack,
  TextField,
  Typography,
  useTheme,
} from '@wso2/oxygen-ui';
import {Lightbulb} from '@wso2/oxygen-ui-icons-react';
import type {ChangeEvent, JSX} from 'react';
import {useEffect, useState} from 'react';
import {useTranslation} from 'react-i18next';
import CimdPreviewCard from './CimdPreviewCard';
import usePreviewCimdDocument from '../../../api/usePreviewCimdDocument';
import type {CimdPreview} from '../../../models/cimd';
import getCimdErrorMessage from '../../../utils/getCimdErrorMessage';

/**
 * Props for the {@link ConfigureMetadataDocument} component.
 *
 * @public
 */
export interface ConfigureMetadataDocumentProps {
  /** The approved document, or null when none has been fetched for the current URL. */
  preview: CimdPreview | null;
  /** Invoked with the validated document, or null when the URL changes. */
  onPreviewChange: (preview: CimdPreview | null) => void;
  /** Invoked whenever the step's readiness changes. */
  onReadyChange?: (isReady: boolean) => void;
  /** Known clients offered as quick picks, from the template's `metadataDocumentClients`. */
  knownClients?: {name: string; clientId: string}[];
  /** Hides the step heading when the form is embedded in another step, such as the MCP Client identity step. */
  embedded?: boolean;
}

/**
 * The Client ID Metadata Document template's first step: the administrator enters the document URL
 * and reviews what the document declares before continuing.
 *
 * @public
 */
export default function ConfigureMetadataDocument({
  preview,
  onPreviewChange,
  onReadyChange = undefined,
  knownClients = [],
  embedded = false,
}: ConfigureMetadataDocumentProps): JSX.Element {
  const {t} = useTranslation();
  const theme = useTheme();
  const previewDocument = usePreviewCimdDocument();
  const [documentUrl, setDocumentUrl] = useState<string>(preview?.clientId ?? '');

  useEffect((): void => {
    onReadyChange?.(preview !== null);
  }, [preview, onReadyChange]);

  const fetchDocument = (url: string): void => {
    onPreviewChange(null);
    previewDocument.mutate(url.trim(), {onSuccess: (data) => onPreviewChange(data)});
  };

  const handleUrlChange = (event: ChangeEvent<HTMLInputElement>): void => {
    setDocumentUrl(event.target.value);
    previewDocument.reset();
    onPreviewChange(null);
  };

  const handleQuickPick = (url: string): void => {
    setDocumentUrl(url);
    fetchDocument(url);
  };

  return (
    <Stack direction="column" spacing={2} data-testid="application-configure-metadata-document">
      {!embedded && (
        <Stack direction="column" spacing={0.5}>
          <Typography variant="h1" gutterBottom>
            {t('applications:cimd.document.title', "Enter the client's metadata document")}
          </Typography>
          <Stack direction="row" alignItems="center" spacing={1}>
            <Lightbulb size={20} color={theme.vars?.palette.warning.main} />
            <Typography variant="body2" color="text.secondary">
              {t(
                'applications:cimd.document.subtitle',
                "The client's URL is its client ID. Its name and sign-in details come from its vendor, with no setup in the client itself.",
              )}
            </Typography>
          </Stack>
        </Stack>
      )}

      <FormControl fullWidth>
        <FormLabel htmlFor="cimd-document-url">
          {t('applications:cimd.document.url.label', 'Metadata document URL')}
        </FormLabel>
        <Stack direction="row" spacing={1}>
          <TextField
            fullWidth
            id="cimd-document-url"
            value={documentUrl}
            onChange={handleUrlChange}
            onKeyDown={(event) => {
              if (event.key === 'Enter' && documentUrl.trim()) fetchDocument(documentUrl);
            }}
            placeholder="https://example.com/oauth/client-metadata.json"
            error={previewDocument.isError}
            inputProps={{'data-testid': 'cimd-document-url-input'}}
          />
          <Button
            variant="outlined"
            onClick={() => fetchDocument(documentUrl)}
            disabled={!documentUrl.trim() || previewDocument.isPending}
            sx={{whiteSpace: 'nowrap', minWidth: 150}}
            data-testid="cimd-fetch-document-button"
          >
            {previewDocument.isPending ? (
              <CircularProgress size={18} />
            ) : (
              t('applications:cimd.document.url.fetch', 'Fetch document')
            )}
          </Button>
        </Stack>
      </FormControl>

      {knownClients.length > 0 && (
        <Stack direction="row" spacing={1} alignItems="center" flexWrap="wrap" useFlexGap>
          <Typography variant="body2" color="text.secondary">
            {t('applications:cimd.document.quickPicks', 'Known clients:')}
          </Typography>
          {knownClients.map((client) => (
            <Chip
              key={client.clientId}
              label={client.name}
              size="small"
              variant={documentUrl === client.clientId ? 'filled' : 'outlined'}
              onClick={() => handleQuickPick(client.clientId)}
            />
          ))}
        </Stack>
      )}

      {previewDocument.isError && (
        <Alert severity="error" data-testid="cimd-preview-error">
          {getCimdErrorMessage(previewDocument.error, t)}
        </Alert>
      )}

      {preview && <CimdPreviewCard preview={preview} />}
    </Stack>
  );
}
