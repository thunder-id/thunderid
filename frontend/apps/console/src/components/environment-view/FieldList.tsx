// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, IconButton, Stack, Tooltip, Typography} from '@wso2/oxygen-ui';
import {Check, Copy} from '@wso2/oxygen-ui-icons-react';
import {useEffect, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {fieldLabel} from './labels';
import ReadOnlyValue from './ReadOnlyValue';
import {isEmptyValue, parseReference} from './values';

/** One label and value row. */
export interface FieldRow {
  key: string;
  label: string;
  value: unknown;
}

/** Copies a value, saying so for a moment after. */
function CopyButton({value, label}: {value: string; label: string}): JSX.Element {
  const {t} = useTranslation();
  const [copied, setCopied] = useState(false);
  useEffect(() => {
    if (!copied) return undefined;
    const timer = window.setTimeout(() => setCopied(false), 2000);
    return () => window.clearTimeout(timer);
  }, [copied]);
  const title = copied
    ? t('common:actions.copied', 'Copied')
    : t('common:environment.value.copy', 'Copy {{label}}', {label});
  return (
    <Tooltip title={title}>
      <IconButton
        size="small"
        aria-label={title}
        onClick={() => {
          void navigator.clipboard.writeText(value).then(() => setCopied(true));
        }}
      >
        {copied ? <Check size={14} /> : <Copy size={14} />}
      </IconButton>
    </Tooltip>
  );
}

export interface FieldRowsProps {
  rows: FieldRow[];
  /** Whether the rows' plain values are offered to copy, as a card of identifiers offers them. */
  copy?: boolean;
  /** Set for rows inside another value, which are drawn lighter and closer. */
  nested?: boolean;
}

/**
 * Fields as a card shows them for reading: each a row with its label on the left and its value on
 * the right, the rows parted by a rule, so a card reads down like a list of settings.
 */
export function FieldRows({rows, copy = false, nested = false}: FieldRowsProps): JSX.Element {
  return (
    <Box sx={nested ? {pl: 2, borderLeft: 2, borderColor: 'divider'} : undefined}>
      {rows.map((row: FieldRow, index: number) => (
        <Box
          key={row.key}
          sx={{
            display: 'grid',
            gridTemplateColumns: {xs: '1fr', sm: nested ? 'minmax(120px, 160px) 1fr' : 'minmax(160px, 220px) 1fr'},
            columnGap: 3,
            rowGap: 0.5,
            alignItems: 'start',
            py: nested ? 1 : 1.5,
            borderBottom: index < rows.length - 1 ? 1 : 0,
            borderColor: 'divider',
          }}
        >
          <Typography variant="body2" color="text.secondary" sx={{pt: 0.25, wordBreak: 'break-word'}}>
            {row.label}
          </Typography>
          <Stack direction="row" alignItems="center" spacing={0.5} sx={{minWidth: 0}}>
            <Box sx={{minWidth: 0, flex: 1}}>
              <ReadOnlyValue value={row.value} />
            </Box>
            {copy && typeof row.value === 'string' && !parseReference(row.value) && (
              <CopyButton value={row.value} label={row.label} />
            )}
          </Stack>
        </Box>
      ))}
    </Box>
  );
}

export interface FieldListProps {
  fields: Record<string, unknown>;
  /** The fields shown, in this order; every field with something to say when absent. */
  order?: string[];
  /** Set for a list inside another value. */
  nested?: boolean;
}

/** A key is shown as it is when it is not a field's name but a key of the thing's own, such as a translation's. */
const isOwnKey = (key: string): boolean => /[.\s/]/.test(key);

/** A resource's fields as rows, leaving out the ones that hold nothing. */
export default function FieldList({fields, order = undefined, nested = false}: FieldListProps): JSX.Element {
  const keys = (order ?? Object.keys(fields)).filter((key: string) => !isEmptyValue(fields[key]));
  return (
    <FieldRows
      nested={nested}
      rows={keys.map((key: string) => ({key, label: isOwnKey(key) ? key : fieldLabel(key), value: fields[key]}))}
    />
  );
}
