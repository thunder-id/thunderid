// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Typography} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';

export interface ScimPayloadPreviewProps {
  payload: Record<string, unknown>;
}

// VS Code dark theme token colours — matches the palette used by JwtPreview's Monaco editor.
const COLORS = {
  key: '#9cdcfe',
  string: '#ce9178',
  number: '#b5cea8',
  boolean: '#569cd6',
  null: '#569cd6',
};

interface Token {
  text: string;
  color?: string;
}

/**
 * Tokenises a JSON string into coloured spans matching VS Code's dark theme,
 * so the preview palette stays consistent with the Monaco-based JwtPreview
 * used for token attributes — without taking @monaco-editor/react as a dep.
 */
function tokenize(json: string): Token[] {
  const tokens: Token[] = [];
  // Regex captures: object key, string value, number, boolean, or null.
  const re =
    /"((?:[^"\\]|\\.)*)"(?=\s*:)/g.source + // key (without the colon)
    '|' +
    /"(?:[^"\\]|\\.)*"/g.source + // string value
    '|' +
    /\b-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?\b/.source + // number
    '|' +
    /\b(?:true|false)\b/.source + // boolean
    '|' +
    /\bnull\b/.source; // null

  const pattern = new RegExp(re, 'g');
  let lastIndex = 0;

  for (const match of json.matchAll(pattern)) {
    if (match.index > lastIndex) {
      tokens.push({text: json.slice(lastIndex, match.index)});
    }

    const raw = match[0];
    let color: string;

    if (match[1] !== undefined) {
      color = COLORS.key;
    } else if (raw.startsWith('"')) {
      color = COLORS.string;
    } else if (raw === 'true' || raw === 'false') {
      color = COLORS.boolean;
    } else if (raw === 'null') {
      color = COLORS.null;
    } else {
      color = COLORS.number;
    }

    tokens.push({text: raw, color});
    lastIndex = match.index + raw.length;
  }

  if (lastIndex < json.length) {
    tokens.push({text: json.slice(lastIndex)});
  }

  return tokens;
}

/**
 * Syntax-highlighted JSON preview of the SCIM payload this mapping would produce.
 * Uses VS Code dark theme colours (matching JwtPreview) without a Monaco dependency.
 */
export default function ScimPayloadPreview({payload}: ScimPayloadPreviewProps): JSX.Element {
  const {t} = useTranslation();
  const json = JSON.stringify(payload, null, 2);
  const tokens = tokenize(json);

  return (
    <Box
      sx={{
        bgcolor: '#1e1e1e',
        border: 1,
        borderColor: 'divider',
        borderRadius: 1,
        p: 2,
        minWidth: 0,
        overflow: 'auto',
      }}
    >
      <Typography
        variant="caption"
        sx={{color: '#9d9d9d', textTransform: 'uppercase', letterSpacing: 1, display: 'block', mb: 1}}
      >
        {t('userTypes:edit.scimMapping.previewLabel', 'SCIM Payload Preview')}
      </Typography>
      <Box
        component="pre"
        data-testid="scim-payload-preview"
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
        {tokens.map((token, i) =>
          token.color ? (
            // eslint-disable-next-line react/no-array-index-key
            <span key={i} style={{color: token.color}}>
              {token.text}
            </span>
          ) : (
            token.text
          ),
        )}
      </Box>
    </Box>
  );
}
