// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {CONNECTION_VENDOR_META, ConnectionTypes} from '@thunderid/configure-connections';
import {validateFlowVariableValue} from '@thunderid/react';
import type {FlowVariableDefinition, FlowVariableType, FlowVariableValue} from '@thunderid/react';
import {IDENTIFIER_PATTERN} from './variableIdentifier';

/**
 * The input types a variable can have, with the label each shows in the editor.
 */
export const VARIABLE_TYPES: {label: string; value: FlowVariableType}[] = [
  {label: 'Text Input', value: 'TEXT_INPUT'},
  {label: 'Email Input', value: 'EMAIL_INPUT'},
  {label: 'Phone Input', value: 'PHONE_INPUT'},
  {label: 'Number Input', value: 'NUMBER_INPUT'},
  {label: 'Date Input', value: 'DATE_INPUT'},
  {label: 'Checkbox', value: 'BOOLEAN_INPUT'},
  {label: 'Select', value: 'SELECT'},
  {label: 'Connection', value: 'CONNECTION_SELECT'},
];

/**
 * The connection types an identity provider can be, which is the one category a Connection variable can name.
 */
const IDENTITY_PROVIDER_TYPES: string[] = [
  ConnectionTypes.GOOGLE,
  ConnectionTypes.GITHUB,
  ConnectionTypes.OIDC,
  ConnectionTypes.OAUTH,
];

/**
 * The connection types a Connection variable can be limited to, with the name the connections catalog gives each.
 *
 * @returns The types, in the catalog's order.
 */
export const getConnectionTypes = (): {label: string; value: string}[] =>
  CONNECTION_VENDOR_META.flatMap((vendor) =>
    vendor.backendType && IDENTITY_PROVIDER_TYPES.includes(vendor.backendType)
      ? [{label: vendor.displayName, value: vendor.backendType}]
      : [],
  );

/**
 * Tells whether a name can be used as a variable identifier, which a `{{var(name)}}` placeholder refers to.
 */
export const isValidIdentifier = (identifier: string): boolean => IDENTIFIER_PATTERN.test(identifier);

/**
 * The default a variable of a type starts with. A number and a checkbox start on a neutral 0 and false. An email, a phone
 * number and a date start blank, which the editor flags until the author enters a real one, so no made-up value is ever
 * inherited by the units the flow is shared with. See {@link defaultPlaceholderFor} for the hint shown meanwhile.
 */
export const defaultValueFor = (type: FlowVariableType): FlowVariableValue => {
  switch (type) {
    case 'NUMBER_INPUT':
      return 0;
    case 'BOOLEAN_INPUT':
      return false;
    default:
      return '';
  }
};

/**
 * An example of what the default of a type looks like, shown as a placeholder and never stored.
 */
export const defaultPlaceholderFor = (type: FlowVariableType): string | undefined => {
  switch (type) {
    case 'EMAIL_INPUT':
      return 'name@example.com';
    case 'PHONE_INPUT':
      return '+10000000000';
    default:
      return undefined;
  }
};

/**
 * Creates a variable with an identifier no other variable has.
 */
export const createVariable = (existing: FlowVariableDefinition[]): FlowVariableDefinition => {
  const taken = new Set<string>(existing.map(({identifier}: FlowVariableDefinition) => identifier));
  let index = existing.length + 1;

  while (taken.has(`variable${index}`)) {
    index += 1;
  }

  return {
    defaultValue: '',
    identifier: `variable${index}`,
    label: `Variable ${index}`,
    type: 'TEXT_INPUT',
  };
};

/**
 * Turns the options typed one per line into the values a choice takes. All booleans make it a boolean, all numbers
 * a number, anything else a string, as the server reads them.
 */
export const parseOptions = (text: string): FlowVariableValue[] => {
  const lines: string[] = text
    .split('\n')
    .map((line: string) => line.trim())
    .filter(Boolean);

  if (lines.length > 0 && lines.every((line: string) => line === 'true' || line === 'false')) {
    return lines.map((line: string) => line === 'true');
  }

  if (lines.length > 0 && lines.every((line: string) => !Number.isNaN(Number(line)))) {
    return lines.map(Number);
  }

  return lines;
};

/**
 * Turns the options of a choice back into one per line.
 */
export const formatOptions = (options: FlowVariableValue[] | undefined): string => (options ?? []).join('\n');

/**
 * Finds what is wrong with a variable as declared, so the editor can say so before the flow is saved.
 *
 * @param definitions - Every variable of the flow.
 * @param index - The variable to check.
 * @returns The problems, none when the variable is fine.
 */
export const findProblems = (
  definitions: FlowVariableDefinition[],
  index: number,
): ('default' | 'duplicate' | 'identifier' | 'options')[] => {
  const definition: FlowVariableDefinition = definitions[index];
  const problems: ('default' | 'duplicate' | 'identifier' | 'options')[] = [];

  if (!isValidIdentifier(definition.identifier)) {
    problems.push('identifier');
  } else if (
    definitions.some(
      (other: FlowVariableDefinition, otherIndex: number) =>
        otherIndex !== index && other.identifier === definition.identifier,
    )
  ) {
    problems.push('duplicate');
  }

  if (definition.type === 'SELECT' && (definition.options ?? []).length === 0) {
    problems.push('options');
  }

  // A text variable may start blank, which the server accepts unless a rule is there for a blank to break. A connection
  // variable may start with no connection too, and is left to the server to judge once it names one.
  const isBlankText: boolean =
    definition.type === 'TEXT_INPUT' &&
    definition.defaultValue === '' &&
    (definition.validation ?? []).every((rule) => rule.type === 'maxLength');

  if (
    definition.type !== 'CONNECTION_SELECT' &&
    !isBlankText &&
    validateFlowVariableValue(definition, definition.defaultValue)
  ) {
    problems.push('default');
  }

  return problems;
};

/**
 * The HTML input type a text-like variable is typed into, which the editor and the Share screen both use so a variable
 * is entered the same way in either. A variable not listed is typed as text.
 */
export const INPUT_TYPES: Partial<Record<FlowVariableDefinition['type'], string>> = {
  DATE_INPUT: 'date',
  EMAIL_INPUT: 'email',
  NUMBER_INPUT: 'number',
  PHONE_INPUT: 'tel',
};
