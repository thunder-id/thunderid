// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {validateFlowVariableValue} from '@thunderid/react';
import type {FlowVariableDefinition, FlowVariableType, FlowVariableValue} from '@thunderid/react';

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
 * The connection types a Connection variable can be limited to, with the label each shows in the editor.
 */
export const CONNECTION_TYPES: {label: string; value: string}[] = [
  {label: 'Google', value: 'google'},
  {label: 'GitHub', value: 'github'},
  {label: 'OpenID Connect', value: 'oidc'},
  {label: 'OAuth 2', value: 'oauth'},
];

const IDENTIFIER_PATTERN = /^[A-Za-z][A-Za-z0-9_]*$/;

/**
 * Tells whether a name can be used as a variable identifier, which a `{{var(name)}}` placeholder refers to.
 */
export const isValidIdentifier = (identifier: string): boolean => IDENTIFIER_PATTERN.test(identifier);

/**
 * The default a variable of a type starts with, so a new variable is valid before anything is filled in.
 */
export const defaultValueFor = (type: FlowVariableType): FlowVariableValue => {
  switch (type) {
    case 'NUMBER_INPUT':
      return 0;
    case 'BOOLEAN_INPUT':
      return false;
    case 'EMAIL_INPUT':
      return 'name@example.com';
    case 'PHONE_INPUT':
      return '+10000000000';
    case 'DATE_INPUT':
      return '2026-01-01';
    default:
      return '';
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
