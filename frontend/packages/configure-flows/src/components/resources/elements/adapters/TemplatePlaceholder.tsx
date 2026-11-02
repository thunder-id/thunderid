// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box} from '@wso2/oxygen-ui';
import {type ReactNode} from 'react';

type Segment = {kind: 'text'; content: string} | {kind: 'template'; typeName: string; key: string};

/**
 * Splits a string into plain-text and `{{type(key)}}` template segments.
 */
function parseSegments(value: string): Segment[] {
  const segments: Segment[] = [];
  const re = /\{\{\s*([a-zA-Z]+)\s*\(\s*([^)]*?)\s*\)\s*\}\}/g;

  let lastIndex = 0;
  let match: RegExpExecArray | null = re.exec(value);
  while (match !== null) {
    if (match.index > lastIndex) {
      segments.push({kind: 'text', content: value.slice(lastIndex, match.index)});
    }
    segments.push({kind: 'template', typeName: match[1].trim(), key: match[2].trim()});
    lastIndex = match.index + match[0].length;
    match = re.exec(value);
  }

  if (lastIndex < value.length) {
    segments.push({kind: 'text', content: value.slice(lastIndex)});
  }

  return segments;
}

/**
 * Returns true when `value` contains at least one `{{type(key)}}` template literal.
 */
// eslint-disable-next-line react-refresh/only-export-components
export function containsTemplateLiteral(value: string | undefined): boolean {
  if (!value) return false;
  return /\{\{\s*[a-zA-Z]+\s*\([^)]*\)\s*\}\}/.test(value);
}

/**
 * Props interface of {@link TemplatePlaceholder}
 */
export interface TemplatePlaceholderProps {
  /**
   * The string to render. May be plain text, a full template (`{{type(key)}}`),
   * or mixed content (`"Hello {{meta(name)}}"").
   */
  value: string;
  /**
   * Optional i18n translation function. When provided, `{{t(key)}}` segments are
   * resolved to their translated string instead of shown as a badge.
   */
  t?: (key: string) => string;
}

/**
 * Renders a string that may contain `{{type(key)}}` template literals with inline
 * highlighted badges in the flow builder canvas, making dynamic values stand out
 * from static text. Mixed content like `"Hello {{meta(name)}}"` is handled correctly.
 *
 * `{{t(key)}}` segments are resolved via the optional `t` prop; all other template
 * types are shown as a styled badge.
 */
function TemplatePlaceholder({value, t}: TemplatePlaceholderProps): ReactNode {
  const segments = parseSegments(value);
  const translate = t ?? ((k: string) => k);

  if (segments.length === 0) {
    return value;
  }

  return segments.map((seg) => {
    const key = `${seg.kind}-${seg.kind === 'text' ? seg.content : `${seg.typeName}-${seg.key}`}`;
    if (seg.kind === 'text') {
      return seg.content;
    }
    // Resolve i18n patterns to translated text when t is available.
    if (seg.typeName === 't') {
      return translate(seg.key);
    }
    return (
      <Box
        key={key}
        component="span"
        sx={{
          display: 'inline-flex',
          alignItems: 'center',
          px: 0.875,
          py: 0.125,
          borderRadius: '4px',
          bgcolor: 'action.hover',
          border: '1px solid',
          borderColor: 'divider',
          fontFamily: 'monospace',
          fontSize: '0.8em',
          lineHeight: 1.6,
          verticalAlign: 'middle',
          whiteSpace: 'nowrap',
        }}
      >
        <Box component="span" sx={{color: 'primary.main', fontWeight: 600}}>
          {seg.typeName}
        </Box>
        <Box component="span" sx={{color: 'text.disabled'}}>
          (
        </Box>
        <Box component="span" sx={{color: 'text.primary'}}>
          {seg.key}
        </Box>
        <Box component="span" sx={{color: 'text.disabled'}}>
          )
        </Box>
      </Box>
    );
  });
}

export default TemplatePlaceholder;
