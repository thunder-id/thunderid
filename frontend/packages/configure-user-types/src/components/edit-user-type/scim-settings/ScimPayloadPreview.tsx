// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Typography} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';

export interface ScimPayloadPreviewProps {
  payload: Record<string, unknown>;
}

/**
 * Plain, dependency-free JSON preview of the SCIM user payload this mapping would produce.
 * Deliberately not syntax-highlighted (unlike the Monaco-based JwtPreview used for token
 * attributes) — this package does not depend on @monaco-editor/react.
 */
export default function ScimPayloadPreview({payload}: ScimPayloadPreviewProps): JSX.Element {
  const {t} = useTranslation();
  const json = JSON.stringify(payload, null, 2);

  return (
    <Box
      sx={{
        bgcolor: '#1e1e1e',
        border: 1,
        borderColor: 'divider',
        borderRadius: 1,
        p: 2,
        height: '100%',
        minWidth: 0,
        overflow: 'auto',
      }}
    >
      <Typography
        variant="caption"
        sx={{color: '#9d9d9d', textTransform: 'uppercase', letterSpacing: 1, display: 'block', mb: 1}}
      >
        {t('userTypes:edit.scimMapping.previewLabel', 'SCIM User Preview')}
      </Typography>
      <Box
        component="pre"
        sx={{
          m: 0,
          color: '#d4d4d4',
          fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace',
          fontSize: 13,
          lineHeight: 1.6,
          whiteSpace: 'pre-wrap',
          wordBreak: 'break-word',
        }}
      >
        {json}
      </Box>
    </Box>
  );
}
