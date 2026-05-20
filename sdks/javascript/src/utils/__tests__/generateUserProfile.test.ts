/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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
import generateUserProfile from '../generateUserProfile';

describe('generateUserProfile', () => {
  it('should extract simple fields present in the ME response', () => {
    const me: Record<string, unknown> = {country: 'US', userName: 'john.doe'};
    const schemas: Record<string, unknown>[] = [
      {multiValued: false, name: 'userName', type: 'STRING'},
      {multiValued: false, name: 'country', type: 'STRING'},
    ];

    const out: Record<string, unknown> = generateUserProfile(me, schemas);

    expect(out['userName']).toBe('john.doe');
    expect(out['country']).toBe('US');
  });

  it('should support dotted paths using get() and sets nested keys using set()', () => {
    const me: Record<string, unknown> = {name: {familyName: 'Doe', givenName: 'John'}};
    const schemas: Record<string, unknown>[] = [
      {multiValued: false, name: 'name.givenName', type: 'STRING'},
      {multiValued: false, name: 'name.familyName', type: 'STRING'},
    ];

    const out: Record<string, unknown> = generateUserProfile(me, schemas);

    expect(out['name'].givenName).toBe('John');
    expect(out['name'].familyName).toBe('Doe');
  });

  it('should wrap a single value into an array for multiValued attributes', () => {
    const me: Record<string, unknown> = {emails: 'john@example.com'};
    const schemas: Record<string, unknown>[] = [{multiValued: true, name: 'emails', type: 'STRING'}];

    const out: Record<string, unknown> = generateUserProfile(me, schemas);

    expect(out['emails']).toEqual(['john@example.com']);
  });

  it('should preserve arrays for multiValued attributes', () => {
    const me: Record<string, unknown> = {emails: ['a@x.com', 'b@x.com']};
    const schemas: Record<string, unknown>[] = [{multiValued: true, name: 'emails', type: 'STRING'}];

    const out: Record<string, unknown> = generateUserProfile(me, schemas);

    expect(out['emails']).toEqual(['a@x.com', 'b@x.com']);
  });

  it('should default missing STRING (non-multiValued) to empty string', () => {
    const me: Record<string, unknown> = {};
    const schemas: Record<string, unknown>[] = [{multiValued: false, name: 'displayName', type: 'STRING'}];

    const out: Record<string, unknown> = generateUserProfile(me, schemas);

    expect(out['displayName']).toBe('');
  });

  it('should leave missing non-STRING (non-multiValued) as undefined', () => {
    const me: Record<string, unknown> = {};
    const schemas: Record<string, unknown>[] = [
      {multiValued: false, name: 'age', type: 'NUMBER'},
      {multiValued: false, name: 'isActive', type: 'BOOLEAN'},
    ];

    const out: Record<string, unknown> = generateUserProfile(me, schemas);

    expect(out).toHaveProperty('age');
    expect(out['age']).toBeUndefined();
    expect(out).toHaveProperty('isActive');
    expect(out['isActive']).toBeUndefined();
  });

  it('should leave missing multiValued attributes as undefined', () => {
    const me: Record<string, unknown> = {};
    const schemas: Record<string, unknown>[] = [{multiValued: true, name: 'groups', type: 'STRING'}];

    const out: Record<string, unknown> = generateUserProfile(me, schemas);

    expect(out).toHaveProperty('groups');
    expect(out['groups']).toBeUndefined();
  });

  it('should ignore schema entries without a name', () => {
    const me: Record<string, unknown> = {userName: 'john'};
    const schemas: Record<string, unknown>[] = [
      {multiValued: false, name: 'userName', type: 'STRING'},
      {multiValued: false, type: 'STRING'},
    ];

    const out: Record<string, unknown> = generateUserProfile(me, schemas);

    expect(out['userName']).toBe('john');
    expect(Object.keys(out).sort()).toEqual(['userName']);
  });

  it('should not mutate the source ME response', () => {
    const me: Record<string, unknown> = {emails: 'a@x.com', userName: 'john'};
    const snapshot: Record<string, unknown> = JSON.parse(JSON.stringify(me));
    const schemas: Record<string, unknown>[] = [
      {multiValued: false, name: 'userName', type: 'STRING'},
      {multiValued: true, name: 'emails', type: 'STRING'},
      {multiValued: false, name: 'missingStr', type: 'STRING'},
    ];

    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    const out: Record<string, unknown> = generateUserProfile(me, schemas);

    expect(me).toEqual(snapshot);
  });

  it('should preserve explicit null values (only undefined triggers defaults)', () => {
    const me: Record<string, unknown> = {nickname: null};
    const schemas: Record<string, unknown>[] = [{multiValued: false, name: 'nickname', type: 'STRING'}];

    const out: Record<string, unknown> = generateUserProfile(me, schemas);

    expect(out['nickname']).toBeNull();
  });

  it('should handle mixed present/missing values in one pass', () => {
    const me: Record<string, unknown> = {
      emails: ['a@x.com'],
      name: {givenName: 'John'},
      userName: 'john',
    };
    const schemas: Record<string, unknown>[] = [
      {multiValued: false, name: 'userName', type: 'STRING'},
      {multiValued: true, name: 'emails', type: 'STRING'},
      {multiValued: false, name: 'name.givenName', type: 'STRING'},
      {multiValued: false, name: 'name.middleName', type: 'STRING'},
      {multiValued: false, name: 'age', type: 'NUMBER'},
      {multiValued: true, name: 'groups', type: 'STRING'},
    ];

    const out: Record<string, unknown> = generateUserProfile(me, schemas);

    expect(out['userName']).toBe('john');
    expect(out['emails']).toEqual(['a@x.com']);
    expect(out?.['name']?.givenName).toBe('John');
    expect(out?.['name']?.middleName).toBe('');
    expect(out['age']).toBeUndefined();
    expect(out['groups']).toBeUndefined();
  });
});
