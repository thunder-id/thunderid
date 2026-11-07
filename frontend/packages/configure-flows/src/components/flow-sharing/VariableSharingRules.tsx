// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {SettingsCard} from '@thunderid/components';
import type {OverlayRules} from '@thunderid/configure-resource-sharing';
import {getFlowVariableOptions} from '@thunderid/react';
import type {FlowVariableDefinition, FlowVariableOption} from '@thunderid/react';
import {
  Autocomplete,
  Box,
  Chip,
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
import {ChevronDown, ChevronRight} from '@wso2/oxygen-ui-icons-react';
import {useState, type ReactElement} from 'react';
import {useTranslation} from 'react-i18next';
import {hasChoices, ruleKeyOf, ruleToForm, withVariableRule, type VariableRuleForm} from './variableRules';
import useFlowVariableConnections from '../variables-dialog/useFlowVariableConnections';

const INPUT_TYPES: Partial<Record<FlowVariableDefinition['type'], string>> = {
  DATE_INPUT: 'date',
  EMAIL_INPUT: 'email',
  NUMBER_INPUT: 'number',
  PHONE_INPUT: 'tel',
};

/**
 * Props interface of {@link VariableSharingRules}
 */
export interface VariableSharingRulesProps {
  /**
   * Leaves out the card around the rules, for a place that frames them itself.
   */
  bare?: boolean;
  /**
   * Stops the rules being changed, as for a flow a resource file declares.
   */
  disabled?: boolean;
  /**
   * Called with the rules once one of them changes.
   */
  onChange: (rules: OverlayRules) => void;
  /**
   * The terms the units reached hold the flow on, by `variables.<identifier>`.
   */
  rules: OverlayRules | undefined;
  /**
   * The variables the flow declares.
   */
  variables: FlowVariableDefinition[];
}

/**
 * What the organization units a flow is shared with may do with each of its variables: set their own value, which
 * choices they may pick from, and what they start with. A variable the units may not set stays at the flow's default,
 * or at the value given here.
 *
 * @param props - Props injected to the component.
 * @returns The VariableSharingRules component.
 */
function VariableSharingRules({
  bare = false,
  disabled = false,
  onChange,
  rules,
  variables,
}: VariableSharingRulesProps): ReactElement {
  const {t} = useTranslation();
  const connections = useFlowVariableConnections();
  const [open, setOpen] = useState<string | null>(null);

  const renderValue = (
    definition: FlowVariableDefinition,
    options: FlowVariableOption[],
    form: VariableRuleForm,
    update: (next: Partial<VariableRuleForm>) => void,
  ): ReactElement => {
    const id = `share-${definition.identifier}-value`;

    if (hasChoices(definition) || definition.type === 'BOOLEAN_INPUT') {
      const choices: FlowVariableOption[] =
        definition.type === 'BOOLEAN_INPUT'
          ? [
              {label: t('flows:core.sharing.variables.yes', 'Yes'), value: 'true'},
              {label: t('flows:core.sharing.variables.no', 'No'), value: 'false'},
            ]
          : options;

      return (
        <Select
          id={id}
          value={form.value}
          onChange={(event) => update({value: event.target.value})}
          displayEmpty
          renderValue={(value: string) =>
            value === '' ? (
              <Typography component="span" color="text.disabled">
                {t('flows:core.sharing.variables.useFlowDefault', "Use the flow's default")}
              </Typography>
            ) : (
              (choices.find((choice: FlowVariableOption) => choice.value === value)?.label ?? value)
            )
          }
          fullWidth
          size="small"
          disabled={disabled}
        >
          <MenuItem value="">{t('flows:core.sharing.variables.useFlowDefault', "Use the flow's default")}</MenuItem>
          {choices.map((choice: FlowVariableOption) => (
            <MenuItem key={choice.value} value={choice.value}>
              {choice.label}
            </MenuItem>
          ))}
        </Select>
      );
    }

    return (
      <TextField
        id={id}
        value={form.value}
        onChange={(event) => update({value: event.target.value})}
        placeholder={t('flows:core.sharing.variables.useFlowDefault', "Use the flow's default")}
        type={INPUT_TYPES[definition.type] ?? 'text'}
        fullWidth
        size="small"
        disabled={disabled}
      />
    );
  };

  const renderChoices = (
    label: string,
    hint: string,
    options: FlowVariableOption[],
    selected: string[],
    onSelect: (values: string[]) => void,
    id: string,
  ): ReactElement => (
    <div>
      <FormLabel htmlFor={id}>{label}</FormLabel>
      <Autocomplete
        multiple
        id={id}
        options={options.map((option: FlowVariableOption) => option.value)}
        value={selected}
        onChange={(_event, values: string[]) => onSelect(values)}
        getOptionLabel={(value: string) => options.find((option) => option.value === value)?.label ?? value}
        renderValue={(values: string[], getItemProps) =>
          values.map((value: string, index: number) => {
            const {key, ...itemProps} = getItemProps({index});

            return (
              <Chip
                key={key}
                size="small"
                label={options.find((option) => option.value === value)?.label ?? value}
                {...itemProps}
              />
            );
          })
        }
        renderInput={(params) => (
          <TextField
            {...params}
            size="small"
            placeholder={
              selected.length === 0 ? t('flows:core.sharing.variables.choicesPlaceholder', 'Select choices') : undefined
            }
          />
        )}
        disabled={disabled}
        size="small"
        fullWidth
      />
      <FormHelperText>{hint}</FormHelperText>
    </div>
  );

  const summaryOf = (form: VariableRuleForm, options: FlowVariableOption[]): string => {
    const shown: string =
      options.find((option: FlowVariableOption) => option.value === form.value)?.label ?? form.value;

    if (form.editable) {
      return form.value === ''
        ? t(
            'flows:core.sharing.variables.summaryChooseDefault',
            "Units start with the flow's default and can set their own.",
          )
        : t('flows:core.sharing.variables.summaryChoose', 'Units start with {{value}} and can set their own.', {
            value: shown,
          });
    }

    return form.value === ''
      ? t(
          'flows:core.sharing.variables.summaryLockedDefault',
          "Units always use the flow's default and can't change it.",
        )
      : t('flows:core.sharing.variables.summaryLocked', "Units always use {{value}} and can't change it.", {
          value: shown,
        });
  };

  const list = (
    <Stack spacing={2}>
      {variables.map((definition: FlowVariableDefinition) => {
        const form: VariableRuleForm = ruleToForm(rules?.[ruleKeyOf(definition.identifier)]);
        const options: FlowVariableOption[] = getFlowVariableOptions(definition, connections);
        const update = (next: Partial<VariableRuleForm>): void =>
          onChange(withVariableRule(rules, definition.identifier, {...form, ...next}));

        const isOpen: boolean = open === definition.identifier;
        const toggle = (): void => setOpen(isOpen ? null : definition.identifier);

        return (
          <Box
            key={definition.identifier}
            data-testid={`share-variable-${definition.identifier}`}
            sx={{
              border: '1px solid',
              borderColor: isOpen ? 'primary.main' : 'divider',
              borderRadius: 1,
              overflow: 'hidden',
            }}
          >
            <Stack
              direction="row"
              alignItems="center"
              gap={1.5}
              role="button"
              tabIndex={0}
              aria-expanded={isOpen}
              onClick={toggle}
              onKeyDown={(event) => {
                if (event.key === 'Enter' || event.key === ' ') {
                  event.preventDefault();
                  toggle();
                }
              }}
              sx={{
                px: 2,
                py: 1.5,
                cursor: 'pointer',
                bgcolor: isOpen ? 'action.selected' : 'transparent',
                '&:hover': {bgcolor: 'action.hover'},
              }}
            >
              {isOpen ? <ChevronDown size={16} /> : <ChevronRight size={16} />}
              <Box flex={1} minWidth={0}>
                <Typography variant="subtitle2" noWrap>
                  {definition.label?.trim() ? definition.label : definition.identifier}
                </Typography>
                <Typography variant="caption" color="text.secondary" noWrap display="block">
                  {definition.identifier}
                </Typography>
              </Box>
              <Typography
                variant="body2"
                color="text.secondary"
                sx={{display: {xs: 'none', sm: 'block'}, maxWidth: '55%', textAlign: 'right'}}
                data-testid={`share-summary-${definition.identifier}`}
              >
                {summaryOf(form, options)}
              </Typography>
            </Stack>
            {isOpen && (
              <Stack spacing={1.5} sx={{p: 2, borderTop: '1px solid', borderColor: 'divider'}}>
                <FormControlLabel
                  control={
                    <Switch
                      checked={form.editable}
                      disabled={disabled}
                      onChange={(_event, checked: boolean) => update({editable: checked})}
                    />
                  }
                  label={t('flows:core.sharing.variables.editable', 'Editable')}
                  sx={{m: 0, gap: 1}}
                />
                {form.editable && hasChoices(definition) && (
                  <>
                    {renderChoices(
                      t('flows:core.sharing.variables.allowed', 'Choices they may pick from'),
                      t('flows:core.sharing.variables.allowedHint', 'Leave empty to allow every choice.'),
                      options,
                      form.allowedValues,
                      (values: string[]) => update({allowedValues: values}),
                      `share-${definition.identifier}-allowed`,
                    )}
                    {renderChoices(
                      t('flows:core.sharing.variables.excluded', 'Choices taken away'),
                      t('flows:core.sharing.variables.excludedHint', 'Removed from the choices above.'),
                      options,
                      form.excludedValues,
                      (values: string[]) => update({excludedValues: values}),
                      `share-${definition.identifier}-excluded`,
                    )}
                  </>
                )}
                <div>
                  <FormLabel htmlFor={`share-${definition.identifier}-value`}>
                    {t('flows:core.sharing.variables.unitDefault', 'Unit default')}
                  </FormLabel>
                  {renderValue(definition, options, form, update)}
                  <FormHelperText>
                    {t(
                      'flows:core.sharing.variables.unitDefaultHint',
                      "Customize the default for this share. Leave blank to use the default from the flow's variable definition.",
                    )}
                  </FormHelperText>
                </div>
              </Stack>
            )}
          </Box>
        );
      })}
    </Stack>
  );

  if (bare) {
    return list;
  }

  return (
    <SettingsCard
      title={t('flows:core.sharing.variables.title', 'Terms for every granted child unit')}
      description={t(
        'flows:core.sharing.variables.description',
        'Without a rule, a variable works as the flow sets it. Lock a variable, or let the units choose within limits you set.',
      )}
    >
      {list}
    </SettingsCard>
  );
}

export default VariableSharingRules;
