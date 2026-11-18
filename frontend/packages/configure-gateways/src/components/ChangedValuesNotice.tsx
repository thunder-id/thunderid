// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Chip, Stack, Typography} from '@wso2/oxygen-ui';
import {useContext, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import ResourceNamesContext from '../contexts/ResourceNames';
import type {ChangedValues, ValueKind, ValueReference} from '../models/deploy';
import {humanize} from '../utils/groupByResource';
import {fieldLabel, parseValueName} from '../utils/valueLabels';

export interface ChangedValuesNoticeProps {
  changed: ChangedValues;
}

/**
 * Names the values set on the gateway since the version it runs was deployed: they take effect with
 * this deploy though no version carries them, so they are shown before anything is written. Each is
 * called by the resource and field it stands in for, as the values page calls it, with its own name
 * beneath. Names and what refers to them only, never a value.
 */
export default function ChangedValuesNotice({changed}: ChangedValuesNoticeProps): JSX.Element | null {
  const {t} = useTranslation('deploy');
  const names = useContext(ResourceNamesContext);
  const references = changed.references ?? [];
  const entries: {kind: ValueKind; name: string}[] = [
    ...(changed.secrets ?? []).map((name: string) => ({kind: 'secret' as ValueKind, name})),
    ...(changed.variables ?? []).map((name: string) => ({kind: 'variable' as ValueKind, name})),
  ];
  if (entries.length === 0) return null;
  const referencesTo = (kind: ValueKind, name: string): ValueReference[] =>
    references.filter((reference: ValueReference) => reference.kind === kind && reference.name === name);
  // What a person calls the value: the resource that uses it and the field, from the version's
  // references, else read back from a name the server derived, the resource named as the gateway's map
  // names it or by its kind. A name someone chose is shown as it is.
  const labelOf = (kind: ValueKind, name: string): {label: string; resource?: string} => {
    const used = referencesTo(kind, name).find((reference: ValueReference) => Boolean(reference.resourceName));
    if (used?.resourceName) {
      return {label: `${used.resourceName} · ${fieldLabel(t, used.field)}`, resource: used.resourceName};
    }
    const derived = parseValueName(name);
    if (!derived) return {label: name};
    const resource = derived.resourceId ? names.get(`${derived.resourceType}/${derived.resourceId}`) : undefined;
    const owner =
      resource ?? t(`values.typeNames.${derived.resourceType}`, {defaultValue: humanize(derived.resourceType)});
    return {label: `${owner} · ${fieldLabel(t, derived.field)}`, resource};
  };
  // The other resources that use the value, beside the one it is called by.
  const usedBy = (kind: ValueKind, name: string, resource?: string): string =>
    referencesTo(kind, name)
      .map((reference: ValueReference) => reference.resourceName ?? reference.resourceId ?? reference.resourceType)
      .filter((value: string, index: number, all: string[]) => value !== resource && all.indexOf(value) === index)
      .join(', ');
  // Laid out as the drawer's change sections are, with nothing to select: every value set since the
  // last deploy takes effect with this one.
  const title = t('changed.title', 'Values set since the last deploy');
  return (
    <Stack spacing={1.25} component="section" aria-label={title} data-testid="changed-values">
      <Typography variant="subtitle2">
        {t('deployChanges.title', '{{title}} · {{count}}', {title, count: entries.length})}
      </Typography>
      <Typography
        variant="body2"
        color="text.secondary"
        sx={{px: 1.5, py: 1, borderRadius: 1.5, bgcolor: 'action.hover'}}
      >
        {t('changed.hint', 'These values take effect with this deploy, though no version carries them.')}
      </Typography>
      {entries.map((entry) => {
        const {label, resource} = labelOf(entry.kind, entry.name);
        const others = usedBy(entry.kind, entry.name, resource);
        return (
          <Box
            key={`${entry.kind}:${entry.name}`}
            data-testid={`changed-value-${entry.name}`}
            sx={{border: 1, borderColor: 'divider', borderRadius: 2, px: 1.5, py: 1}}
          >
            <Stack direction="row" alignItems="center" spacing={1.5}>
              <Chip
                size="small"
                variant="outlined"
                label={entry.kind === 'secret' ? t('changed.secret', 'Secret') : t('changed.variable', 'Variable')}
                sx={{height: 20, fontSize: 11, minWidth: 72, flexShrink: 0}}
              />
              <Box sx={{flex: 1, minWidth: 0}}>
                <Typography variant="body2" fontWeight={500} noWrap title={label}>
                  {label}
                </Typography>
                {label !== entry.name && (
                  <Typography
                    variant="caption"
                    color="text.secondary"
                    component="div"
                    noWrap
                    title={entry.name}
                    data-testid="changed-value-name"
                    sx={{fontFamily: 'monospace'}}
                  >
                    {entry.name}
                  </Typography>
                )}
                {others && (
                  <Typography variant="caption" color="text.secondary" component="div" noWrap>
                    {t('changed.usedBy', 'used by {{names}}', {names: others})}
                  </Typography>
                )}
              </Box>
            </Stack>
          </Box>
        );
      })}
    </Stack>
  );
}
