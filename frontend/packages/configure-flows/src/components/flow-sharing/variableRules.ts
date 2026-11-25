// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {OverlayRule, OverlayRules, SharingPolicy} from '@thunderid/configure-resource-sharing';
import {validateFlowVariableValue} from '@thunderid/react';
import type {FlowVariableDefinition} from '@thunderid/react';
import type {Node} from '@xyflow/react';
import {StaticStepTypes} from '../../models/steps';

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

const RULE_PREFIX = 'variables.';

/**
 * The key a variable's rule goes under in a sharing policy.
 */
export const ruleKeyOf = (identifier: string): string => `${RULE_PREFIX}${identifier}`;

/**
 * Tells whether a sharing rule is one of a flow's variables, which the Share screen edits.
 */
export const editsVariableRule = (field: string): boolean => field.startsWith(RULE_PREFIX);

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

/**
 * The identifiers of the variables a set of sharing policies states a rule for. Saving the flow without one of them
 * deletes its rule and the values units saved for it.
 *
 * @param policies - The sharing policies of a flow
 * @returns The identifiers the policies name
 */
export const sharedVariableIdentifiers = (policies: SharingPolicy[] | undefined): Set<string> => {
  const identifiers = new Set<string>();
  const collect = (rules: OverlayRules | undefined): void => {
    Object.keys(rules ?? {}).forEach((field: string) => {
      if (editsVariableRule(field)) identifiers.add(field.slice(RULE_PREFIX.length));
    });
  };

  (policies ?? []).forEach((policy: SharingPolicy) => {
    collect(policy.allUnits?.overlayRules);
    [policy.childUnits, policy.otherTreeRoots].forEach((section) => {
      if (section?.mode === 'all') collect(section.overlayRules);
      if (section?.mode === 'selected') section.units.forEach((unit) => collect(unit.overlayRules));
    });
  });

  return identifiers;
};

/**
 * Tells whether a value a unit starts with breaks the rules of the variable, as the server would refuse it. A blank
 * value leaves the flow's own default in place, and a choice is one the variable already offers, so neither is checked.
 *
 * @param definition - The variable
 * @param raw - What was typed for the unit's default
 * @returns Whether the value is not valid for the variable
 */
export const isInvalidUnitDefault = (definition: FlowVariableDefinition, raw: string): boolean => {
  if (raw === '' || hasChoices(definition) || definition.type === 'BOOLEAN_INPUT') return false;

  const value: string | number = definition.type === 'NUMBER_INPUT' && !Number.isNaN(Number(raw)) ? Number(raw) : raw;

  return Boolean(validateFlowVariableValue(definition, value));
};

/**
 * The variables the variables nodes of a set of canvas nodes declare, in the order the nodes are given.
 *
 * @param nodes - The canvas nodes of the flow
 * @returns The declared variables
 */
export const declaredVariablesOf = (nodes: Node[]): FlowVariableDefinition[] =>
  nodes
    .filter((node: Node) => node.type === StaticStepTypes.Variables)
    .flatMap((node: Node) => {
      const inputs: unknown = (node.data as {inputs?: unknown} | undefined)?.inputs;

      return Array.isArray(inputs) ? (inputs as FlowVariableDefinition[]) : [];
    });

/**
 * Tells whether a sharing policy states a rule for a variable that a set of canvas nodes no longer declares. Saving
 * the flow takes such a rule off the server, so a draft built on that policy is out of date.
 *
 * @param policy - The flow's saved sharing policy, or `undefined` when it has none
 * @param nodes - The canvas nodes the flow was saved from
 * @returns Whether the policy names a variable the nodes do not declare
 */
export const hasTermsForRemovedVariable = (policy: SharingPolicy | undefined, nodes: Node[]): boolean => {
  const declared = new Set(declaredVariablesOf(nodes).map((variable: FlowVariableDefinition) => variable.identifier));

  return [...sharedVariableIdentifiers(policy ? [policy] : undefined)].some(
    (identifier: string) => !declared.has(identifier),
  );
};

/**
 * Tells whether a value can be the unit's default under a rule the units may edit: inside the choices they may pick
 * from, when those are limited, and not among the ones taken away. The server refuses a default outside them.
 *
 * @param form - The rule being edited
 * @param value - The default to check
 * @returns Whether the rule allows the default
 */
export const isAllowedDefault = (form: VariableRuleForm, value: string): boolean =>
  (form.allowedValues.length === 0 || form.allowedValues.includes(value)) && !form.excludedValues.includes(value);

/**
 * Tells whether any default a draft gives a unit breaks the rules of its variable, as the server would refuse it. It
 * looks at the terms of all child units and at the terms of each unit named on its own.
 *
 * @param draft - The terms of all child units and of each unit named on its own
 * @param variables - The variables the flow declares
 * @returns Whether some default is not valid for its variable
 */
export const hasInvalidUnitDefaults = (
  draft: {childUnitRules?: OverlayRules; childUnits: {rules?: OverlayRules}[]},
  variables: FlowVariableDefinition[],
): boolean =>
  [draft.childUnitRules, ...draft.childUnits.map((unit) => unit.rules)].some((rules: OverlayRules | undefined) =>
    Object.entries(rules ?? {}).some(([field, rule]) => {
      const definition = variables.find((variable: FlowVariableDefinition) => ruleKeyOf(variable.identifier) === field);
      const value: string | undefined = rule.value?.[0];

      return definition !== undefined && value !== undefined && isInvalidUnitDefault(definition, value);
    }),
  );
