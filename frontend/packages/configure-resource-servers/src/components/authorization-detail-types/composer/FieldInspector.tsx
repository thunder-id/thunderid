// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  Alert,
  Autocomplete,
  Box,
  Button,
  Checkbox,
  FormControl,
  FormControlLabel,
  FormHelperText,
  FormLabel,
  IconButton,
  MenuItem,
  Radio,
  RadioGroup,
  Select,
  Stack,
  TextField,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import {Info, X} from '@wso2/oxygen-ui-icons-react';
import {useEffect, useRef, useState, type JSX, type KeyboardEvent, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import ComposerPanel from './ComposerPanel';
import KindIcon from './KindIcon';
import PanelSection from './PanelSection';
import useComposerLabels, {READING_ORDER} from './useComposerLabels';
import type {FieldKind, ItemKind, Modification, SchemaField} from '../../../models/authorization-detail-type';
import {
  effectiveModification,
  modificationsFor,
  supportsLimits,
  type FieldIssue,
} from '../../../utils/authorization-detail-schema';
import {
  changeKind,
  droppedSettings,
  setSuppliedBy,
  toggleParty,
  type DroppedSetting,
} from '../../../utils/schema-composer';

const KINDS: FieldKind[] = ['string', 'number', 'boolean', 'date', 'dateTime', 'array', 'object'];
const ITEM_KINDS: ItemKind[] = ['string', 'number'];

// A date and time limit is written as RFC 3339, which has no native input.
const DATE_TIME_HINT = '2026-12-31T23:59:59Z';

// The issues each section owns, by the end of their message key, so a section with one starts open.
const LIMIT_ISSUES = [
  'limitInvalid',
  'limitOrder',
  'maxLengthInvalid',
  'maxLengthTooLarge',
  'minItemsInvalid',
  'minItemsTooLarge',
];
const ALLOWED_VALUE_ISSUES = ['allowedValuesInvalid', 'allowedValuesDuplicate', 'allowedValuesSpaces'];
const POLICY_ISSUES = [
  'modifiableByRequired',
  'modifiableInSupplied',
  'processorMissing',
  'typeSupplied',
  'dateTimeUser',
  'policyRefused',
];

export interface FieldInspectorProps {
  field: SchemaField;
  /** The dotted path of the field. */
  path: string;
  /** Whether the processor supplies an object the field belongs to. */
  inheritsProcessor: boolean;
  issues: FieldIssue[];
  readOnly: boolean;
  /**
   * Brings the settings into view and moves focus into them each time it changes, as when the field is
   * added or selected on the card: to the name, or to the heading of a read-only field.
   */
  focusRequest?: object | null;
  /** A notice about the field shown above its settings, such as a summary that names its old path. */
  notice?: ReactNode;
  onChange: (field: SchemaField) => void;
  onClose: () => void;
}

/**
 * Edits the selected field in collapsible sections: its basics, its limits, the values a list allows,
 * and who can change it, which sets who supplies the value, how it may change after the request, and
 * who may change it. A collapsed section sums up its settings in its header.
 */
export default function FieldInspector({
  field,
  path,
  inheritsProcessor,
  issues,
  readOnly,
  focusRequest = null,
  notice = undefined,
  onChange,
  onClose,
}: FieldInspectorProps): JSX.Element {
  const {t, i18n} = useTranslation();
  const labels = useComposerLabels();
  const nameRef = useRef<HTMLInputElement>(null);
  const headingRef = useRef<HTMLHeadingElement>(null);
  const rootRef = useRef<HTMLDivElement>(null);
  // The field as it was before its kind changed, with the settings the change removed, until the
  // change is undone or the notice is dismissed.
  const [kindChange, setKindChange] = useState<{before: SchemaField; dropped: DroppedSetting[]} | null>(null);
  // An allowed value being typed, and why it was not added.
  const [allowedDraft, setAllowedDraft] = useState('');
  const [allowedError, setAllowedError] = useState<string | null>(null);

  const droppedLabels: Record<DroppedSetting, string> = {
    fields: t('authorizationDetailTypes:composer.kindChange.fields', 'the fields inside it'),
    maxLength: t('authorizationDetailTypes:composer.kindChange.maxLength', 'the maximum length'),
    allowedValues: t('authorizationDetailTypes:composer.kindChange.allowedValues', 'the allowed values'),
    minItems: t('authorizationDetailTypes:composer.kindChange.minItems', 'the minimum number of items'),
    limits: t('authorizationDetailTypes:composer.kindChange.limits', 'the minimum and maximum'),
    change: t('authorizationDetailTypes:composer.kindChange.change', 'the rule for changing it'),
  };

  const changeFieldKind = (kind: FieldKind): void => {
    const next = changeKind(field, kind);
    const dropped = droppedSettings(field, next);
    onChange(next);
    setKindChange(dropped.length > 0 ? {before: kindChange?.before ?? field, dropped} : null);
  };

  // The settings sit below the card on narrow screens, so they are scrolled to before taking focus.
  useEffect(() => {
    if (!focusRequest) return;
    rootRef.current?.scrollIntoView({block: 'nearest'});
    (readOnly ? headingRef.current : nameRef.current)?.focus({preventScroll: true});
  }, [focusRequest, readOnly]);

  const label = (text: string, tooltip: string): ReactNode => (
    <Tooltip title={tooltip} placement="top" arrow describeChild>
      <Stack direction="row" alignItems="center" spacing={0.5} component="span">
        <span>{text}</span>
        <Info size={14} color="inherit" />
      </Stack>
    </Tooltip>
  );

  const update = (change: Partial<SchemaField>): void => onChange({...field, ...change});
  const updatePolicy = (change: Partial<SchemaField['policy']>): void =>
    onChange({...field, policy: {...field.policy, ...change}});

  const hasIssue = (...keys: string[]): boolean =>
    issues.some((issue) => keys.some((key) => issue.messageKey.endsWith(key)));
  const limitError = hasIssue('limitInvalid', 'limitOrder');
  const supplied = field.policy.suppliedBy === 'processor';
  const modification = effectiveModification(field);
  // Nothing inside an object the processor supplies can change, so the question is asked only to take
  // back a rule set before.
  const fixed = supplied || (inheritsProcessor && modification === 'none');
  const modifications = modificationsFor(field);
  const heading = labels.heading(field);
  const flat = {field, processorSupplied: supplied || inheritsProcessor};
  const hasLimits = supportsLimits(field) || field.kind === 'string' || field.kind === 'array';
  // The consent screen cannot change a date and time, so the user is offered only to take back a choice
  // made before.
  const parties = READING_ORDER.filter(
    (party) => party !== 'user' || field.kind !== 'dateTime' || field.policy.modifiableBy?.includes('user'),
  );

  // An answer of a question, with a line under it that explains it.
  const option = (control: JSX.Element, text: string, hint: string, id: string): JSX.Element => (
    <Box key={id}>
      <FormControlLabel
        control={control}
        label={text}
        slotProps={{typography: {variant: 'body2', sx: {fontWeight: 500}}}}
        sx={{mr: 0}}
      />
      <Typography id={id} variant="caption" color="text.secondary" component="div" sx={{pl: 3.75, mt: -0.75}}>
        {hint}
      </Typography>
    </Box>
  );

  const limitsSummary = (): string => {
    const value = (limit?: number | string): string => (limit === undefined ? '' : String(limit).trim());
    if (field.kind === 'array') {
      const count = value(field.minItems);
      return count
        ? t('authorizationDetailTypes:composer.limits.minItems', 'At least {{count}} items', {
            count: Number(count),
            defaultValue_one: 'At least {{count}} item',
          })
        : t('authorizationDetailTypes:composer.limits.none', 'No limits');
    }
    if (field.kind === 'string') {
      const length = value(field.maxLength);
      return length
        ? t('authorizationDetailTypes:composer.limits.length', 'Up to {{count}} characters', {
            count: Number(length),
            defaultValue_one: 'Up to {{count}} character',
          })
        : t('authorizationDetailTypes:composer.limits.none', 'No limits');
    }
    const min = value(field.policy.minimum);
    const max = value(field.policy.maximum);
    const options = {min, max, interpolation: {escapeValue: false}};
    if (min && max) return t('authorizationDetailTypes:composer.limits.range', '{{min}} to {{max}}', options);
    if (max) return t('authorizationDetailTypes:composer.limits.max', 'Up to {{max}}', options);
    if (min) return t('authorizationDetailTypes:composer.limits.min', 'From {{min}}', options);
    return t('authorizationDetailTypes:composer.limits.none', 'No limits');
  };

  const limitInput = (limit: 'minimum' | 'maximum'): JSX.Element => (
    <TextField
      labelPlacement="top"
      label={
        limit === 'minimum'
          ? t('authorizationDetailTypes:fields.minimum', 'Minimum')
          : t('authorizationDetailTypes:fields.maximum', 'Maximum')
      }
      fullWidth
      id={`${field.key}-${limit}`}
      size="small"
      error={limitError}
      value={field.policy[limit] ?? ''}
      disabled={readOnly}
      type={field.kind === 'date' ? 'date' : 'text'}
      placeholder={
        limit === 'minimum'
          ? t('authorizationDetailTypes:fields.noMinimum', 'No minimum')
          : t('authorizationDetailTypes:fields.noMaximum', 'No maximum')
      }
      helperText={
        field.kind === 'dateTime'
          ? t('authorizationDetailTypes:fields.dateTimeHint', 'e.g., {{example}}', {example: DATE_TIME_HINT})
          : undefined
      }
      slotProps={{htmlInput: {inputMode: field.kind === 'number' ? 'decimal' : undefined}}}
      onChange={(e) => updatePolicy({[limit]: e.target.value})}
    />
  );

  const title = (
    <Stack direction="row" spacing={1} alignItems="center" component="span">
      <Box component="span" sx={{display: 'flex', color: 'text.secondary'}}>
        <KindIcon kind={field.kind} />
      </Box>
      <span>{heading}</span>
    </Stack>
  );

  return (
    <Box
      ref={rootRef}
      onKeyDown={(e) => {
        // Escape closes the settings, unless a menu or a list inside them took it to close itself.
        if (e.key !== 'Escape' || e.defaultPrevented) return;
        e.preventDefault();
        onClose();
      }}
    >
      <ComposerPanel
        title={title}
        subtitle={path && path !== heading ? <Box sx={{fontFamily: 'monospace'}}>{path}</Box> : undefined}
        aria-label={t('authorizationDetailTypes:composer.inspector.label', 'Field settings')}
        headingRef={headingRef}
        actions={
          <IconButton
            size="small"
            aria-label={t('authorizationDetailTypes:composer.inspector.close', 'Close field settings')}
            onClick={onClose}
          >
            <X size={16} />
          </IconButton>
        }
        flush
      >
        {notice && <Box sx={{mx: 3, mt: 2}}>{notice}</Box>}

        {kindChange && !readOnly && (
          <Alert
            severity="info"
            data-testid="kind-change-notice"
            sx={{mx: 3, mt: 2}}
            action={
              <>
                <Button
                  color="inherit"
                  size="small"
                  onClick={() => {
                    // The basics edited since the change stay as they are.
                    onChange({...kindChange.before, name: field.name, title: field.title, required: field.required});
                    setKindChange(null);
                  }}
                >
                  {t('authorizationDetailTypes:composer.removed.undo', 'Undo')}
                </Button>
                <IconButton
                  color="inherit"
                  size="small"
                  aria-label={t('common:actions.close', 'Close')}
                  onClick={() => setKindChange(null)}
                >
                  <X size={16} />
                </IconButton>
              </>
            }
          >
            {t(
              'authorizationDetailTypes:composer.kindChange.notice',
              'Changing the type removed {{settings}}, which the new type cannot keep.',
              {
                settings: new Intl.ListFormat(i18n.language, {type: 'conjunction'}).format(
                  kindChange.dropped.map((setting) => droppedLabels[setting]),
                ),
                interpolation: {escapeValue: false},
              },
            )}
          </Alert>
        )}

        {issues.length > 0 && (
          <Alert severity="error" sx={{mx: 3, mt: 2}}>
            <Stack spacing={0.5}>
              {issues.map((issue) => (
                <span key={issue.messageKey}>
                  {t(`authorizationDetailTypes:${issue.messageKey}`, issue.defaultMessage)}
                </span>
              ))}
            </Stack>
          </Alert>
        )}

        <PanelSection
          title={t('authorizationDetailTypes:composer.inspector.basics', 'Basics')}
          summary={[labels.kind[field.kind], field.required ? t('common:form.required', 'Required') : '']
            .filter(Boolean)
            .join(' · ')}
          defaultExpanded
        >
          {/* Spaced by gap, as margins between children would undo the inset that lines up the checkbox. */}
          <Stack spacing={2.5} useFlexGap>
            <TextField
              labelPlacement="top"
              label={t('authorizationDetailTypes:fields.name', 'Field name')}
              required={!readOnly}
              fullWidth
              id={`${field.key}-name`}
              size="small"
              inputRef={nameRef}
              error={hasIssue('nameRequired', 'nameSpaces', 'nameDot', 'nameInnerSpace', 'nameDuplicate')}
              value={field.name}
              disabled={readOnly}
              placeholder={t('authorizationDetailTypes:fields.namePlaceholder', 'e.g., amount')}
              onChange={(e) => update({name: e.target.value})}
            />
            <TextField
              labelPlacement="top"
              label={t('authorizationDetailTypes:fields.title', 'Display name')}
              fullWidth
              id={`${field.key}-title`}
              size="small"
              value={field.title}
              disabled={readOnly}
              placeholder={t('authorizationDetailTypes:fields.titlePlaceholder', 'e.g., Amount')}
              helperText={t('authorizationDetailTypes:fields.titleHint', 'Shown to the user on the consent screen.')}
              onChange={(e) => update({title: e.target.value})}
            />
            <Stack direction="row" spacing={2}>
              <FormControl fullWidth>
                <FormLabel id={`${field.key}-kind-label`}>
                  {t('authorizationDetailTypes:fields.kind.label', 'Type')}
                </FormLabel>
                <Select
                  size="small"
                  labelId={`${field.key}-kind-label`}
                  value={field.kind}
                  disabled={readOnly}
                  onChange={(e) => changeFieldKind(e.target.value as FieldKind)}
                >
                  {KINDS.map((kind) => (
                    <MenuItem key={kind} value={kind}>
                      {labels.kind[kind]}
                    </MenuItem>
                  ))}
                </Select>
              </FormControl>
              {field.kind === 'array' && (
                <FormControl fullWidth>
                  <FormLabel id={`${field.key}-items-label`}>
                    {t('authorizationDetailTypes:fields.itemKind.label', 'Item type')}
                  </FormLabel>
                  <Select
                    size="small"
                    labelId={`${field.key}-items-label`}
                    value={field.itemKind}
                    disabled={readOnly}
                    onChange={(e) => update({itemKind: e.target.value as ItemKind})}
                  >
                    {ITEM_KINDS.map((kind) => (
                      <MenuItem key={kind} value={kind}>
                        {labels.itemKind[kind]}
                      </MenuItem>
                    ))}
                  </Select>
                </FormControl>
              )}
            </Stack>
            <FormControlLabel
              control={
                <Checkbox
                  checked={field.required}
                  disabled={readOnly}
                  onChange={(e) => update({required: e.target.checked})}
                />
              }
              label={label(
                t('common:form.required', 'Required'),
                t('authorizationDetailTypes:fields.requiredTooltip', 'Every request must include this field'),
              )}
            />
          </Stack>
        </PanelSection>

        {hasLimits && (
          <PanelSection
            title={t('authorizationDetailTypes:composer.inspector.limits', 'Limits')}
            summary={limitsSummary()}
            defaultExpanded={hasIssue(...LIMIT_ISSUES)}
          >
            {supportsLimits(field) && (
              <Stack direction="row" spacing={2}>
                {limitInput('minimum')}
                {limitInput('maximum')}
              </Stack>
            )}
            {field.kind === 'array' && (
              <TextField
                labelPlacement="top"
                label={t('authorizationDetailTypes:fields.minItems', 'Minimum number of items')}
                fullWidth
                id={`${field.key}-min-items`}
                size="small"
                error={hasIssue('minItemsInvalid', 'minItemsTooLarge')}
                value={field.minItems ?? ''}
                disabled={readOnly}
                placeholder={t('authorizationDetailTypes:fields.noMinItems', 'No minimum')}
                helperText={t(
                  'authorizationDetailTypes:fields.minItemsHint',
                  'A request must list at least this many items.',
                )}
                slotProps={{htmlInput: {inputMode: 'numeric'}}}
                onChange={(e) => update({minItems: e.target.value})}
              />
            )}
            {field.kind === 'string' && (
              <TextField
                labelPlacement="top"
                label={t('authorizationDetailTypes:fields.maxLength', 'Maximum length')}
                fullWidth
                id={`${field.key}-max-length`}
                size="small"
                error={hasIssue('maxLengthInvalid', 'maxLengthTooLarge')}
                value={field.maxLength ?? ''}
                disabled={readOnly}
                placeholder={t('authorizationDetailTypes:fields.noMaxLength', 'No limit')}
                helperText={t(
                  'authorizationDetailTypes:fields.maxLengthHint',
                  'Values are shown to the user on the consent screen, so keep free text short.',
                )}
                slotProps={{htmlInput: {inputMode: 'numeric'}}}
                onChange={(e) => update({maxLength: e.target.value})}
              />
            )}
          </PanelSection>
        )}

        {field.kind === 'array' && (
          <PanelSection
            title={t('authorizationDetailTypes:fields.allowedValues', 'Allowed values')}
            summary={
              field.allowedValues?.length
                ? field.allowedValues.join(', ')
                : t('authorizationDetailTypes:composer.inspector.anyValue', 'Any value')
            }
            defaultExpanded={hasIssue(...ALLOWED_VALUE_ISSUES)}
          >
            <Autocomplete
              multiple
              freeSolo
              fullWidth
              size="small"
              options={[]}
              value={field.allowedValues ?? []}
              inputValue={allowedDraft}
              disabled={readOnly}
              onInputChange={(_event, text, reason) => {
                if (reason !== 'input') return;
                setAllowedDraft(text);
                setAllowedError(null);
              }}
              // A typed value is added on Enter, below; this removes values.
              onChange={(_event, values, reason) => {
                if (reason !== 'createOption') update({allowedValues: values.map((value) => String(value))});
              }}
              onKeyDown={(event: KeyboardEvent<HTMLDivElement> & {defaultMuiPrevented?: boolean}) => {
                if (event.key !== 'Enter' || !allowedDraft) return;
                // The value is trimmed, as the server refuses one with surrounding space, and a blank or
                // repeated value is refused here rather than dropped.
                event.defaultMuiPrevented = true;
                event.preventDefault();
                const value = allowedDraft.trim();
                const current = field.allowedValues ?? [];
                const same = (other: string): boolean =>
                  field.itemKind === 'number' && Number.isFinite(Number(value)) && other.trim() !== ''
                    ? Number(other) === Number(value)
                    : other.trim() === value;
                // A refused value stays selected, so the next one typed replaces it rather than joining it.
                if (!value || current.some(same)) (event.target as HTMLInputElement).select();
                if (!value) {
                  setAllowedError(
                    t('authorizationDetailTypes:fields.allowedValueBlank', 'Enter a value, not only spaces.'),
                  );
                } else if (current.some(same)) {
                  setAllowedError(
                    t('authorizationDetailTypes:fields.allowedValueRepeated', '{{value}} is already allowed.', {
                      value,
                      interpolation: {escapeValue: false},
                    }),
                  );
                } else {
                  update({allowedValues: [...current, value]});
                  setAllowedDraft('');
                }
              }}
              renderInput={(params) => (
                <TextField
                  {...params}
                  id={`${field.key}-allowed-values`}
                  inputProps={{
                    ...params.inputProps,
                    'aria-label': t('authorizationDetailTypes:fields.allowedValues', 'Allowed values'),
                  }}
                  error={Boolean(allowedError) || hasIssue(...ALLOWED_VALUE_ISSUES)}
                  placeholder={
                    field.allowedValues?.length
                      ? undefined
                      : t('authorizationDetailTypes:fields.anyValue', 'Any value. Type a value and press Enter')
                  }
                  helperText={
                    allowedError ??
                    t(
                      'authorizationDetailTypes:fields.allowedValuesHint',
                      'Each item must be one of these values. Leave empty to allow any value.',
                    )
                  }
                />
              )}
            />
          </PanelSection>
        )}

        <PanelSection
          title={t('authorizationDetailTypes:composer.inspector.policy', 'Who sets and changes it')}
          summary={labels.policySummary(flat)}
          defaultExpanded={hasIssue(...POLICY_ISSUES)}
        >
          <Stack spacing={3}>
            <Typography
              variant="body2"
              data-testid={`${field.key}-rule`}
              sx={{px: 1.5, py: 1, borderRadius: 1.5, bgcolor: 'action.hover'}}
            >
              {labels.policySummary(flat)}
            </Typography>

            <FormControl fullWidth component="fieldset" disabled={readOnly}>
              <FormLabel component="legend" id={`${field.key}-source-label`} sx={{mb: 0.5}}>
                {t('authorizationDetailTypes:composer.inspector.source', 'Who provides this value?')}
              </FormLabel>
              <RadioGroup
                aria-labelledby={`${field.key}-source-label`}
                value={supplied ? 'processor' : 'client'}
                onChange={(_, next) => onChange(setSuppliedBy(field, next === 'processor'))}
              >
                {option(
                  <Radio
                    size="small"
                    value="client"
                    slotProps={{input: {'aria-describedby': `${field.key}-source-client`}}}
                  />,
                  t('authorizationDetailTypes:composer.inspector.sourceClient', 'The client, in its request'),
                  t(
                    'authorizationDetailTypes:composer.inspector.sourceClientHint',
                    'The client that asks for approval sends this value.',
                  ),
                  `${field.key}-source-client`,
                )}
                {option(
                  <Radio
                    size="small"
                    value="processor"
                    slotProps={{input: {'aria-describedby': `${field.key}-source-processor`}}}
                  />,
                  t(
                    'authorizationDetailTypes:composer.inspector.sourceProcessor',
                    'Your external processor, after the request',
                  ),
                  field.kind === 'object'
                    ? t(
                        'authorizationDetailTypes:composer.inspector.sourceProcessorObjectHint',
                        'The external processor, set on the External processor tab, fills in every field of this object. Clients cannot send it.',
                      )
                    : t(
                        'authorizationDetailTypes:composer.inspector.sourceProcessorHint',
                        'The external processor, set on the External processor tab, fills it in. Clients cannot send it.',
                      ),
                  `${field.key}-source-processor`,
                )}
              </RadioGroup>
              {inheritsProcessor && (
                <FormHelperText sx={{mx: 0}}>
                  {t(
                    'authorizationDetailTypes:composer.inspector.inheritsProcessor',
                    'Your external processor fills in the whole object this field belongs to.',
                  )}
                </FormHelperText>
              )}
            </FormControl>

            {!fixed && modifications.length > 1 && (
              <FormControl fullWidth component="fieldset" disabled={readOnly}>
                <FormLabel component="legend" id={`${field.key}-change-label`} sx={{mb: 0.5}}>
                  {t('authorizationDetailTypes:composer.inspector.change', 'Can it change after the request?')}
                </FormLabel>
                <RadioGroup
                  aria-labelledby={`${field.key}-change-label`}
                  value={modification}
                  onChange={(_, next) => updatePolicy({modification: next as Modification})}
                >
                  {modifications.map((m) => (
                    <FormControlLabel
                      key={m}
                      value={m}
                      control={<Radio size="small" />}
                      label={labels.modification(field, m)}
                      slotProps={{typography: {variant: 'body2', sx: {fontWeight: 500}}}}
                      sx={{mr: 0}}
                    />
                  ))}
                </RadioGroup>
                <FormHelperText sx={{mx: 0}}>
                  {t(
                    'authorizationDetailTypes:composer.inspector.changeHint',
                    'Allow only the direction that gives less access, such as a lower spending limit or an earlier end date.',
                  )}
                </FormHelperText>
              </FormControl>
            )}

            {!fixed && modifications.length === 1 && (
              <Typography variant="body2" color="text.secondary">
                {t(
                  'authorizationDetailTypes:composer.inspector.noChanges',
                  'Only numbers, dates, and lists can change after the request, so this value stays as requested.',
                )}
              </Typography>
            )}

            {modification !== 'none' && (
              <FormControl fullWidth component="fieldset" disabled={readOnly} error={hasIssue('modifiableByRequired')}>
                <FormLabel component="legend" sx={{mb: 0.5}}>
                  {t('authorizationDetailTypes:composer.inspector.who', 'Who can change it?')}
                </FormLabel>
                <Stack spacing={0.5}>
                  {parties.map((party) =>
                    option(
                      <Checkbox
                        size="small"
                        checked={field.policy.modifiableBy?.includes(party) ?? false}
                        onChange={() => onChange(toggleParty(field, party))}
                        slotProps={{input: {'aria-describedby': `${field.key}-party-${party}`}}}
                      />,
                      labels.party[party].label,
                      labels.party[party].hint,
                      `${field.key}-party-${party}`,
                    ),
                  )}
                </Stack>
              </FormControl>
            )}

            {modification === 'addItems' && (
              <Alert severity="warning">
                {t(
                  'authorizationDetailTypes:fields.addItemsWarning',
                  'Use this only for lists whose items take permission away, such as excluded merchants. On a list like actions, adding items would widen what was approved.',
                )}
              </Alert>
            )}
          </Stack>
        </PanelSection>
      </ComposerPanel>
    </Box>
  );
}
