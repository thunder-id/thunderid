// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Alert, Box, Button, Card, Chip, Stack, TextField, Typography} from '@wso2/oxygen-ui';
import {useMemo, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import LaneIcon from './LaneIcon';
import {useSetStoredValue} from '../api/useStoredValues';
import {LANES, laneOfType, type Lane} from '../explorer/model';
import type {StoredValue, ValueReference} from '../models/deploy';
import {humanize} from '../utils/groupByResource';
import {fromListText, toListText} from '../utils/listValue';
import {fieldLabel, valuesByResource, type ResourceValue, type ResourceValues} from '../utils/valueLabels';

export interface ResourceValuesViewProps {
  gatewayId: string;
  /** The gateway's name, to say whose values these are. */
  name: string;
  /** The places the version the gateway runs refers to its values. */
  references: ValueReference[];
  variables: StoredValue[];
  secrets: StoredValue[];
}

/** The resources of one kind, as the explorer groups them, or the kinds it has no lane for. */
interface Section {
  key: string;
  lane?: Lane;
  title: string;
  resources: ResourceValues[];
}

const keyOf = (value: ResourceValue): string => `${value.kind}:${value.name}`;

/**
 * The values a gateway holds, resource by resource: each resource a card with the values it uses,
 * named for a person, each set or not and editable in place. A secret's value is never shown.
 */
export default function ResourceValuesView({
  gatewayId,
  name,
  references,
  variables,
  secrets,
}: ResourceValuesViewProps): JSX.Element {
  const {t} = useTranslation('deploy');
  const setVariable = useSetStoredValue(gatewayId, 'variable');
  const setSecret = useSetStoredValue(gatewayId, 'secret');
  const [editing, setEditing] = useState<string | null>(null);
  const [draft, setDraft] = useState('');
  const [error, setError] = useState<string | null>(null);

  const sections: Section[] = useMemo(() => {
    const resources = valuesByResource(references);
    const byLane = LANES.map((lane: Lane) => ({
      key: lane.id,
      lane,
      title: t(lane.title[0], lane.title[1]),
      resources: resources.filter((resource: ResourceValues) => laneOfType(resource.resourceType)?.id === lane.id),
    }));
    const other = resources.filter((resource: ResourceValues) => !laneOfType(resource.resourceType));
    const otherTypes = [...new Set(other.map((resource: ResourceValues) => resource.resourceType))];
    return [
      ...byLane,
      ...otherTypes.map((type: string) => ({
        key: type,
        title: humanize(type),
        resources: other.filter((resource: ResourceValues) => resource.resourceType === type),
      })),
    ].filter((section: Section) => section.resources.length > 0);
  }, [references, t]);

  const stored = (value: ResourceValue): StoredValue | undefined =>
    (value.kind === 'secret' ? secrets : variables).find((entry: StoredValue) => entry.name === value.name);

  const start = (value: ResourceValue): void => {
    setError(null);
    setEditing(keyOf(value));
    setDraft(value.kind === 'secret' ? '' : toListText(stored(value)?.value ?? ''));
  };
  const save = async (value: ResourceValue): Promise<void> => {
    setError(null);
    const mutation = value.kind === 'secret' ? setSecret : setVariable;
    try {
      await mutation.mutateAsync({
        name: value.name,
        value: value.list ? fromListText(draft) : draft,
        description: stored(value)?.description,
      });
      setEditing(null);
    } catch (caught: unknown) {
      setError(
        caught instanceof Error && caught.message
          ? caught.message
          : t('values.saveFailed', 'The value could not be saved.'),
      );
    }
  };

  if (sections.length === 0) {
    return (
      <Typography variant="body2" color="text.secondary" sx={{py: 2}}>
        {t('resourceValues.none', 'Nothing {{name}} runs uses a variable or a secret.', {name})}
      </Typography>
    );
  }

  const row = (value: ResourceValue): JSX.Element => {
    const key = keyOf(value);
    const held = stored(value);
    const isSet = value.kind === 'secret' ? Boolean(held) : held !== undefined;
    const isEditing = editing === key;
    const saving = (value.kind === 'secret' ? setSecret : setVariable).isPending;
    return (
      <Box key={key} data-testid={`resource-value-${value.name}`} sx={{py: 1.25, borderTop: 1, borderColor: 'divider'}}>
        <Stack direction="row" spacing={1.5} alignItems="center" sx={{flexWrap: 'wrap', rowGap: 1}}>
          <Box sx={{flex: 1, minWidth: 180}}>
            <Typography variant="body2" fontWeight={500}>
              {fieldLabel(t, value.field)}
            </Typography>
            <Typography variant="caption" color="text.secondary" sx={{fontFamily: 'monospace'}}>
              {value.name}
            </Typography>
          </Box>
          {!isEditing &&
            (isSet ? (
              <Chip
                size="small"
                variant="outlined"
                color="success"
                label={value.kind === 'secret' ? t('values.stored', 'Stored') : t('resourceValues.set', 'Set')}
              />
            ) : (
              <Chip size="small" variant="outlined" color="warning" label={t('values.notSet', 'Not set')} />
            ))}
          {!isEditing && (
            <Button
              size="small"
              variant="text"
              onClick={() => start(value)}
              data-testid={`resource-value-edit-${value.name}`}
            >
              {isSet ? t('resourceValues.edit', 'Edit') : t('resourceValues.setValue', 'Set')}
            </Button>
          )}
        </Stack>
        {isEditing && (
          <Stack spacing={1} sx={{mt: 1}}>
            <TextField
              size="small"
              fullWidth
              type={value.kind === 'secret' ? 'password' : 'text'}
              multiline={value.list}
              minRows={value.list ? 2 : undefined}
              autoComplete={value.kind === 'secret' ? 'new-password' : 'off'}
              value={draft}
              onChange={(event) => setDraft(event.target.value)}
              placeholder={
                value.kind === 'secret'
                  ? t('missing.secretPlaceholder', 'Secret value')
                  : t('missing.variablePlaceholder', 'Variable value')
              }
              helperText={value.list ? t('missing.listHint', 'One per line.') : undefined}
              inputProps={{'aria-label': fieldLabel(t, value.field)}}
            />
            <Stack direction="row" spacing={1} justifyContent="flex-end">
              <Button size="small" variant="text" color="inherit" onClick={() => setEditing(null)} disabled={saving}>
                {t('applyPanel.cancel', 'Cancel')}
              </Button>
              <Button
                size="small"
                variant="contained"
                onClick={() => void save(value)}
                disabled={saving || draft.trim() === ''}
                data-testid={`resource-value-save-${value.name}`}
              >
                {t('resourceValues.save', 'Save')}
              </Button>
            </Stack>
          </Stack>
        )}
      </Box>
    );
  };

  return (
    <Stack spacing={3} data-testid="resource-values">
      {error && <Alert severity="error">{error}</Alert>}
      {sections.map((section: Section) => (
        <Box key={section.key} component="section" aria-label={section.title}>
          <Stack direction="row" spacing={1} alignItems="center" sx={{mb: 1.5}}>
            {section.lane && (
              <Box sx={{width: 8, height: 8, borderRadius: 0.5, bgcolor: section.lane.color, flexShrink: 0}} />
            )}
            <Typography variant="subtitle2">{section.title}</Typography>
          </Stack>
          <Box sx={{display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(320px, 1fr))', gap: 2}}>
            {section.resources.map((resource: ResourceValues) => {
              const missing = resource.values.filter((value: ResourceValue) => {
                const held = stored(value);
                return value.kind === 'secret' ? !held : held === undefined;
              }).length;
              return (
                <Card key={resource.key} variant="outlined" sx={{p: 2}} data-testid={`resource-card-${resource.key}`}>
                  <Stack direction="row" spacing={1.5} alignItems="center" sx={{mb: 1}}>
                    <Box
                      sx={{
                        width: 36,
                        height: 36,
                        borderRadius: 1.5,
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'center',
                        border: 1,
                        borderColor: section.lane?.color ?? 'divider',
                        color: section.lane?.color ?? 'text.secondary',
                        flexShrink: 0,
                      }}
                    >
                      {section.lane && <LaneIcon name={section.lane.icon} size={18} />}
                    </Box>
                    <Box sx={{flex: 1, minWidth: 0}}>
                      <Typography variant="body1" fontWeight={600} noWrap>
                        {resource.resourceName ?? resource.resourceId ?? resource.key}
                      </Typography>
                      <Typography variant="caption" color={missing > 0 ? 'warning.main' : 'text.secondary'}>
                        {missing > 0
                          ? t('resourceValues.missing', '{{count}} to set', {count: missing})
                          : t('resourceValues.count', '{{count}} values', {count: resource.values.length})}
                      </Typography>
                    </Box>
                  </Stack>
                  {resource.values.map(row)}
                </Card>
              );
            })}
          </Box>
        </Box>
      ))}
    </Stack>
  );
}
