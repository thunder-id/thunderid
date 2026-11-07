// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {FlowVariableDefinition, FlowVariableOption, FlowVariableType, FlowVariableValue} from '@thunderid/react';
import {
  Autocomplete,
  Box,
  Chip,
  FormControl,
  FormControlLabel,
  FormHelperText,
  FormLabel,
  MenuItem,
  Select,
  Stack,
  Switch,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import type {ChangeEvent, ReactElement} from 'react';
import {useTranslation} from 'react-i18next';
import {
  CONNECTION_TYPES,
  VARIABLE_TYPES,
  defaultValueFor,
  findProblems,
  formatOptions,
  isValidIdentifier,
  parseOptions,
} from './variableDefinitions';

/**
 * Props interface of {@link VariableEditor}
 */
export interface VariableEditorProps {
  /**
   * The connections a default can be picked from, for a connection variable.
   */
  connectionOptions: FlowVariableOption[];
  /**
   * Every variable of the flow, which tells whether this one's identifier is taken.
   */
  definitions: FlowVariableDefinition[];
  /**
   * Position of the edited variable among the others.
   */
  index: number;
  onChange: (definition: FlowVariableDefinition) => void;
}

const TEXT_TYPES: FlowVariableType[] = ['TEXT_INPUT', 'EMAIL_INPUT', 'PHONE_INPUT'];

/**
 * Sets one field of a variable, dropping the field when it is emptied so it is not written as an empty value.
 */
const withField = <K extends keyof FlowVariableDefinition>(
  definition: FlowVariableDefinition,
  key: K,
  value: FlowVariableDefinition[K] | undefined,
): FlowVariableDefinition => {
  const next: FlowVariableDefinition = {...definition};

  if (value === undefined || value === '' || (Array.isArray(value) && value.length === 0)) {
    delete next[key];
  } else {
    next[key] = value as FlowVariableDefinition[K];
  }

  return next;
};

/**
 * The right side of the variables editor: every setting of one variable, which is what the flow's owner decides
 * and a delegated admin then sees as a label, a description and an input.
 */
function VariableEditor({connectionOptions, definitions, index, onChange}: VariableEditorProps): ReactElement {
  const {t} = useTranslation();
  const definition: FlowVariableDefinition = definitions[index];
  const problems: ReturnType<typeof findProblems> = findProblems(definitions, index);
  const isText: boolean = TEXT_TYPES.includes(definition.type);

  const rule = (type: 'maxLength' | 'minLength' | 'regex'): string | number | undefined =>
    definition.validation?.find((candidate) => candidate.type === type)?.value;

  const setRule = (type: 'maxLength' | 'minLength' | 'regex', value: string): void => {
    const others = (definition.validation ?? []).filter((candidate) => candidate.type !== type);
    const next = value === '' ? others : [...others, {type, value: type === 'regex' ? value : Number(value)}];

    onChange(withField(definition, 'validation', next));
  };

  const changeType = (type: FlowVariableType): void => {
    const next: FlowVariableDefinition = {
      defaultValue: defaultValueFor(type),
      identifier: definition.identifier,
      type,
    };

    if (definition.label) {
      next.label = definition.label;
    }
    if (definition.description) {
      next.description = definition.description;
    }
    if (type === 'CONNECTION_SELECT') {
      next.category = 'IDENTITY_PROVIDER';
    }

    onChange(next);
  };

  const renderDefault = (): ReactElement => {
    switch (definition.type) {
      case 'BOOLEAN_INPUT':
        return (
          <FormControlLabel
            control={
              <Switch
                checked={definition.defaultValue === true}
                onChange={(event: ChangeEvent<HTMLInputElement>) =>
                  onChange(withField(definition, 'defaultValue', event.target.checked))
                }
              />
            }
            label={t('flows:core.variables.editor.defaultOn', 'On by default')}
          />
        );
      case 'SELECT':
      case 'CONNECTION_SELECT': {
        const choices: {label: string; value: string}[] =
          definition.type === 'SELECT'
            ? (definition.options ?? []).map((option: FlowVariableValue) => ({
                label: String(option),
                value: String(option),
              }))
            : connectionOptions;

        return (
          <Select
            size="small"
            fullWidth
            displayEmpty
            renderValue={(value: string) =>
              value === '' ? (
                <Typography component="span" color="text.disabled">
                  {definition.type === 'CONNECTION_SELECT'
                    ? t('flows:core.variables.editor.blankDefault', 'None')
                    : t('flows:core.variables.editor.selectDefault', 'Select a default')}
                </Typography>
              ) : (
                (choices.find((choice) => choice.value === value)?.label ?? value)
              )
            }
            value={definition.defaultValue === undefined ? '' : String(definition.defaultValue)}
            inputProps={{'aria-label': t('flows:core.variables.editor.defaultValue', 'Default value')}}
            onChange={(event) => {
              const chosen: FlowVariableValue | undefined = (definition.options ?? []).find(
                (option: FlowVariableValue) => String(option) === event.target.value,
              );

              onChange({...definition, defaultValue: chosen ?? String(event.target.value)});
            }}
          >
            {definition.type === 'CONNECTION_SELECT' ? (
              <MenuItem value="">{t('flows:core.variables.editor.blankDefault', 'None')}</MenuItem>
            ) : (
              <MenuItem value="" disabled>
                {t('flows:core.variables.editor.selectDefault', 'Select a default')}
              </MenuItem>
            )}
            {choices.map((choice) => (
              <MenuItem key={choice.value} value={choice.value}>
                {choice.label}
              </MenuItem>
            ))}
          </Select>
        );
      }
      default:
        return (
          <TextField
            size="small"
            fullWidth
            type={definition.type === 'NUMBER_INPUT' ? 'number' : definition.type === 'DATE_INPUT' ? 'date' : 'text'}
            placeholder={t('flows:core.variables.editor.defaultPlaceholder', 'No default')}
            value={definition.defaultValue === undefined ? '' : String(definition.defaultValue)}
            inputProps={{'aria-label': t('flows:core.variables.editor.defaultValue', 'Default value')}}
            error={problems.includes('default')}
            helperText={
              problems.includes('default')
                ? t('flows:core.variables.editor.defaultInvalid', 'Enter a default that is valid for this input.')
                : undefined
            }
            onChange={(event: ChangeEvent<HTMLInputElement>) => {
              const raw: string = event.target.value;

              onChange({
                ...definition,
                defaultValue:
                  definition.type === 'NUMBER_INPUT' && raw !== '' && !Number.isNaN(Number(raw)) ? Number(raw) : raw,
              });
            }}
          />
        );
    }
  };

  return (
    <Stack gap={2.5} data-testid="variable-editor">
      <FormControl fullWidth size="small" error={problems.includes('identifier') || problems.includes('duplicate')}>
        <FormLabel htmlFor="variable-identifier">{t('flows:core.variables.editor.identifier', 'Identifier')}</FormLabel>
        <TextField
          id="variable-identifier"
          size="small"
          placeholder={t('flows:core.variables.editor.identifierPlaceholder', 'support_email')}
          value={definition.identifier}
          error={!isValidIdentifier(definition.identifier) || problems.includes('duplicate')}
          onChange={(event: ChangeEvent<HTMLInputElement>) => onChange({...definition, identifier: event.target.value})}
        />
        <FormHelperText>
          {problems.includes('duplicate')
            ? t('flows:core.variables.editor.identifierTaken', 'Another variable already uses this identifier.')
            : problems.includes('identifier')
              ? t(
                  'flows:core.variables.editor.identifierInvalid',
                  'Start with a letter, then use letters, digits and underscores.',
                )
              : t('flows:core.variables.editor.identifierHelp', 'Steps read it as {{placeholder}}.', {
                  placeholder: `{{var(${definition.identifier.trim() === '' ? 'identifier' : definition.identifier.trim()})}}`,
                })}
        </FormHelperText>
      </FormControl>

      <FormControl fullWidth size="small">
        <FormLabel htmlFor="variable-type">{t('flows:core.variables.editor.type', 'Type')}</FormLabel>
        <Select
          id="variable-type"
          size="small"
          value={definition.type}
          onChange={(event) => changeType(event.target.value as FlowVariableType)}
        >
          {VARIABLE_TYPES.map((type) => (
            <MenuItem key={type.value} value={type.value}>
              {t(`flows:core.variables.types.${type.value}`, type.label)}
            </MenuItem>
          ))}
        </Select>
      </FormControl>

      <FormControl fullWidth size="small">
        <FormLabel htmlFor="variable-label">{t('flows:core.variables.editor.label', 'Label')}</FormLabel>
        <TextField
          id="variable-label"
          size="small"
          placeholder={t('flows:core.variables.editor.labelPlaceholder', 'Support email')}
          value={definition.label ?? ''}
          onChange={(event: ChangeEvent<HTMLInputElement>) =>
            onChange(withField(definition, 'label', event.target.value))
          }
        />
      </FormControl>

      <FormControl fullWidth size="small">
        <FormLabel htmlFor="variable-description">
          {t('flows:core.variables.editor.description', 'Description')}
        </FormLabel>
        <TextField
          id="variable-description"
          size="small"
          placeholder={t(
            'flows:core.variables.editor.descriptionPlaceholder',
            'Shown to the delegated admin under the label',
          )}
          multiline
          minRows={2}
          value={definition.description ?? ''}
          onChange={(event: ChangeEvent<HTMLInputElement>) =>
            onChange(withField(definition, 'description', event.target.value))
          }
        />
      </FormControl>

      {definition.type === 'SELECT' && (
        <FormControl fullWidth size="small" error={problems.includes('options')}>
          <FormLabel htmlFor="variable-options">{t('flows:core.variables.editor.options', 'Options')}</FormLabel>
          <TextField
            id="variable-options"
            size="small"
            placeholder={t('flows:core.variables.editor.optionsPlaceholder', 'Option one\nOption two')}
            multiline
            minRows={3}
            defaultValue={formatOptions(definition.options)}
            onChange={(event: ChangeEvent<HTMLInputElement>) =>
              onChange(withField(definition, 'options', parseOptions(event.target.value)))
            }
          />
          <FormHelperText>{t('flows:core.variables.editor.optionsHelp', 'One option per line.')}</FormHelperText>
        </FormControl>
      )}

      {definition.type === 'NUMBER_INPUT' && (
        <Stack direction="row" gap={2}>
          <TextField
            size="small"
            type="number"
            label={t('flows:core.variables.editor.min', 'Minimum')}
            placeholder={t('flows:core.variables.editor.minPlaceholder', 'No minimum')}
            slotProps={{inputLabel: {shrink: true}}}
            value={definition.min ?? ''}
            onChange={(event: ChangeEvent<HTMLInputElement>) =>
              onChange(withField(definition, 'min', event.target.value === '' ? undefined : Number(event.target.value)))
            }
          />
          <TextField
            size="small"
            type="number"
            label={t('flows:core.variables.editor.max', 'Maximum')}
            placeholder={t('flows:core.variables.editor.maxPlaceholder', 'No maximum')}
            slotProps={{inputLabel: {shrink: true}}}
            value={definition.max ?? ''}
            onChange={(event: ChangeEvent<HTMLInputElement>) =>
              onChange(withField(definition, 'max', event.target.value === '' ? undefined : Number(event.target.value)))
            }
          />
        </Stack>
      )}

      {definition.type === 'CONNECTION_SELECT' && (
        <FormControl fullWidth size="small">
          <FormLabel htmlFor="variable-connection-types">
            {t('flows:core.variables.editor.connectionTypes', 'Allowed connection types')}
          </FormLabel>
          <Autocomplete
            multiple
            id="variable-connection-types"
            size="small"
            options={CONNECTION_TYPES.map((type) => type.value)}
            value={definition.connectionTypes ?? []}
            onChange={(_event, values: string[]) => onChange(withField(definition, 'connectionTypes', values))}
            getOptionLabel={(value: string) => CONNECTION_TYPES.find((type) => type.value === value)?.label ?? value}
            renderValue={(values: string[], getItemProps) =>
              values.map((value: string, index: number) => {
                const {key, ...itemProps} = getItemProps({index});

                return (
                  <Chip
                    key={key}
                    size="small"
                    label={CONNECTION_TYPES.find((type) => type.value === value)?.label ?? value}
                    {...itemProps}
                  />
                );
              })
            }
            renderInput={(params) => (
              <TextField
                {...params}
                placeholder={
                  (definition.connectionTypes ?? []).length === 0
                    ? t('flows:core.variables.editor.connectionTypesAll', 'All')
                    : undefined
                }
              />
            )}
          />
          <FormHelperText>
            {t(
              'flows:core.variables.editor.connectionTypesHelp',
              'Limit the connections offered to these types. Leave empty for all.',
            )}
          </FormHelperText>
        </FormControl>
      )}

      <FormControl fullWidth size="small">
        <FormLabel>{t('flows:core.variables.editor.defaultValue', 'Default value')}</FormLabel>
        {renderDefault()}
        {definition.type === 'CONNECTION_SELECT' &&
          (definition.defaultValue === undefined || definition.defaultValue === '') && (
            <FormHelperText>
              {t(
                'flows:core.variables.editor.blankDefaultHelp',
                'With no default, the flow is not available to a unit until it picks a connection.',
              )}
            </FormHelperText>
          )}
      </FormControl>

      {isText && (
        <Box>
          <Typography variant="subtitle2" sx={{mb: 1}}>
            {t('flows:core.variables.editor.rules', 'Rules')}
          </Typography>
          <Stack gap={2}>
            <Stack direction="row" gap={2}>
              <TextField
                size="small"
                type="number"
                label={t('flows:core.variables.editor.minLength', 'Minimum length')}
                placeholder={t('flows:core.variables.editor.minLengthPlaceholder', 'No minimum')}
                slotProps={{inputLabel: {shrink: true}}}
                value={rule('minLength') ?? ''}
                onChange={(event: ChangeEvent<HTMLInputElement>) => setRule('minLength', event.target.value)}
              />
              <TextField
                size="small"
                type="number"
                label={t('flows:core.variables.editor.maxLength', 'Maximum length')}
                placeholder={t('flows:core.variables.editor.maxLengthPlaceholder', 'No maximum')}
                slotProps={{inputLabel: {shrink: true}}}
                value={rule('maxLength') ?? ''}
                onChange={(event: ChangeEvent<HTMLInputElement>) => setRule('maxLength', event.target.value)}
              />
            </Stack>
            <TextField
              size="small"
              fullWidth
              label={t('flows:core.variables.editor.regex', 'Pattern (regular expression)')}
              placeholder={t('flows:core.variables.editor.regexPlaceholder', '^[a-z]+$')}
              slotProps={{inputLabel: {shrink: true}}}
              value={rule('regex') ?? ''}
              onChange={(event: ChangeEvent<HTMLInputElement>) => setRule('regex', event.target.value)}
            />
          </Stack>
        </Box>
      )}
    </Stack>
  );
}

export default VariableEditor;
