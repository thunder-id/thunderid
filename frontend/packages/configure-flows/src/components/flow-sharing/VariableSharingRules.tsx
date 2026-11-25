// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {SettingsCard} from '@thunderid/components';
import type {OverlayRules} from '@thunderid/configure-resource-sharing';
import {getFlowVariableOptions} from '@thunderid/react';
import type {FlowVariableDefinition, FlowVariableOption} from '@thunderid/react';
import {
  Accordion,
  AccordionDetails,
  AccordionSummary,
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
import {ChevronDown} from '@wso2/oxygen-ui-icons-react';
import {useState, type ReactElement} from 'react';
import {useTranslation} from 'react-i18next';
import {
  hasChoices,
  isAllowedDefault,
  isInvalidUnitDefault,
  ruleKeyOf,
  ruleToForm,
  withVariableRule,
  type VariableRuleForm,
} from './variableRules';
import ConnectionsLoadError from '../variables-dialog/ConnectionsLoadError';
import useFlowVariableConnections from '../variables-dialog/useFlowVariableConnections';
import {INPUT_TYPES} from '../variables-dialog/variableDefinitions';

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
   * The organization unit that owns the flow, which limits the connections offered to the ones it owns or was shared.
   */
  ownerOuId?: string;
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
  ownerOuId = undefined,
  rules,
  variables,
}: VariableSharingRulesProps): ReactElement {
  const {t} = useTranslation();
  const {connections, error: connectionsError, refetch: refetchConnections} = useFlowVariableConnections(ownerOuId);
  const [open, setOpen] = useState<string | null>(null);

  const renderValue = (
    definition: FlowVariableDefinition,
    options: FlowVariableOption[],
    form: VariableRuleForm,
    update: (next: Partial<VariableRuleForm>) => void,
    invalid: boolean,
  ): ReactElement => {
    const id = `share-${definition.identifier}-value`;
    const labelId = `${id}-label`;

    if (hasChoices(definition) || definition.type === 'BOOLEAN_INPUT') {
      const choices: FlowVariableOption[] =
        definition.type === 'BOOLEAN_INPUT'
          ? [
              {label: t('common:actions.yes', 'Yes'), value: 'true'},
              {label: t('common:actions.no', 'No'), value: 'false'},
            ]
          : // A choice the units may not pick from cannot be the default they start with.
            options.filter((option: FlowVariableOption) => !form.editable || isAllowedDefault(form, option.value));

      return (
        <Select
          id={id}
          labelId={labelId}
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
        error={invalid}
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
            "Organization units start with the flow's default and can set their own.",
          )
        : t(
            'flows:core.sharing.variables.summaryChoose',
            'Organization units start with {{value}} and can set their own.',
            {
              value: shown,
            },
          );
    }

    return form.value === ''
      ? t(
          'flows:core.sharing.variables.summaryLockedDefault',
          "Organization units always use the flow's default and can't change it.",
        )
      : t(
          'flows:core.sharing.variables.summaryLocked',
          "Organization units always use {{value}} and can't change it.",
          {
            value: shown,
          },
        );
  };

  const list = (
    <Stack spacing={2}>
      {connectionsError && <ConnectionsLoadError error={connectionsError} onRetry={refetchConnections} />}
      {variables.map((definition: FlowVariableDefinition) => {
        const form: VariableRuleForm = ruleToForm(rules?.[ruleKeyOf(definition.identifier)]);
        const options: FlowVariableOption[] = getFlowVariableOptions(definition, connections);
        const update = (next: Partial<VariableRuleForm>): void => {
          const merged: VariableRuleForm = {...form, ...next};
          // Narrowing the choices can leave the default outside them, which the server would refuse, so it is cleared.
          const kept: VariableRuleForm =
            hasChoices(definition) && merged.editable && merged.value !== '' && !isAllowedDefault(merged, merged.value)
              ? {...merged, value: ''}
              : merged;

          onChange(withVariableRule(rules, definition.identifier, kept));
        };

        const invalidDefault: boolean = isInvalidUnitDefault(definition, form.value);
        const isOpen: boolean = open === definition.identifier;

        return (
          <Accordion
            key={definition.identifier}
            data-testid={`share-variable-${definition.identifier}`}
            expanded={isOpen}
            onChange={(_event, expanded: boolean) => setOpen(expanded ? definition.identifier : null)}
            disableGutters
            square
            slotProps={{transition: {unmountOnExit: true}}}
            sx={{
              border: '1px solid',
              borderColor: isOpen ? 'primary.main' : 'divider',
              borderRadius: 1,
              boxShadow: 'none',
              overflow: 'hidden',
              '&:before': {display: 'none'},
            }}
          >
            <AccordionSummary
              expandIcon={<ChevronDown size={16} />}
              aria-controls={`share-${definition.identifier}-content`}
              id={`share-${definition.identifier}-header`}
              sx={{bgcolor: isOpen ? 'action.selected' : 'transparent', px: 2}}
              slotProps={{content: {sx: {alignItems: 'center', gap: 1.5, minWidth: 0}}}}
            >
              <Box flex={1} minWidth={0}>
                <Typography variant="subtitle2" component="span" noWrap display="block">
                  {definition.label?.trim() ? definition.label : definition.identifier}
                </Typography>
                <Typography variant="caption" component="span" color="text.secondary" noWrap display="block">
                  {definition.identifier}
                </Typography>
              </Box>
              <Typography
                variant="body2"
                component="span"
                color="text.secondary"
                sx={{display: {xs: 'none', sm: 'block'}, maxWidth: '55%', textAlign: 'right'}}
                data-testid={`share-summary-${definition.identifier}`}
              >
                {summaryOf(form, options)}
              </Typography>
            </AccordionSummary>
            <AccordionDetails sx={{borderTop: '1px solid', borderColor: 'divider', p: 2}}>
              <Stack spacing={1.5}>
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
                  <FormLabel
                    id={`share-${definition.identifier}-value-label`}
                    htmlFor={`share-${definition.identifier}-value`}
                  >
                    {t('flows:core.sharing.variables.unitDefault', 'Organization unit default')}
                  </FormLabel>
                  {renderValue(definition, options, form, update, invalidDefault)}
                  <FormHelperText error={invalidDefault}>
                    {invalidDefault
                      ? t(
                          'flows:core.sharing.variables.unitDefaultInvalid',
                          'Enter a default that is valid for this input.',
                        )
                      : t(
                          'flows:core.sharing.variables.unitDefaultHint',
                          "Customize the default for this share. Leave blank to use the default from the flow's variable definition.",
                        )}
                  </FormHelperText>
                </div>
              </Stack>
            </AccordionDetails>
          </Accordion>
        );
      })}
    </Stack>
  );

  if (bare) {
    return list;
  }

  return (
    <SettingsCard
      title={t('flows:core.sharing.variables.title', 'Terms for every granted child organization unit')}
      description={t(
        'flows:core.sharing.variables.description',
        'Without a rule, a variable works as the flow sets it. Lock a variable, or let the organization units choose within limits you set.',
      )}
    >
      {list}
    </SettingsCard>
  );
}

export default VariableSharingRules;
