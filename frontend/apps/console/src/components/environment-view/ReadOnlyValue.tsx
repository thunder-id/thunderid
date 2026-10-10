// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Chip, IconButton, Link, Stack, Tooltip, Typography} from '@wso2/oxygen-ui';
import {KeyRound, Lock, Pencil, Variable} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {Link as RouterLink} from 'react-router';
import {useEnvironmentView} from './EnvironmentViewContext';
import FieldList from './FieldList';
import ObjectTable from './ObjectTable';
import {isObject, isSchema} from './paths';
import SchemaTable from './SchemaTable';
import {MASKED_SECRET, parseListValue, parseReference, type ValueReference} from './values';

/** A value short and plain enough to read as a tag, such as a grant type or a scope. */
const TAG = /^[A-Za-z0-9_:.+-]{1,40}$/;
/** A colour as a theme writes one. */
const COLOR = /^#(?:[0-9a-f]{3,4}|[0-9a-f]{6}|[0-9a-f]{8})$/i;
/** What a read shows in place of a secret it holds. */
const MASKED = /^\*{3,}$/;

export function Text({children, mono = false}: {children: string; mono?: boolean}): JSX.Element {
  const long = children.length > 120 || children.includes('\n');
  return (
    <Typography
      variant="body2"
      component={long ? 'pre' : 'span'}
      sx={{
        fontFamily: mono || long ? 'monospace' : undefined,
        whiteSpace: long ? 'pre-wrap' : undefined,
        wordBreak: 'break-word',
        m: 0,
      }}
    >
      {children}
    </Typography>
  );
}

/** A secret as a read shows it: held, but never shown. */
function Masked(): JSX.Element {
  const {t} = useTranslation();
  return (
    <Chip
      size="small"
      variant="outlined"
      icon={<Lock size={12} />}
      label={t('common:environment.value.masked', 'Set, not shown')}
    />
  );
}

/**
 * A value the gateway fills from its own store: what the gateway holds, or that it holds nothing,
 * with the name it is held under and the action that sets it. This is the one thing an environment
 * view changes.
 */
function ReferencedValue({reference, list}: {reference: ValueReference; list: boolean}): JSX.Element {
  const {t} = useTranslation();
  const {variables, secrets, edit} = useEnvironmentView();
  const secret = reference.kind === 'secret';
  const held: string | undefined = secret
    ? secrets.has(reference.name)
      ? MASKED_SECRET
      : undefined
    : variables.get(reference.name);
  const items: string[] | undefined = !secret && held !== undefined && list ? parseListValue(held) : undefined;
  const label = secret
    ? t('common:environment.value.setSecret', 'Set secret {{name}}', {name: reference.name})
    : t('common:environment.value.setVariable', 'Set variable {{name}}', {name: reference.name});

  let shown: JSX.Element;
  if (held === undefined) {
    shown = (
      <Chip size="small" color="error" variant="outlined" label={t('common:environment.value.notSet', 'Not set')} />
    );
  } else if (secret) {
    shown = <Masked />;
  } else if (items) {
    shown = <Items items={items} />;
  } else {
    shown = <Text mono={held.includes('://')}>{held}</Text>;
  }
  return (
    <Stack direction="row" spacing={1} alignItems="center" flexWrap="wrap" useFlexGap>
      {shown}
      <Tooltip
        title={
          secret
            ? t('common:environment.value.secretHint', 'Held by this environment as the secret {{name}}', {
                name: reference.name,
              })
            : t('common:environment.value.variableHint', 'Held by this environment as the variable {{name}}', {
                name: reference.name,
              })
        }
      >
        <Chip
          size="small"
          variant="outlined"
          icon={secret ? <KeyRound size={12} /> : <Variable size={12} />}
          label={reference.name}
          sx={{fontFamily: 'monospace'}}
        />
      </Tooltip>
      <Tooltip title={label}>
        <IconButton size="small" aria-label={label} onClick={() => edit(reference, list)}>
          <Pencil size={14} />
        </IconButton>
      </Tooltip>
    </Stack>
  );
}

/** A string, linked to the resource it names when it is an identifier of one. */
function StringValue({value}: {value: string}): JSX.Element {
  const {resources} = useEnvironmentView();
  const linked = resources.get(value);
  if (linked) {
    return (
      <Link component={RouterLink} to={linked.path} variant="body2">
        {linked.name}
      </Link>
    );
  }
  if (MASKED.test(value)) {
    return <Masked />;
  }
  if (COLOR.test(value)) {
    return (
      <Stack direction="row" spacing={1} alignItems="center">
        <Box sx={{width: 16, height: 16, borderRadius: 0.5, bgcolor: value, border: 1, borderColor: 'divider'}} />
        <Text mono>{value}</Text>
      </Stack>
    );
  }
  return <Text mono={value.includes('://') || /^[a-f0-9-]{32,}$/i.test(value)}>{value}</Text>;
}

/**
 * The items of a list: as tags when every one is short and plain, as the configuration's pages show
 * grant types and scopes, and otherwise one under another.
 */
export function Items({items}: {items: unknown[]}): JSX.Element {
  const {resources} = useEnvironmentView();
  // A reference is never a tag: it stands for what the gateway holds, which is shown in its place.
  const tags = items.every(
    (item: unknown) => typeof item === 'string' && TAG.test(item) && !parseReference(item) && !resources.has(item),
  );
  if (tags) {
    return (
      <Stack direction="row" spacing={0.5} flexWrap="wrap" useFlexGap>
        {items.map((item: unknown) => (
          <Chip key={String(item)} size="small" variant="outlined" label={String(item)} />
        ))}
      </Stack>
    );
  }
  return (
    <Stack spacing={0.5}>
      {items.map((item: unknown, index: number) => (
        // A list's items have no identity but their place.
        // eslint-disable-next-line react/no-array-index-key
        <ReadOnlyValue key={index} value={item} listItem />
      ))}
    </Stack>
  );
}

/**
 * Shows one value of a resource as it reads: text, a setting that is on or off, tags, items one
 * under another, a list of fields, a schema, or a table of like entries. A value the gateway fills
 * shows what it holds.
 */
export default function ReadOnlyValue({value, listItem = false}: {value: unknown; listItem?: boolean}): JSX.Element {
  const {t} = useTranslation();
  const reference = parseReference(value);
  if (reference) {
    return <ReferencedValue reference={reference} list={listItem} />;
  }
  if (typeof value === 'boolean') {
    return (
      <Chip
        size="small"
        variant="outlined"
        color={value ? 'success' : 'default'}
        label={
          value ? t('common:environment.value.enabled', 'Enabled') : t('common:environment.value.disabled', 'Disabled')
        }
      />
    );
  }
  if (typeof value === 'string') {
    return <StringValue value={value} />;
  }
  if (typeof value === 'number' || typeof value === 'bigint') {
    return <Text>{String(value)}</Text>;
  }
  if (Array.isArray(value)) {
    if (value.length > 0 && value.every(isObject)) {
      return <ObjectTable rows={value} />;
    }
    return <Items items={value as unknown[]} />;
  }
  if (isObject(value)) {
    if (isSchema(value)) {
      return <SchemaTable schema={value} />;
    }
    return <FieldList fields={value} nested />;
  }
  return (
    <Typography variant="body2" color="text.disabled">
      -
    </Typography>
  );
}
