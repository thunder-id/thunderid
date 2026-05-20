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
import type {Schema, SchemaAttribute} from '../../models/scim2-schema';
import flattenUserSchema from '../flattenUserSchema';

const baseAttr = (overrides: Partial<SchemaAttribute> = {}): SchemaAttribute => ({
  caseExact: false,
  multiValued: false,
  mutability: 'readWrite',
  name: 'attr',
  returned: 'default',
  type: 'string',
  uniqueness: 'none',
  ...overrides,
});

const baseSchema = (overrides: Partial<Schema> = {}): Schema => ({
  attributes: [],
  description: 'User schema',
  id: 'urn:ietf:params:scim:schemas:core:2.0:User',
  name: 'User',
  ...overrides,
});

describe('flattenUserSchema', () => {
  it('should return empty array when input is empty', () => {
    expect(flattenUserSchema([])).toEqual([]);
  });

  it('should ignore schemas with missing/undefined attributes', () => {
    const schema: Schema = baseSchema({attributes: undefined as any});
    expect(flattenUserSchema([schema])).toEqual([]);
  });

  it('should flatten simple (non-complex) top-level attributes directly', () => {
    const schema: Schema = baseSchema({
      attributes: [baseAttr({name: 'userName'}), baseAttr({name: 'active', type: 'boolean'})],
    });

    const out: ReturnType<typeof flattenUserSchema> = flattenUserSchema([schema]);

    expect(out).toEqual([
      expect.objectContaining({name: 'userName', schemaId: schema.id}),
      expect.objectContaining({name: 'active', schemaId: schema.id}),
    ]);
    // Ensure other props are preserved
    expect(out[0]).toMatchObject({multiValued: false, type: 'string'});
    expect(out[1]).toMatchObject({multiValued: false, type: 'boolean'});
  });

  it('should flatten complex attributes into dot-notation (includes only sub-attributes, not the parent)', () => {
    const schema: Schema = baseSchema({
      attributes: [
        baseAttr({
          name: 'name',
          subAttributes: [baseAttr({name: 'givenName'}), baseAttr({name: 'familyName'})],
          type: 'complex',
        }),
      ],
    });

    const out: ReturnType<typeof flattenUserSchema> = flattenUserSchema([schema]);
    expect(out).toEqual([
      expect.objectContaining({name: 'name.givenName', schemaId: schema.id}),
      expect.objectContaining({name: 'name.familyName', schemaId: schema.id}),
    ]);

    const names: string[] = out.map((a: {name: string}) => a.name);
    expect(names).not.toContain('name');
  });

  it('should drop complex attributes with an empty subAttributes array (no parent emitted)', () => {
    const schema: Schema = baseSchema({
      attributes: [
        baseAttr({
          name: 'address',
          subAttributes: [], // empty — nothing should be emitted
          type: 'complex',
        }),
      ],
    });

    expect(flattenUserSchema([schema])).toEqual([]);
  });

  it('should handle deeper nesting by only including leaf sub-attributes (one level processed)', () => {
    const schema: Schema = baseSchema({
      attributes: [
        baseAttr({
          name: 'profile',
          subAttributes: [
            baseAttr({
              name: 'contact',
              subAttributes: [baseAttr({name: 'email'}), baseAttr({name: 'phone', type: 'string'})],
              type: 'complex',
            }),
            baseAttr({name: 'nickname'}),
          ],
          type: 'complex',
        }),
      ],
    });

    const out: ReturnType<typeof flattenUserSchema> = flattenUserSchema([schema]);
    expect(out.map((a: {name: string}) => a.name)).toEqual(['profile.contact', 'profile.nickname']);
    out.forEach((a: {schemaId?: string}) => expect(a.schemaId).toBe(schema.id));
  });

  it('should support multiple schemas and tags each flattened attribute with the correct schemaId', () => {
    const userSchema: Schema = baseSchema({
      attributes: [
        baseAttr({name: 'userName'}),
        baseAttr({
          name: 'name',
          subAttributes: [baseAttr({name: 'givenName'})],
          type: 'complex',
        }),
      ],
      id: 'urn:user',
    });

    const groupSchema: Schema = baseSchema({
      attributes: [
        baseAttr({name: 'displayName'}),
        baseAttr({
          name: 'owner',
          subAttributes: [baseAttr({name: 'value'})],
          type: 'complex',
        }),
      ],
      description: 'Group schema',
      id: 'urn:group',
      name: 'Group',
    });

    const out: ReturnType<typeof flattenUserSchema> = flattenUserSchema([userSchema, groupSchema]);

    expect(out).toEqual([
      expect.objectContaining({name: 'userName', schemaId: 'urn:user'}),
      expect.objectContaining({name: 'name.givenName', schemaId: 'urn:user'}),
      expect.objectContaining({name: 'displayName', schemaId: 'urn:group'}),
      expect.objectContaining({name: 'owner.value', schemaId: 'urn:group'}),
    ]);
  });
});
