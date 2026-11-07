// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import {formToRule, hasChoices, ruleKeyOf, ruleToForm, withVariableRule} from '../variableRules';

describe('variableRules', () => {
  it('names a variable rule by the variable identifier', () => {
    expect(ruleKeyOf('connection1')).toBe('variables.connection1');
  });

  it('tells the variables that are picked from choices', () => {
    expect(hasChoices({identifier: 'a', type: 'SELECT'})).toBe(true);
    expect(hasChoices({identifier: 'a', type: 'CONNECTION_SELECT'})).toBe(true);
    expect(hasChoices({identifier: 'a', type: 'TEXT_INPUT'})).toBe(false);
  });

  it('reads a variable no rule names as one the units may not set', () => {
    expect(ruleToForm(undefined)).toEqual({allowedValues: [], editable: false, excludedValues: [], value: ''});
  });

  it('reads every field of a rule', () => {
    expect(ruleToForm({allowedValues: ['a', 'b'], editable: true, excludedValues: ['b'], value: ['a']})).toEqual({
      allowedValues: ['a', 'b'],
      editable: true,
      excludedValues: ['b'],
      value: 'a',
    });
  });

  it('writes a rule for an editable variable with only what was set', () => {
    expect(formToRule({allowedValues: [], editable: true, excludedValues: [], value: ''})).toEqual({editable: true});
    expect(formToRule({allowedValues: ['a'], editable: true, excludedValues: ['b'], value: 'a'})).toEqual({
      allowedValues: ['a'],
      editable: true,
      excludedValues: ['b'],
      value: ['a'],
    });
  });

  it('holds a variable the units may not set to a value, or leaves it at the default with no rule', () => {
    expect(formToRule({allowedValues: ['a'], editable: false, excludedValues: [], value: 'x'})).toEqual({
      editable: false,
      value: ['x'],
    });
    expect(formToRule({allowedValues: [], editable: false, excludedValues: [], value: ''})).toBeUndefined();
  });

  it('sets and removes the rule of one variable without touching the others', () => {
    const rules = {'variables.other': {editable: true}};
    const withRule = withVariableRule(rules, 'connection1', {
      allowedValues: [],
      editable: true,
      excludedValues: [],
      value: '',
    });

    expect(withRule).toEqual({'variables.connection1': {editable: true}, 'variables.other': {editable: true}});
    expect(
      withVariableRule(withRule, 'connection1', {allowedValues: [], editable: false, excludedValues: [], value: ''}),
    ).toEqual(rules);
    expect(rules).toEqual({'variables.other': {editable: true}});
  });
});
