/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {describe, it, expect} from 'vitest';
import type {PropertyDefinition, UserTypeDefinition} from '../../models/users';
import {attributeConformsToSchema, dropNonConformingOptionalAttributes} from '../dropNonConformingAttributes';

describe('attributeConformsToSchema', () => {
  it('checks primitive types', () => {
    expect(attributeConformsToSchema('hi', {type: 'string'})).toBe(true);
    expect(attributeConformsToSchema(5, {type: 'string'})).toBe(false);
    expect(attributeConformsToSchema(5, {type: 'number'})).toBe(true);
    expect(attributeConformsToSchema('5', {type: 'number'})).toBe(false);
    expect(attributeConformsToSchema(true, {type: 'boolean'})).toBe(true);
    expect(attributeConformsToSchema('true', {type: 'boolean'})).toBe(false);
    expect(attributeConformsToSchema([1], {type: 'array', items: {type: 'number'}})).toBe(true);
    expect(attributeConformsToSchema('x', {type: 'array', items: {type: 'number'}})).toBe(false);
    expect(attributeConformsToSchema({a: 1}, {type: 'object', properties: {}})).toBe(true);
    expect(attributeConformsToSchema([1], {type: 'object', properties: {}})).toBe(false);
  });

  it('checks enum membership', () => {
    const def: PropertyDefinition = {type: 'string', enum: ['ACTIVE', 'INACTIVE']};
    expect(attributeConformsToSchema('ACTIVE', def)).toBe(true);
    expect(attributeConformsToSchema('PENDING', def)).toBe(false);
  });

  it('checks regex and tolerates an unparseable pattern', () => {
    expect(attributeConformsToSchema('abc', {type: 'string', regex: '^[a-z]+$'})).toBe(true);
    expect(attributeConformsToSchema('ABC', {type: 'string', regex: '^[a-z]+$'})).toBe(false);
    // An invalid schema regex can't judge the value, so it is not dropped.
    expect(attributeConformsToSchema('abc', {type: 'string', regex: '('})).toBe(true);
  });
});

describe('dropNonConformingOptionalAttributes', () => {
  const schema: UserTypeDefinition = {
    age: {type: 'number'},
    nickname: {type: 'string'},
    email: {type: 'string', required: true},
  };

  it('drops an optional attribute whose value no longer matches the schema', () => {
    const result = dropNonConformingOptionalAttributes({age: 'not-a-number', nickname: 'jo'}, schema);
    expect(result).toEqual({nickname: 'jo'});
  });

  it('keeps a required attribute even when its value no longer matches', () => {
    const result = dropNonConformingOptionalAttributes({email: 12345}, schema);
    expect(result).toEqual({email: 12345});
  });

  it('keeps conforming values', () => {
    const result = dropNonConformingOptionalAttributes({age: 30, nickname: 'jo'}, schema);
    expect(result).toEqual({age: 30, nickname: 'jo'});
  });

  it('leaves undeclared keys untouched (backend strips those)', () => {
    const result = dropNonConformingOptionalAttributes({stale: 'x', age: 30}, schema);
    expect(result).toEqual({stale: 'x', age: 30});
  });

  it('returns the input unchanged when no schema is available', () => {
    const attrs = {age: 'not-a-number'};
    expect(dropNonConformingOptionalAttributes(attrs, undefined)).toBe(attrs);
  });
});
