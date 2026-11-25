// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {Node} from '@xyflow/react';
import {describe, it, expect} from 'vitest';
import {
  editsVariableRule,
  formToRule,
  declaredVariablesOf,
  hasInvalidUnitDefaults,
  hasTermsForRemovedVariable,
  isAllowedDefault,
  hasChoices,
  isInvalidUnitDefault,
  ruleKeyOf,
  ruleToForm,
  sharedVariableIdentifiers,
  withVariableRule,
} from '../variableRules';

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

  it('tells the rules of a flow variable from others', () => {
    expect(editsVariableRule('variables.region')).toBe(true);
    expect(editsVariableRule('roles')).toBe(false);
  });

  it('collects the variables a set of policies names, whichever section holds the rule', () => {
    const rule = {editable: true};

    expect(
      sharedVariableIdentifiers([
        {
          id: 'p1',
          stage: 'share',
          version: 1,
          allUnits: {overlayRules: {'variables.a': rule}},
          childUnits: {mode: 'selected', units: [{ouId: 'retail', reach: 'unit', overlayRules: {'variables.b': rule}}]},
          otherTreeRoots: {mode: 'all', overlayRules: {'variables.c': rule, assignments: {editable: false}}},
        },
      ]),
    ).toEqual(new Set(['a', 'b', 'c']));
    expect(sharedVariableIdentifiers(undefined)).toEqual(new Set());
  });

  describe('isInvalidUnitDefault', () => {
    it('refuses a value that breaks the rules of the variable', () => {
      expect(isInvalidUnitDefault({identifier: 'a', type: 'EMAIL_INPUT'}, 'not-an-email')).toBe(true);
      expect(
        isInvalidUnitDefault({identifier: 'a', type: 'TEXT_INPUT', validation: [{type: 'regex', value: '^a'}]}, 'beta'),
      ).toBe(true);
      expect(isInvalidUnitDefault({identifier: 'a', type: 'NUMBER_INPUT'}, 'abc')).toBe(true);
    });

    it('accepts a value that fits', () => {
      expect(isInvalidUnitDefault({identifier: 'a', type: 'EMAIL_INPUT'}, 'ops@acme.io')).toBe(false);
      expect(
        isInvalidUnitDefault(
          {identifier: 'a', type: 'TEXT_INPUT', validation: [{type: 'regex', value: '^a'}]},
          'alpha',
        ),
      ).toBe(false);
      expect(isInvalidUnitDefault({identifier: 'a', type: 'NUMBER_INPUT'}, '12')).toBe(false);
    });

    it('leaves a blank value, a choice and a switch alone', () => {
      expect(isInvalidUnitDefault({identifier: 'a', type: 'EMAIL_INPUT'}, '')).toBe(false);
      expect(isInvalidUnitDefault({identifier: 'a', type: 'SELECT', options: ['x']}, 'y')).toBe(false);
      expect(isInvalidUnitDefault({identifier: 'a', type: 'BOOLEAN_INPUT'}, 'true')).toBe(false);
    });
  });

  describe('hasTermsForRemovedVariable', () => {
    const policy = {
      id: 'p1',
      stage: 'share' as const,
      version: 1,
      childUnits: {
        mode: 'all' as const,
        overlayRules: {'variables.a': {editable: true}, 'variables.b': {editable: false}},
      },
    };
    const variablesNode = (identifiers: string[]) =>
      ({
        id: 'vars',
        type: 'VARIABLES',
        position: {x: 0, y: 0},
        data: {inputs: identifiers.map((identifier) => ({identifier, type: 'TEXT_INPUT'}))},
      }) as Node;

    it('is false while every variable the policy names is still declared', () => {
      expect(hasTermsForRemovedVariable(policy, [variablesNode(['a', 'b', 'c'])])).toBe(false);
    });

    it('is true once a variable the policy names is no longer declared', () => {
      expect(hasTermsForRemovedVariable(policy, [variablesNode(['a'])])).toBe(true);
      expect(hasTermsForRemovedVariable(policy, [])).toBe(true);
    });

    it('is false when there is no policy, and reads the variables of every variables node', () => {
      expect(hasTermsForRemovedVariable(undefined, [])).toBe(false);
      expect(hasTermsForRemovedVariable(policy, [variablesNode(['a']), variablesNode(['b'])])).toBe(false);
    });
  });

  describe('isAllowedDefault', () => {
    const form = {allowedValues: [] as string[], editable: true, excludedValues: [] as string[], value: ''};

    it('accepts any default while the choices are not limited', () => {
      expect(isAllowedDefault(form, 'x')).toBe(true);
    });

    it('accepts only a default inside the choices the units may pick from', () => {
      expect(isAllowedDefault({...form, allowedValues: ['a', 'b']}, 'a')).toBe(true);
      expect(isAllowedDefault({...form, allowedValues: ['a', 'b']}, 'c')).toBe(false);
    });

    it('refuses a default among the choices taken away', () => {
      expect(isAllowedDefault({...form, excludedValues: ['b']}, 'b')).toBe(false);
      expect(isAllowedDefault({...form, allowedValues: ['a', 'b'], excludedValues: ['b']}, 'a')).toBe(true);
    });
  });

  describe('declaredVariablesOf', () => {
    it('collects the variables of every variables node and skips other nodes', () => {
      const a = {identifier: 'a', type: 'TEXT_INPUT' as const};
      const b = {identifier: 'b', type: 'TEXT_INPUT' as const};
      const nodes = [
        {id: 'v1', type: 'VARIABLES', position: {x: 0, y: 0}, data: {inputs: [a]}},
        {id: 'step', type: 'TASK_EXECUTION', position: {x: 0, y: 0}, data: {inputs: [{identifier: 'x'}]}},
        {id: 'v2', type: 'VARIABLES', position: {x: 0, y: 0}, data: {}},
        {id: 'v3', type: 'VARIABLES', position: {x: 0, y: 0}, data: {inputs: [b]}},
      ] as Node[];

      expect(declaredVariablesOf(nodes)).toEqual([a, b]);
    });
  });

  describe('hasInvalidUnitDefaults', () => {
    const email = {identifier: 'supportEmail', type: 'EMAIL_INPUT' as const};
    const rule = (value?: string[]) => ({editable: true, ...(value ? {value} : {})});

    it('is false when every default fits its variable, or there is none', () => {
      expect(
        hasInvalidUnitDefaults({childUnitRules: {'variables.supportEmail': rule(['ops@acme.io'])}, childUnits: []}, [
          email,
        ]),
      ).toBe(false);
      expect(
        hasInvalidUnitDefaults({childUnitRules: {'variables.supportEmail': rule()}, childUnits: []}, [email]),
      ).toBe(false);
      expect(hasInvalidUnitDefaults({childUnits: []}, [email])).toBe(false);
    });

    it('is true for a bad default among the terms of all child units', () => {
      expect(
        hasInvalidUnitDefaults({childUnitRules: {'variables.supportEmail': rule(['not-an-email'])}, childUnits: []}, [
          email,
        ]),
      ).toBe(true);
    });

    it('is true for a bad default in the terms of one named unit', () => {
      expect(
        hasInvalidUnitDefaults(
          {
            childUnits: [{rules: undefined}, {rules: {'variables.supportEmail': rule(['not-an-email'])}}],
          },
          [email],
        ),
      ).toBe(true);
    });

    it('leaves a rule for a variable the flow does not declare to the server', () => {
      expect(
        hasInvalidUnitDefaults({childUnitRules: {'variables.gone': rule(['whatever'])}, childUnits: []}, [email]),
      ).toBe(false);
    });
  });
});
