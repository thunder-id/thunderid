// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {OverlayRule, OverlayRules} from '@thunderid/configure-resource-sharing';
import type {FlowVariableDefinition} from '@thunderid/react';

/**
 * What a sharing policy says the units reached may do with one variable, in the terms the Share screen edits.
 */
export interface VariableRuleForm {
  /**
   * Choices the unit may not pick, taken away last.
   */
  excludedValues: string[];
  /**
   * Choices the unit may pick from. Empty leaves every choice open.
   */
  allowedValues: string[];
  /**
   * Whether the unit may set its own value.
   */
  editable: boolean;
  /**
   * What the unit starts with, or is held to when it may not edit. Empty keeps the flow's own default.
   */
  value: string;
}

/**
 * The key a variable's rule goes under in a sharing policy.
 */
export const ruleKeyOf = (identifier: string): string => `variables.${identifier}`;

/**
 * Tells whether a variable is picked from a list of choices, which a policy can narrow.
 */
export const hasChoices = (definition: FlowVariableDefinition): boolean =>
  definition.type === 'SELECT' || definition.type === 'CONNECTION_SELECT';

/**
 * Reads a rule into the form the Share screen edits. A variable no rule names is one the units may not set.
 */
export const ruleToForm = (rule: OverlayRule | undefined): VariableRuleForm => ({
  allowedValues: [...(rule?.allowedValues ?? [])],
  editable: rule?.editable === true,
  excludedValues: [...(rule?.excludedValues ?? [])],
  value: rule?.value?.[0] ?? '',
});

/**
 * Writes the form as a rule. A variable the units may not set and that has no value of its own to hold them to needs
 * no rule, because a variable no policy names already stays at the flow's default.
 */
export const formToRule = (form: VariableRuleForm): OverlayRule | undefined => {
  if (!form.editable) {
    return form.value === '' ? undefined : {editable: false, value: [form.value]};
  }

  const rule: OverlayRule = {editable: true};

  if (form.value !== '') rule.value = [form.value];
  if (form.allowedValues.length > 0) rule.allowedValues = [...form.allowedValues];
  if (form.excludedValues.length > 0) rule.excludedValues = [...form.excludedValues];

  return rule;
};

/**
 * Returns the rules with one variable's rule set from a form, or left out when it needs none.
 */
export const withVariableRule = (
  rules: OverlayRules | undefined,
  identifier: string,
  form: VariableRuleForm,
): OverlayRules => {
  const next: OverlayRules = {...rules};
  const rule: OverlayRule | undefined = formToRule(form);

  if (rule) {
    next[ruleKeyOf(identifier)] = rule;
  } else {
    delete next[ruleKeyOf(identifier)];
  }

  return next;
};
