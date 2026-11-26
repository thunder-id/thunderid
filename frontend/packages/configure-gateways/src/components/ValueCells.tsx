// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Button, Chip, Stack, Tooltip, Typography} from '@wso2/oxygen-ui';
import {Pencil} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import type {ValueRow} from './ValuesTable';
import useValueActionLabel from '../hooks/useValueActionLabel';
import type {ValueKind, ValueReference} from '../models/deploy';
import {humanize} from '../utils/groupByResource';
import {toListText} from '../utils/listValue';
import {fieldLabel, parseValueName} from '../utils/valueLabels';
import {isHeld, placesOf} from '../utils/valueRows';

/**
 * What a value is called: the field it stands in, with the other places that use it, and its name below;
 * or, for a name the server derived that nothing uses, the type and field read back from it. With
 * `withResource`, the resource that uses it is named above it, for a list not grouped by resource. A value
 * only a hidden resource uses shows nothing but that: its name could spell the hidden resource's type and ID.
 */
export function ValueName({row, withResource = false}: {row: ValueRow; withResource?: boolean}): JSX.Element {
  const {t} = useTranslation('deploy');
  if (row.hiddenUse) {
    return <Chip size="small" variant="outlined" label={t('values.hiddenUse', 'Used by a resource not shown here')} />;
  }
  const labelOf = (field: string | undefined): string => fieldLabel(t, field);
  const places = placesOf(row);
  // The running version does not use it: the current configuration does, or nothing does.
  const notRunning = row.references.length === 0;
  // A name the server derived reads back as a type and a field, which says more than the name does.
  const derived = places.length === 0 ? parseValueName(row.name) : null;
  let title = row.name;
  if (places.length > 0) {
    title = labelOf(places[0].field);
  } else if (derived) {
    title = `${t(`values.typeNames.${derived.resourceType}`, {defaultValue: humanize(derived.resourceType)})} · ${labelOf(derived.field)}`;
  }
  const resource = withResource && places.length > 0 ? places[0] : undefined;
  const resourceLabel = resource
    ? t('values.resourceLabel', '{{type}}: {{name}}', {
        type: t(`values.typeNames.${resource.resourceType}`, {defaultValue: humanize(resource.resourceType)}),
        name: resource.resourceName ?? resource.resourceId ?? '',
      })
    : undefined;
  const others = [
    ...new Set(
      places
        .slice(1)
        .map((reference: ValueReference) =>
          [reference.resourceName ?? reference.resourceId, labelOf(reference.field)].filter(Boolean).join(' · '),
        ),
    ),
  ];
  return (
    <Box>
      {resourceLabel && (
        <Typography variant="caption" color="text.secondary" component="div">
          {resourceLabel}
        </Typography>
      )}
      <Stack direction="row" spacing={1} alignItems="center">
        <Typography variant="body2" sx={{fontWeight: 500}}>
          {title}
        </Typography>
        {notRunning && places.length > 0 && (
          <Chip size="small" variant="outlined" label={t('values.notRunning', 'Not used by the running version')} />
        )}
        {others.length > 0 && (
          <Tooltip title={others.join(', ')}>
            <Typography component="span" variant="body2" color="text.secondary">
              {t('values.more', '+{{count}} more', {count: others.length})}
            </Typography>
          </Tooltip>
        )}
      </Stack>
      {(places.length > 0 || derived) && (
        <Typography variant="caption" color="text.secondary" sx={{fontFamily: 'monospace'}}>
          {row.name}
        </Typography>
      )}
    </Box>
  );
}

/**
 * The button that opens a row's value to be set. Its accessible name starts with the text it shows, so
 * a voice command that reads the button finds it. It is disabled while a removal is pending, since a
 * removal that lands after a save would delete what was just saved.
 */
export function SetValueButton({
  row,
  kind,
  onClick,
  disabled = false,
}: {
  row: ValueRow;
  kind: ValueKind;
  onClick: () => void;
  disabled?: boolean;
}): JSX.Element {
  const {text, label} = useValueActionLabel(row, kind);
  return (
    <Button
      size="small"
      variant="text"
      startIcon={<Pencil size={14} />}
      onClick={onClick}
      aria-label={label}
      disabled={disabled}
    >
      {text}
    </Button>
  );
}

/**
 * What a gateway holds for a row: a variable's value, that a secret is stored, or that nothing is set. A
 * value only a hidden resource uses shows nothing.
 */
export function ValueText({row, kind}: {row: ValueRow; kind: ValueKind}): JSX.Element | null {
  const {t} = useTranslation('deploy');
  const secret = kind === 'secret';
  if (row.hiddenUse) return null;
  if (!isHeld(row, secret)) {
    return <Chip size="small" color="error" variant="outlined" label={t('values.notSet', 'Not set')} />;
  }
  if (secret) return <Chip size="small" variant="outlined" label={t('values.stored', 'Stored')} />;
  return (
    <Typography
      variant="body2"
      sx={{fontFamily: 'monospace', fontSize: 13, whiteSpace: 'pre-wrap', wordBreak: 'break-all'}}
    >
      {toListText(row.stored?.value ?? '') || t('values.empty', '(empty)')}
    </Typography>
  );
}
