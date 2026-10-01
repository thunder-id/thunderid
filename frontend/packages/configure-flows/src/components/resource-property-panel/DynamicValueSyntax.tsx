// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Typography} from '@wso2/oxygen-ui';
import {type ReactElement} from 'react';

/**
 * Props interface of {@link DynamicValueSyntax}
 */
export interface DynamicValueSyntaxProps {
  /**
   * The raw value, e.g. `{{meta(application.name)}}` or `{{t(flowI18n:key)}}`.
   */
  value: string;
}

/**
 * Regular expression to parse a dynamic value token: {{type(key)}}
 */
const DYNAMIC_TOKEN_RE = /^\{\{([a-zA-Z]+)\(([^)]*)\)\}\}$/;

/**
 * Renders a `{{type(key)}}` string with syntax highlighting:
 * - `{{` / `}}` delimiters — muted
 * - type name (e.g. `meta`, `t`) — primary accent
 * - `(` / `)` — muted
 * - key — default text
 *
 * Falls back to plain monospace text for unrecognised patterns.
 */
function DynamicValueSyntax({value}: DynamicValueSyntaxProps): ReactElement {
  const match = DYNAMIC_TOKEN_RE.exec(value.trim());

  if (!match) {
    return (
      <Typography variant="body2" sx={{wordBreak: 'break-word', fontFamily: 'monospace'}}>
        {value}
      </Typography>
    );
  }

  const [, typeName, key] = match;

  return (
    <Box component="span" sx={{fontFamily: 'monospace', fontSize: '0.875rem', lineHeight: 1.5, wordBreak: 'break-all'}}>
      <Box component="span" sx={{color: 'text.disabled'}}>
        {'{{'}
      </Box>
      <Box component="span" sx={{color: 'primary.main', fontWeight: 600}}>
        {typeName}
      </Box>
      <Box component="span" sx={{color: 'text.disabled'}}>
        (
      </Box>
      <Box component="span" sx={{color: 'text.primary'}}>
        {key}
      </Box>
      <Box component="span" sx={{color: 'text.disabled'}}>
        {')}}'}
      </Box>
    </Box>
  );
}

export default DynamicValueSyntax;
