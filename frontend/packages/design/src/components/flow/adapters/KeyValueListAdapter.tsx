// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {cn} from '@thunderid/utils';
import {Box, Typography} from '@wso2/oxygen-ui';
import {Fragment, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import type {FlowComponent} from '../../../models/flow';

/**
 * One row of a KEY_VALUE_LIST: what to call the value, and the value.
 */
interface KeyValuePair {
  label?: string;
  value?: string;
}

/**
 * Props for the KeyValueListAdapter component.
 */
interface KeyValueListAdapterProps {
  additionalData?: Record<string, unknown>;
  component: FlowComponent;
  resolve: (template: string | undefined) => string | undefined;
}

/**
 * Reads the pairs the component's `source` key names in `additionalData`. An executor publishes
 * them as a JSON array, since additional data carries strings, so both an array and its encoding
 * are accepted. Anything else is not a list of pairs and yields none.
 *
 * @param raw - The raw value found under the source key.
 * @returns The pairs to render, empty when the value is not a list of them.
 */
function readPairs(raw: unknown): KeyValuePair[] {
  let parsed: unknown = raw;

  if (typeof raw === 'string') {
    try {
      parsed = JSON.parse(raw);
    } catch {
      return [];
    }
  }

  if (!Array.isArray(parsed)) {
    return [];
  }

  // A non-string value would reach React as a child and throw, so it is dropped as the SDK parsers do.
  return parsed.filter(
    (entry): entry is KeyValuePair =>
      typeof entry === 'object' &&
      entry !== null &&
      typeof (entry as KeyValuePair).value === 'string' &&
      ['string', 'undefined'].includes(typeof (entry as KeyValuePair).label),
  );
}

/**
 * Adapter component to render a list of key-value pairs, sourced at runtime from the
 * `additionalData` key named in the component's `source`. It is bound to a key rather than to a
 * fixed set of rows so that whatever publishes the pairs decides how many there are and what they
 * are called, and the flow author only names where to read them.
 *
 * Nothing renders when the source holds no pairs, since an empty panel tells the End-User nothing.
 *
 * @param props - The properties for the adapter, including the flow component configuration, the
 * resolve function for template strings, and the additional data holding the pairs.
 * @returns The rendered pairs, or null when there are none.
 */
export default function KeyValueListAdapter({
  component,
  resolve,
  additionalData = undefined,
}: KeyValueListAdapterProps): JSX.Element | null {
  const {t} = useTranslation();

  const sourceKey = component.source;
  const pairs = readPairs(sourceKey && additionalData ? additionalData[sourceKey] : undefined).filter(
    (pair) => (pair.value ?? '') !== '',
  );

  if (pairs.length === 0) {
    return null;
  }

  const label = component.label ? t(resolve(component.label) ?? component.label) : undefined;

  return (
    <Box
      id={component.id}
      className={[cn('Flow--key-value-list'), component.classes].filter(Boolean).join(' ')}
      sx={{display: 'flex', flexDirection: 'column', gap: 0.5, width: '100%'}}
    >
      {label && (
        <Typography variant="body2" color="text.secondary" sx={{fontWeight: 500}}>
          {label}
        </Typography>
      )}
      <Box
        sx={{
          backgroundColor: 'action.hover',
          border: '1px solid',
          borderColor: 'divider',
          borderRadius: 1,
          // Two columns rather than a row each, so every value starts at the same edge however
          // long the labels beside them are.
          columnGap: 3,
          display: 'grid',
          gridTemplateColumns: 'minmax(0, max-content) minmax(0, 1fr)',
          p: 2,
          rowGap: 1.5,
        }}
      >
        {pairs.map((pair, index) => (
          // Pairs are positional and carry no id, so the index is the only stable key available.
          // eslint-disable-next-line react/no-array-index-key
          <Fragment key={`${pair.label ?? ''}-${index}`}>
            <Typography variant="body2" color="text.secondary">
              {t(resolve(pair.label) ?? pair.label ?? '')}
            </Typography>
            <Typography variant="body2" sx={{fontWeight: 500, overflowWrap: 'anywhere'}}>
              {pair.value}
            </Typography>
          </Fragment>
        ))}
      </Box>
    </Box>
  );
}
