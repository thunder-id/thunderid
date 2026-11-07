// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, ButtonBase, Chip, Collapse, Stack, TextField, Typography} from '@wso2/oxygen-ui';
import {ChevronDown, ChevronUp, CircleAlert, CircleCheck} from '@wso2/oxygen-ui-icons-react';
import {useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import type {ValueReference} from '../models/deploy';
import {humanize} from '../utils/groupByResource';
import {fieldKey, type MissingField} from '../utils/missingValues';
import {fieldLabel} from '../utils/valueLabels';

export interface MissingValuesFieldsProps {
  fields: MissingField[];
  values: Record<string, string>;
  onChange: (field: MissingField, value: string) => void;
  disabled?: boolean;
  /**
   * The gateway the values are set on. Given, the panel is titled for it and opens and closes, as
   * the deploy drawer shows it.
   */
  target?: string;
}

/** The values one resource uses, or the values nothing names a resource for under a null resource. */
interface FieldGroup {
  key: string;
  resourceType?: string;
  resourceName?: string;
  fields: MissingField[];
}

/** Groups the fields by the resource that first refers to each, in the order they come. */
function byResource(fields: MissingField[]): FieldGroup[] {
  const groups = new Map<string, FieldGroup>();
  fields.forEach((field: MissingField) => {
    const reference = field.references[0];
    const key = reference ? `${reference.resourceType}/${reference.resourceId ?? reference.resourceName ?? ''}` : '';
    const group = groups.get(key) ?? {
      key,
      resourceType: reference?.resourceType,
      resourceName: reference?.resourceName ?? reference?.resourceId,
      fields: [],
    };
    group.fields.push(field);
    groups.set(key, group);
  });
  return [...groups.values()];
}

const WARNING = 'var(--oxygen-palette-warning-mainChannel)';

/**
 * An input for each value the target lacks: text for a variable, a password field for a secret, grouped
 * by the resource that uses it and named for a person, with the value's own name beside. The values sit
 * together in a gently marked panel that counts how many are left, and each empty field is outlined
 * until it is filled.
 */
export default function MissingValuesFields({
  fields,
  values,
  onChange,
  disabled = false,
  target = undefined,
}: MissingValuesFieldsProps): JSX.Element {
  const {t} = useTranslation('deploy');
  const [open, setOpen] = useState(true);
  const filled = fields.filter((field: MissingField) => (values[fieldKey(field)] ?? '').trim() !== '').length;
  const done = filled === fields.length;
  let title = done ? t('missing.allSet', 'All values set') : t('missing.title', 'Values this gateway needs');
  if (target)
    title = t('missing.titleFor', 'Values {{target}} needs · {{count}} to set', {
      target,
      count: fields.length - filled,
    });
  const fieldInput = (field: MissingField): JSX.Element => {
    const id = `missing-${field.kind}-${field.name}`;
    // Another resource that uses the same value, beside the one it is listed under.
    const others = field.references
      .slice(1)
      .map((reference: ValueReference) => reference.resourceName ?? reference.resourceId)
      .filter(Boolean)
      .join(', ');
    const empty = (values[fieldKey(field)] ?? '').trim() === '';
    return (
      <Box key={fieldKey(field)} data-empty={empty}>
        <Typography
          component="label"
          htmlFor={id}
          variant="body2"
          sx={{display: 'flex', alignItems: 'center', gap: 0.75, mb: 0.5, flexWrap: 'wrap'}}
        >
          <Box
            component="span"
            aria-hidden
            sx={{display: 'inline-flex', color: empty ? 'warning.main' : 'success.main'}}
          >
            {empty ? <CircleAlert size={12} /> : <CircleCheck size={12} />}
          </Box>
          <Box component="span" sx={{fontWeight: 500}}>
            {field.references[0]?.field ? fieldLabel(t, field.references[0].field) : field.name}
          </Box>
          {field.references[0]?.field && (
            <Typography component="span" variant="caption" color="text.secondary" sx={{fontFamily: 'monospace'}}>
              {field.name}
            </Typography>
          )}
          {others && (
            <Typography component="span" variant="caption" color="text.secondary">
              {t('missing.alsoUsedBy', 'also used by {{names}}', {names: others})}
            </Typography>
          )}
          {empty && (
            <Typography component="span" variant="caption" sx={{ml: 'auto', color: 'warning.main'}}>
              {t('missing.needed', 'Needed')}
            </Typography>
          )}
        </Typography>
        <TextField
          id={id}
          size="small"
          fullWidth
          type={field.kind === 'secret' ? 'password' : 'text'}
          multiline={field.list}
          minRows={field.list ? 2 : undefined}
          autoComplete={field.kind === 'secret' ? 'new-password' : 'off'}
          value={values[fieldKey(field)] ?? ''}
          onChange={(event) => onChange(field, event.target.value)}
          disabled={disabled}
          placeholder={
            field.kind === 'secret'
              ? t('missing.secretPlaceholder', 'Secret value')
              : t('missing.variablePlaceholder', 'Variable value')
          }
          helperText={field.list ? t('missing.listHint', 'One per line.') : undefined}
          sx={{
            '& .MuiOutlinedInput-root': {bgcolor: 'background.paper'},
            ...(empty
              ? {
                  '& .MuiOutlinedInput-root:not(.Mui-focused) .MuiOutlinedInput-notchedOutline': {
                    borderColor: `rgba(${WARNING} / 0.6)`,
                  },
                }
              : {}),
          }}
        />
      </Box>
    );
  };

  return (
    <Stack
      spacing={1.5}
      data-testid="missing-values"
      sx={{
        p: 2,
        borderRadius: 2,
        border: 1,
        borderColor: done ? 'divider' : `rgba(${WARNING} / 0.45)`,
        borderLeft: 4,
        borderLeftColor: done ? 'success.main' : 'warning.main',
        bgcolor: done ? 'transparent' : `rgba(${WARNING} / 0.06)`,
        transition: 'background-color 0.2s, border-color 0.2s',
      }}
    >
      <Stack
        direction="row"
        alignItems="center"
        spacing={1}
        component={target ? ButtonBase : 'div'}
        onClick={target ? () => setOpen((previous: boolean) => !previous) : undefined}
        aria-expanded={target ? open : undefined}
        sx={{width: '100%', textAlign: 'left', justifyContent: 'flex-start'}}
      >
        <Box sx={{display: 'flex', color: done ? 'success.main' : 'warning.main'}}>
          {done ? <CircleCheck size={18} /> : <CircleAlert size={18} />}
        </Box>
        <Typography variant="subtitle2" sx={{flex: 1}}>
          {title}
        </Typography>
        <Chip
          size="small"
          variant="outlined"
          color={done ? 'success' : 'warning'}
          label={t('missing.progress', '{{filled}} of {{total}} set', {filled, total: fields.length})}
          sx={{height: 20, fontSize: 11}}
        />
        {target && (open ? <ChevronUp size={16} /> : <ChevronDown size={16} />)}
      </Stack>
      <Collapse in={!target || open}>
        <Stack spacing={2}>
          {byResource(fields).map((group: FieldGroup) => (
            <Stack key={group.key} spacing={1.25} data-testid={`missing-group-${group.key}`}>
              {group.resourceType && (
                <Typography variant="caption" color="text.secondary" sx={{fontWeight: 600}}>
                  {[
                    group.resourceName,
                    t(`changes.types.${group.resourceType}`, {defaultValue: humanize(group.resourceType)}),
                  ]
                    .filter(Boolean)
                    .join(' · ')}
                </Typography>
              )}
              {group.fields.map(fieldInput)}
            </Stack>
          ))}
        </Stack>
      </Collapse>
    </Stack>
  );
}
