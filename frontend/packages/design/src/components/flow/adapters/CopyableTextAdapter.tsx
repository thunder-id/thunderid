// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, IconButton, InputAdornment, TextField, Typography} from '@wso2/oxygen-ui';
import {Check, Copy, Eye, EyeOff} from '@wso2/oxygen-ui-icons-react';
import {useCallback, useEffect, useRef, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import type {FlowComponent} from '../../../models/flow';

/**
 * Props for the CopyableTextAdapter component.
 */
interface CopyableTextAdapterProps {
  additionalData?: Record<string, unknown>;
  component: FlowComponent;
  resolve: (template: string | undefined) => string | undefined;
}

/**
 * Adapter component to render a copyable text field within a flow. It displays a label (if provided) and a value
 * with a copy-to-clipboard button. The value is sourced from the `additionalData` using the `source` key defined
 * in the component configuration. When the copy button is clicked, it attempts to copy the value to the clipboard
 * and provides feedback to the user.
 *
 * The copy control is an icon button inside the field in both layouts. When the component is `masked`, the value
 * sits in a read-only password field that can be revealed and copied, so a secret is copyable without being readable
 * at a glance.
 *
 * @param {CopyableTextAdapterProps} props - The properties for the adapter, including the flow component
 * configuration, the resolve function for template strings, and any additional data needed to source the value.
 * @returns {JSX.Element} The rendered copyable text field with label and copy button.
 */
export default function CopyableTextAdapter({
  component,
  resolve,
  additionalData = undefined,
}: CopyableTextAdapterProps): JSX.Element {
  const {t} = useTranslation();
  const [copied, setCopied] = useState(false);
  const [isRevealed, setIsRevealed] = useState(false);
  const copyTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(
    () => () => {
      if (copyTimeoutRef.current !== null) clearTimeout(copyTimeoutRef.current);
    },
    [],
  );

  const sourceKey = (component as FlowComponent & {source?: string}).source;
  const rawValue = sourceKey && additionalData ? additionalData[sourceKey] : undefined;
  const value =
    typeof rawValue === 'string' || typeof rawValue === 'number' || typeof rawValue === 'boolean'
      ? String(rawValue)
      : '';
  const label = component.label ? t(resolve(component.label) ?? component.label) : undefined;

  // Handle the copy action, attempting to use the Clipboard API and falling back to a textarea method if necessary.
  const handleCopy = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(value);
    } catch {
      const textArea = document.createElement('textarea');
      textArea.value = value;
      document.body.appendChild(textArea);
      textArea.select();
      document.execCommand('copy');
      document.body.removeChild(textArea);
    }
    setCopied(true);
    if (copyTimeoutRef.current !== null) clearTimeout(copyTimeoutRef.current);
    copyTimeoutRef.current = setTimeout(() => setCopied(false), 3000);
  }, [value]);

  const copyLabel = copied ? t('common:actions.copied', 'Copied!') : t('common:actions.copy', 'Copy');

  const copyButton = (
    <IconButton
      aria-label={copyLabel}
      onClick={() => {
        void handleCopy();
      }}
      edge="end"
      size="small"
    >
      {copied ? <Check size={16} /> : <Copy size={16} />}
    </IconButton>
  );

  if (component.masked) {
    return (
      <Box sx={{display: 'flex', flexDirection: 'column', gap: 0.5, width: '100%'}}>
        {label && (
          <Typography variant="body2" color="text.secondary" sx={{fontWeight: 500}}>
            {label}
          </Typography>
        )}
        <TextField
          fullWidth
          size="small"
          id={component.id ?? sourceKey}
          data-testid={component.id ?? sourceKey}
          type={isRevealed ? 'text' : 'password'}
          value={value}
          slotProps={{
            htmlInput: {'aria-label': label, autoComplete: 'off'},
            input: {
              readOnly: true,
              endAdornment: (
                <InputAdornment position="end">
                  <IconButton
                    aria-label={isRevealed ? t('common:actions.hide', 'Hide') : t('common:actions.show', 'Show')}
                    onClick={() => setIsRevealed((previous) => !previous)}
                    edge="end"
                    size="small"
                  >
                    {isRevealed ? <EyeOff size={16} /> : <Eye size={16} />}
                  </IconButton>
                  <Box component="span" sx={{ml: 0.5}}>
                    {copyButton}
                  </Box>
                </InputAdornment>
              ),
            },
          }}
        />
      </Box>
    );
  }

  return (
    <Box sx={{display: 'flex', flexDirection: 'column', gap: 0.5, width: '100%'}}>
      {label && (
        <Typography variant="body2" color="text.secondary" sx={{fontWeight: 500}}>
          {label}
        </Typography>
      )}
      <Box
        sx={{
          alignItems: 'center',
          backgroundColor: 'background.default',
          border: '1px solid',
          borderColor: 'divider',
          borderRadius: 1,
          display: 'flex',
          gap: 1,
          pl: 1.5,
          pr: 1,
          py: 0.75,
        }}
      >
        <Typography
          variant="body2"
          sx={{
            flex: 1,
            fontFamily: 'monospace',
            fontSize: '0.85rem',
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            whiteSpace: 'nowrap',
          }}
        >
          {value}
        </Typography>
        {copyButton}
      </Box>
    </Box>
  );
}
