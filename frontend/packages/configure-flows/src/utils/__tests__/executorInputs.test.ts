// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import type {Resource} from '../../models/resources';
import type {FlowNodeInput} from '../../models/responses';
import {getExecutorInputs, hasInvalidInputConfig, toBooleanDefault} from '../executorInputs';

describe('executorInputs', () => {
  describe('getExecutorInputs', () => {
    const stepWithInputs = (inputs: unknown): Resource =>
      ({id: 'step-1', data: {action: {executor: {name: 'TestExecutor', inputs}}}}) as unknown as Resource;

    it('should return the inputs configured on the executor', () => {
      const inputs = [{type: 'TEXT_INPUT', identifier: 'locale', required: false}];

      expect(getExecutorInputs(stepWithInputs(inputs))).toEqual(inputs);
    });

    it.each([
      ['no inputs', stepWithInputs(undefined)],
      ['inputs that are not a list', stepWithInputs('invalid')],
      ['no executor', {id: 'step-1', data: {}} as unknown as Resource],
      ['no step data', {id: 'element-1'} as unknown as Resource],
    ])('should return none for %s', (_label, resource) => {
      expect(getExecutorInputs(resource)).toEqual([]);
    });
  });

  describe('hasInvalidInputConfig', () => {
    const input = (extra: Partial<FlowNodeInput>): FlowNodeInput => ({
      type: 'TEXT_INPUT',
      identifier: 'locale',
      required: false,
      ...extra,
    });

    it.each([
      ['a constant with no default', input({constant: true})],
      ['a constant with an empty default', input({constant: true, default: ''})],
    ])('should flag %s', (_label, value) => {
      expect(hasInvalidInputConfig(value)).toBe(true);
    });

    it.each([
      ['a constant with a default', input({constant: true, default: 'en-US'})],
      ['a default that is not constant', input({default: 'en-US'})],
      ['a constant flag set to false', input({constant: false})],
      ['neither setting', input({})],
    ])('should accept %s', (_label, value) => {
      expect(hasInvalidInputConfig(value)).toBe(false);
    });
  });

  describe('toBooleanDefault', () => {
    it.each(['true', 'True', 'TRUE', 't', 'T', '1'])('should read %s as true', (value) => {
      expect(toBooleanDefault(value)).toBe('true');
    });

    it.each(['false', 'False', 'FALSE', 'f', 'F', '0'])('should read %s as false', (value) => {
      expect(toBooleanDefault(value)).toBe('false');
    });

    it.each([undefined, '', 'maybe', 'yes', 'tRuE'])('should return an empty string for %s', (value) => {
      expect(toBooleanDefault(value)).toBe('');
    });
  });
});
