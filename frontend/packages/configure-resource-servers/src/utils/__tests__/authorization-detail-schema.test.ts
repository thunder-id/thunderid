// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import {flightBookingSchema, flightBookingTemplate} from './flightBookingSchema';
import type {SchemaField} from '../../models/authorization-detail-type';
import {
  checkConsentTemplate,
  effectiveModification,
  fieldsToSchema,
  isValidLimit,
  isValidProcessorEndpoint,
  isValidTypeValue,
  malformedPlaceholders,
  missingStandardFields,
  modificationsFor,
  newField,
  optionalPaths,
  pathsReferencing,
  previewConsentTemplate,
  sampleData,
  schemaKeywordIssues,
  renameTemplatePaths,
  sampleValue,
  schemaDefinitionText,
  schemaPaths,
  schemaToFields,
  templatePaths,
  usesProcessor,
  validateFields,
  valuePaths,
  valueTypeFor,
} from '../authorization-detail-schema';

const paymentSchema = {
  type: 'object',
  required: ['instructedAmount', 'creditorName'],
  properties: {
    actions: {
      type: 'array',
      items: {type: 'string'},
      title: 'Action',
      policy: {modification: 'removeItems', modifiableBy: ['client']},
    },
    instructedAmount: {
      type: 'object',
      required: ['amount', 'currency'],
      title: 'Amount',
      properties: {
        amount: {
          type: 'number',
          title: 'Amount',
          minimum: 0.01,
          maximum: 10000,
          policy: {
            modification: 'decrease',
            modifiableBy: ['processor', 'user', 'client'],
          },
        },
        currency: {type: 'string', title: 'Currency', maxLength: 3},
      },
    },
    creditorName: {type: 'string', title: 'Pay to'},
    debtorAccount: {
      type: 'object',
      properties: {iban: {type: 'string', title: 'From account', policy: {suppliedBy: 'processor'}}},
    },
    executeBy: {type: 'string', format: 'date', policy: {maximum: '2026-12-31'}},
    retries: {type: 'array', items: {type: 'number'}},
  },
};

function field(overrides: Partial<SchemaField>): SchemaField {
  return {...newField(), name: 'f', ...overrides};
}

describe('schemaToFields and fieldsToSchema', () => {
  it('round-trips a schema with every kind of field policy', () => {
    const fields = schemaToFields(paymentSchema);
    expect(fields).not.toBeNull();
    expect(fieldsToSchema(fields!)).toEqual(paymentSchema);
  });

  it('keeps a field named __proto__', () => {
    const schema: unknown = JSON.parse(
      '{"type":"object","properties":{"__proto__":{"type":"string"},"payee":{"type":"string"}}}',
    );
    expect(JSON.stringify(fieldsToSchema(schemaToFields(schema)!))).toBe(JSON.stringify(schema));
  });

  it('reads nested fields, required flags, list items, and policies', () => {
    const fields = schemaToFields(paymentSchema)!;
    const amount = fields.find((f) => f.name === 'instructedAmount')!.children.find((f) => f.name === 'amount')!;
    expect(amount.required).toBe(true);
    expect(amount.policy).toEqual({
      modification: 'decrease',
      modifiableBy: ['processor', 'user', 'client'],
      minimum: '0.01',
      maximum: '10000',
    });
    expect(fields.find((f) => f.name === 'instructedAmount')!.children[1].maxLength).toBe('3');
    expect(fields.find((f) => f.name === 'retries')!.itemKind).toBe('number');
  });

  it('reads and writes the allowed values of a list', () => {
    const schema = {
      type: 'object',
      properties: {
        actions: {
          type: 'array',
          items: {type: 'string', enum: ['initiate', 'status']},
          policy: {modification: 'removeItems', modifiableBy: ['user', 'client']},
        },
        codes: {type: 'array', items: {type: 'number', enum: [1, 2]}},
      },
    };
    const fields = schemaToFields(schema)!;
    expect(fields[0].allowedValues).toEqual(['initiate', 'status']);
    expect(fields[1].allowedValues).toEqual(['1', '2']);
    expect(fieldsToSchema(fields)).toEqual(schema);
    expect(
      schemaToFields({type: 'object', properties: {a: {type: 'array', items: {type: 'string'}}}})![0],
    ).not.toHaveProperty('allowedValues');
  });

  it('opens a list whose allowed values do not match its item type as JSON only', () => {
    expect(
      schemaToFields({type: 'object', properties: {a: {type: 'array', items: {type: 'number', enum: ['x']}}}}),
    ).toBeNull();
    expect(
      schemaToFields({type: 'object', properties: {a: {type: 'array', items: {type: 'string', enum: 'x'}}}}),
    ).toBeNull();
    expect(schemaToFields({type: 'object', properties: {a: {type: 'string', enum: ['x']}}})).toBeNull();
  });

  it('opens schemas that use other JSON Schema features as JSON only', () => {
    expect(schemaToFields({type: 'object', properties: {a: {type: 'string', pattern: '^x$'}}})).toBeNull();
    expect(schemaToFields({type: 'object', properties: {a: {type: 'string', policy: {other: 1}}}})).toBeNull();
    expect(schemaToFields({type: 'object', properties: {a: {type: 'array', items: {type: 'object'}}}})).toBeNull();
    expect(schemaToFields({type: 'object', properties: {a: {type: 'string', policy: {modification: 'x'}}}})).toBeNull();
    expect(schemaToFields({type: 'object', $schema: 'https://json-schema.org/draft/2020-12/schema'})).toBeNull();
    expect(
      schemaToFields({
        type: 'object',
        properties: {s: {type: 'string', policy: {modification: 'selectOne', modifiableBy: ['user']}}},
      }),
    ).toBeNull();
    expect(schemaToFields({type: 'array'})).toBeNull();
    expect(schemaToFields({type: 'object'})).toEqual([]);
  });

  it('writes no modification for a field the processor supplies', () => {
    const schema = fieldsToSchema([
      field({name: 'iban', policy: {suppliedBy: 'processor', modification: 'decrease', modifiableBy: ['client']}}),
    ]);
    expect(schema).toEqual({
      type: 'object',
      properties: {iban: {type: 'string', policy: {suppliedBy: 'processor'}}},
    });
  });

  it('writes limits without a value type, and drops limits a field cannot hold', () => {
    const schema = fieldsToSchema([
      field({name: 'n', kind: 'number', policy: {minimum: '1.50'}}),
      field({name: 'd', kind: 'dateTime', policy: {minimum: '2026-01-01T00:00:00Z'}}),
      field({name: 'b', kind: 'boolean', policy: {maximum: '3'}}),
    ]);
    expect(schema['properties']).toEqual({
      n: {type: 'number', minimum: 1.5},
      d: {type: 'string', format: 'date-time', policy: {minimum: '2026-01-01T00:00:00Z'}},
      b: {type: 'boolean'},
    });
  });

  it('limits a number with its own keywords, never its policy', () => {
    const schema = {
      type: 'object',
      properties: {
        n: {type: 'number', minimum: -5, maximum: 2.5, policy: {modification: 'increase', modifiableBy: ['user']}},
      },
    };
    const fields = schemaToFields(schema)!;
    expect(fields[0].policy).toEqual({modification: 'increase', modifiableBy: ['user'], minimum: '-5', maximum: '2.5'});
    expect(fieldsToSchema(fields)).toEqual(schema);
    expect(schemaToFields({type: 'object', properties: {n: {type: 'number', policy: {maximum: 3}}}})).toBeNull();
    expect(schemaToFields({type: 'object', properties: {n: {type: 'number', maximum: '3'}}})).toBeNull();
    expect(schemaToFields({type: 'object', properties: {d: {type: 'string', format: 'date', maximum: 3}}})).toBeNull();
  });

  it('reads and writes the maximum length of text, and only of text', () => {
    const schema = {type: 'object', properties: {note: {type: 'string', maxLength: 100}}};
    const fields = schemaToFields(schema)!;
    expect(fields[0].maxLength).toBe('100');
    expect(fieldsToSchema(fields)).toEqual(schema);
    expect(fieldsToSchema([field({maxLength: ' '})])['properties']).toEqual({f: {type: 'string'}});
    expect(fieldsToSchema([field({kind: 'number', maxLength: '5'})])['properties']).toEqual({f: {type: 'number'}});
    expect(schemaToFields({type: 'object', properties: {n: {type: 'number', maxLength: 5}}})).toBeNull();
    expect(
      schemaToFields({type: 'object', properties: {d: {type: 'string', format: 'date', maxLength: 5}}}),
    ).toBeNull();
    expect(schemaToFields({type: 'object', properties: {s: {type: 'string', maxLength: 1.5}}})).toBeNull();
  });

  it('reads and writes the fewest items a list holds, and only of a list', () => {
    const schema = {type: 'object', properties: {tags: {type: 'array', items: {type: 'string'}, minItems: 1}}};
    const fields = schemaToFields(schema)!;
    expect(fields[0].minItems).toBe('1');
    expect(fieldsToSchema(fields)).toEqual(schema);
    expect(fieldsToSchema([field({kind: 'array', minItems: ' '})])['properties']).toEqual({
      f: {type: 'array', items: {type: 'string'}},
    });
    expect(fieldsToSchema([field({minItems: '2'})])['properties']).toEqual({f: {type: 'string'}});
    expect(schemaToFields({type: 'object', properties: {s: {type: 'string', minItems: 1}}})).toBeNull();
    expect(
      schemaToFields({type: 'object', properties: {l: {type: 'array', items: {type: 'string'}, minItems: -1}}}),
    ).toBeNull();
  });

  it('opens the flight booking type, whose lists require an item, in the builder and writes it back', () => {
    const fields = schemaToFields(flightBookingSchema);
    expect(fields, 'the builder can show every keyword the type uses').not.toBeNull();
    expect(fields!.map((f) => f.name)).toEqual(['actions', 'flight', 'intent', 'locations', 'maxPrice', 'travelers']);
    // Required names are written in the order of the fields.
    expect(fieldsToSchema(fields!)).toEqual({
      ...flightBookingSchema,
      required: ['actions', 'flight', 'intent', 'locations', 'maxPrice', 'travelers'],
      properties: {
        ...flightBookingSchema.properties,
        flight: {...flightBookingSchema.properties.flight, required: ['from', 'id', 'to']},
      },
    });
    expect(validateFields(fields!)).toEqual([]);
    expect(previewConsentTemplate(flightBookingTemplate, fields!)).toEqual({
      text: 'Book flight ‹Flight› from ‹From› to ‹To›, spending up to 100 EUR',
      unknownPaths: [],
      malformed: [],
      unsafeText: false,
      tooLong: false,
    });
  });
});

describe('field rules', () => {
  it('offers the changes each kind supports', () => {
    expect(modificationsFor(field({kind: 'number'}))).toEqual(['none', 'decrease', 'increase']);
    expect(modificationsFor(field({kind: 'date'}))).toEqual(['none', 'decrease', 'increase']);
    expect(modificationsFor(field({kind: 'array'}))).toEqual(['none', 'removeItems', 'addItems']);
    expect(modificationsFor(field({kind: 'string'}))).toEqual(['none']);
    expect(modificationsFor(field({kind: 'object'}))).toEqual(['none']);
    expect(modificationsFor(field({kind: 'boolean'}))).toEqual(['none']);
  });

  it('orders numbers and dates by their type, as the server infers it', () => {
    expect(valueTypeFor(field({kind: 'string'}))).toBeUndefined();
    expect(valueTypeFor(field({kind: 'number'}))).toBe('number');
    expect(valueTypeFor(field({kind: 'date'}))).toBe('date');
    expect(valueTypeFor(field({kind: 'dateTime'}))).toBe('dateTime');
    expect(valueTypeFor(field({kind: 'boolean'}))).toBeUndefined();
  });

  it('reads dates by their format and writes them back', () => {
    const schema = {
      type: 'object',
      properties: {
        until: {type: 'string', format: 'date'},
        at: {type: 'string', format: 'date-time', policy: {modification: 'decrease', modifiableBy: ['user']}},
      },
    };
    const fields = schemaToFields(schema)!;
    expect(fields.map((f) => f.kind)).toEqual(['date', 'dateTime']);
    expect(fieldsToSchema(fields)).toEqual(schema);
  });

  it('cannot show what the server orders differently or the builder does not offer', () => {
    for (const property of [
      {type: 'integer'},
      {type: 'string', format: 'decimal'},
      {type: 'string', format: 'email'},
      {type: 'number', format: 'date'},
      {type: 'number', policy: {valueType: 'number'}},
    ]) {
      expect(schemaToFields({type: 'object', properties: {a: property}})).toBeNull();
    }
  });

  it('treats a supplied field as unchangeable', () => {
    expect(effectiveModification(field({policy: {suppliedBy: 'processor', modification: 'decrease'}}))).toBe('none');
    expect(effectiveModification(field({kind: 'boolean', policy: {modification: 'decrease'}}))).toBe('none');
  });
});

describe('validateFields', () => {
  it('flags allowed values that are not numbers or appear twice', () => {
    const numbers = {...newField('array'), name: 'codes', itemKind: 'number' as const, allowedValues: ['1', 'x']};
    const repeated = {...newField('array'), name: 'actions', allowedValues: ['a', 'a']};
    expect(validateFields([numbers]).map((i) => i.messageKey)).toEqual(['fields.errors.allowedValuesInvalid']);
    expect(validateFields([repeated]).map((i) => i.messageKey)).toEqual(['fields.errors.allowedValuesDuplicate']);
  });

  it('compares number allowed values by value, and refuses numbers the server cannot hold', () => {
    const numbers = (allowedValues: string[]): SchemaField =>
      ({...newField('array'), name: 'codes', itemKind: 'number', allowedValues}) as SchemaField;
    expect(validateFields([numbers(['1', '1.0'])]).map((i) => i.messageKey)).toEqual([
      'fields.errors.allowedValuesDuplicate',
    ]);
    expect(validateFields([numbers(['1', '1e0', '10'])]).map((i) => i.messageKey)).toEqual([
      'fields.errors.allowedValuesDuplicate',
    ]);
    expect(validateFields([numbers(['1', '2'])])).toEqual([]);
    expect(validateFields([numbers(['1e400'])]).map((i) => i.messageKey)).toEqual([
      'fields.errors.allowedValuesInvalid',
    ]);
    const texts = {...newField('array'), name: 'codes', allowedValues: ['1', '1.0']};
    expect(validateFields([texts]), 'text is compared as written').toEqual([]);
  });

  it('refuses a date and time the user would change on the consent screen', () => {
    const when = (modifiableBy: SchemaField['policy']['modifiableBy']): SchemaField =>
      field({kind: 'dateTime', policy: {modification: 'decrease', modifiableBy}});
    expect(validateFields([when(['user', 'client'])]).map((i) => i.messageKey)).toEqual(['fields.errors.dateTimeUser']);
    expect(validateFields([when(['client', 'processor'])])).toEqual([]);
    expect(validateFields([field({kind: 'date', policy: {modification: 'decrease', modifiableBy: ['user']}})])).toEqual(
      [],
    );
  });

  const messages = (fields: SchemaField[]): string[] => validateFields(fields).map((issue) => issue.messageKey);

  it('accepts a valid set of fields', () => {
    expect(validateFields(schemaToFields(paymentSchema)!)).toEqual([]);
  });

  it('refuses a dot in a field name', () => {
    expect(messages([field({name: 'a.b'})])).toContain('fields.errors.nameDot');
  });

  it('reports missing and duplicate names, and reserves none', () => {
    expect(messages([field({name: ' '})])).toContain('fields.errors.nameRequired');
    expect(messages([field({name: 'a'}), field({name: 'a'})])).toContain('fields.errors.nameDuplicate');
    expect(messages([field({name: 'detailId'})])).toEqual([]);
  });

  it('refuses only a supplied type', () => {
    expect(messages([field({name: 'type', policy: {suppliedBy: 'processor'}})])).toContain(
      'fields.errors.typeSupplied',
    );
    expect(messages([field({name: 'actions', policy: {suppliedBy: 'processor'}})])).toEqual([]);
  });

  it('requires who may change a modifiable field', () => {
    expect(messages([field({kind: 'number', policy: {modification: 'decrease'}})])).toContain(
      'fields.errors.modifiableByRequired',
    );
  });

  it('checks limits against the value type', () => {
    expect(messages([field({kind: 'number', policy: {maximum: '1.5.0'}})])).toContain('fields.errors.limitInvalid');
    expect(messages([field({kind: 'number', policy: {maximum: '10000.00'}})])).toEqual([]);
    expect(messages([field({kind: 'number', policy: {minimum: '10', maximum: '2'}})])).toContain(
      'fields.errors.limitOrder',
    );
    expect(messages([field({kind: 'date', policy: {minimum: '2026-12-31', maximum: '2026-01-01'}})])).toContain(
      'fields.errors.limitOrder',
    );
    expect(messages([field({kind: 'number', policy: {minimum: '2', maximum: '10'}})])).toEqual([]);
  });

  it('checks the fewest items of a list is a whole number', () => {
    expect(validateFields([field({kind: 'array', minItems: 'one'})]).map((i) => i.messageKey)).toEqual([
      'fields.errors.minItemsInvalid',
    ]);
    expect(validateFields([field({kind: 'array', minItems: '0'})])).toEqual([]);
  });

  it('checks the maximum length is a whole number', () => {
    expect(messages([field({maxLength: '0'})])).toEqual([]);
    for (const maxLength of ['1.5', '-1', 'ten']) {
      expect(messages([field({maxLength})])).toContain('fields.errors.maxLengthInvalid');
    }
    expect(messages([field({maxLength: '100'})])).toEqual([]);
    expect(messages([field({maxLength: ''})])).toEqual([]);
  });

  it('checks nested fields', () => {
    expect(messages([field({kind: 'object', name: 'g', children: [field({name: ''})]})])).toContain(
      'fields.errors.nameRequired',
    );
  });
});

describe('isValidLimit', () => {
  it('parses each value type', () => {
    expect(isValidLimit('-1.25', 'number')).toBe(true);
    expect(isValidLimit('1e3', 'number')).toBe(true);
    expect(isValidLimit('1.5E-7', 'number')).toBe(true);
    expect(isValidLimit('1e', 'number')).toBe(false);
    expect(isValidLimit('2026-02-30x', 'date')).toBe(false);
    expect(isValidLimit('2026-01-31', 'date')).toBe(true);
    expect(isValidLimit('2026-02-30', 'date')).toBe(false);
    expect(isValidLimit('2026-01-31T09:00:00Z', 'dateTime')).toBe(true);
    expect(isValidLimit('2026-01-31T09:00:00.5+05:30', 'dateTime')).toBe(true);
    for (const value of ['2026-01-31', '2026-01-31T09:00', 'Jan 31 2026', '2026-02-30T09:00:00Z', 'soon']) {
      expect(isValidLimit(value, 'dateTime')).toBe(false);
    }
  });

  it('reads a date and time and a number as the server does', () => {
    expect(isValidLimit('2026-01-31T09:00:00-08:00', 'dateTime')).toBe(true);
    expect(isValidLimit('2026-01-31T09:00:00.123456789Z', 'dateTime')).toBe(true);
    for (const value of [
      '2026-12-31T23:59:60Z',
      '2026-01-31t09:00:00Z',
      '2026-01-31T09:00:00z',
      '2026-01-31T09:00:00+0530',
    ]) {
      expect(isValidLimit(value, 'dateTime'), value).toBe(false);
    }
    expect(isValidLimit('1e308', 'number')).toBe(true);
    expect(isValidLimit('1e400', 'number'), 'not a finite double').toBe(false);
    expect(isValidLimit('-1e400', 'number')).toBe(false);
  });
});

describe("the server's rules", () => {
  const messages = (fields: SchemaField[]): string[] => validateFields(fields).map((issue) => issue.messageKey);

  it('reports a name or an allowed value with surrounding space instead of trimming it', () => {
    expect(messages([field({name: ' a'})])).toContain('fields.errors.nameSpaces');
    expect(messages([field({kind: 'array', allowedValues: ['pay ', 'status']})])).toContain(
      'fields.errors.allowedValuesSpaces',
    );
  });

  it('requires the type member to be text the processor does not set', () => {
    expect(messages([field({name: 'type', kind: 'number'})])).toContain('fields.errors.typeKind');
    const dated = validateFields([field({name: 'type', kind: 'date'})]);
    expect(dated.map((issue) => issue.defaultMessage)).toEqual([
      'The type identifies the request, so it must be plain text.',
    ]);
    expect(messages([field({name: 'type', kind: 'object', children: [field({name: 'x'})]})])).toContain(
      'fields.errors.typeKind',
    );
    expect(messages([field({name: 'type'})])).toEqual([]);
  });

  it('keeps an empty list of allowed values, and titles, as written', () => {
    expect(schemaToFields({type: 'object', properties: {a: {type: 'array', items: {type: 'string', enum: []}}}})).toBe(
      null,
    );
    const fields = schemaToFields({type: 'object', properties: {a: {type: 'string', title: ' Amount '}}});
    expect(fieldsToSchema(fields!)).toEqual({type: 'object', properties: {a: {type: 'string', title: ' Amount '}}});
  });

  it('marks a consent summary longer than the server stores', () => {
    expect(previewConsentTemplate('é'.repeat(500), []).tooLong).toBe(false);
    expect(previewConsentTemplate('é'.repeat(501), []).tooLong).toBe(true);
    expect(previewConsentTemplate(`  ${'é'.repeat(500)}\n`, []).tooLong, 'it is measured as it is saved').toBe(false);
  });

  it('reads placeholders as the server does', () => {
    const fields = schemaToFields({type: 'object', properties: {départ: {type: 'string'}}});
    expect(previewConsentTemplate('From {{départ}}', fields!).unknownPaths).toEqual([]);
    expect(previewConsentTemplate('{{a{b}}', fields!).unknownPaths).toEqual([]);
  });
});

describe('usesProcessor', () => {
  it('finds fields the processor sets or changes, also inside objects', () => {
    expect(usesProcessor(schemaToFields(paymentSchema)!)).toBe(true);
    expect(usesProcessor([field({kind: 'number', policy: {modification: 'decrease', modifiableBy: ['user']}})])).toBe(
      false,
    );
    expect(usesProcessor([field({kind: 'object', children: [field({policy: {suppliedBy: 'processor'}})]})])).toBe(true);
  });
});

describe('valuePaths', () => {
  it('lists the paths of fields that hold a value', () => {
    expect(valuePaths(schemaToFields(paymentSchema)!)).toEqual([
      'actions',
      'instructedAmount.amount',
      'instructedAmount.currency',
      'creditorName',
      'debtorAccount.iban',
      'executeBy',
      'retries',
    ]);
  });
});

describe('previewConsentTemplate', () => {
  it('fills placeholders with sample values and reports unknown ones', () => {
    const fields = schemaToFields(paymentSchema)!;
    const preview = previewConsentTemplate(
      'Pay {{ instructedAmount.amount }} {{instructedAmount.currency}} to {{creditorName}} by {{executeBy}} {{actions}} {{nope}}',
      fields,
    );
    expect(preview.text).toBe('Pay 100 EUR to ‹Pay to› by 2026-01-31 first, second ');
    expect(preview.unknownPaths).toEqual(['nope']);
  });

  it('samples a number within its limits, a currency as a code, and writes values as the rows do', () => {
    expect(sampleValue(field({kind: 'number'}))).toBe('100');
    expect(sampleValue(field({kind: 'number', policy: {maximum: '9', minimum: '1'}}))).toBe('9');
    expect(sampleValue(field({kind: 'number', policy: {minimum: '250'}}))).toBe('250');
    expect(sampleValue(field({kind: 'number', policy: {maximum: 'x'}})), 'an invalid limit is ignored').toBe('100');
    expect(sampleValue(field({name: 'currency', title: 'Currency'}))).toBe('EUR');
    expect(sampleValue(field({name: 'settlementCurrency'}))).toBe('EUR');
    expect(sampleValue(field({name: 'payee', title: 'Pay to'}))).toBe('‹Pay to›');
    expect(sampleValue(field({kind: 'array', itemKind: 'number'}))).toBe('1, 2');
  });

  it('samples a date and a date and time within their limits', () => {
    expect(sampleValue(field({kind: 'date'}))).toBe('2026-01-31');
    expect(sampleValue(field({kind: 'date', policy: {minimum: '2027-03-01'}}))).toBe('2027-03-01');
    expect(sampleValue(field({kind: 'date', policy: {maximum: '2025-06-30'}}))).toBe('2025-06-30');
    expect(sampleValue(field({kind: 'dateTime', policy: {minimum: '2026-02-01T00:00:00Z'}}))).toBe(
      '2026-02-01T00:00:00Z',
    );
    expect(sampleValue(field({kind: 'dateTime', policy: {maximum: '2026-01-31T08:00:00+01:00'}}))).toBe(
      '2026-01-31T08:00:00+01:00',
    );
    expect(sampleValue(field({kind: 'dateTime', policy: {maximum: 'soon'}})), 'an invalid limit is ignored').toBe(
      '2026-01-31T09:00:00Z',
    );
  });

  it('takes a placeholder that names an object, and writes the object as the server does, as JSON', () => {
    const fields = schemaToFields(paymentSchema)!;
    const preview = previewConsentTemplate('Pay {{instructedAmount}}', fields);
    expect(preview.unknownPaths).toEqual([]);
    expect(preview.text).toBe('Pay {"amount":100,"currency":"EUR"}');
    expect(
      checkConsentTemplate('Pay {{instructedAmount}}', schemaPaths(paymentSchema)).unknownPaths,
      'as for a schema the composer cannot show',
    ).toEqual([]);
  });

  it('knows only the type besides declared fields', () => {
    expect(previewConsentTemplate('{{type}}', []).unknownPaths).toEqual([]);
    expect(previewConsentTemplate('{{locations}}', []).unknownPaths).toEqual(['locations']);
  });
});

describe('checkConsentTemplate', () => {
  it('checks a summary against the paths a schema the composer cannot show declares', () => {
    const schema = {
      type: 'object',
      properties: {
        payee: {type: 'object', properties: {iban: {type: 'string', pattern: '^[A-Z]+$'}}},
        amount: {anyOf: [{type: 'number'}]},
      },
    };
    expect(schemaPaths(schema)).toEqual(['type', 'payee', 'payee.iban', 'amount']);
    const check = checkConsentTemplate(
      'Pay {{amount}} to {{payee.iban}} {{payee}} {{type}} {{fee}}',
      schemaPaths(schema),
    );
    expect(check.unknownPaths).toEqual(['fee']);
    expect(check.tooLong).toBe(false);
    expect(checkConsentTemplate(` ${'é'.repeat(501)} `, ['type']).tooLong).toBe(true);
  });
});

describe('schemaDefinitionText', () => {
  it('is the same for schemas the server treats as one', () => {
    const base = {
      type: 'object',
      required: ['a', 'b'],
      properties: {
        a: {type: 'number', policy: {modification: 'decrease', modifiableBy: ['user', 'client']}},
        b: {type: 'object', properties: {c: {type: 'string'}}},
      },
    };
    const same = {
      properties: {
        b: {properties: {c: {type: 'string', policy: {suppliedBy: 'client', modification: 'none'}}}, type: 'object'},
        a: {policy: {modifiableBy: ['client', 'user', 'client'], modification: 'decrease'}, type: 'number'},
      },
      required: ['b', 'type', 'a', 'a'],
      type: 'object',
    };
    expect(schemaDefinitionText(same)).toBe(schemaDefinitionText(base));
    expect(schemaDefinitionText({type: 'object', properties: {}})).toBe(schemaDefinitionText({type: 'object'}));

    for (const other of [
      {...base, required: ['a']},
      {...base, properties: {...base.properties, a: {type: 'number'}}},
      {...base, properties: {...base.properties, d: {type: 'string'}}},
    ]) {
      expect(schemaDefinitionText(other)).not.toBe(schemaDefinitionText(base));
    }
    expect(
      schemaDefinitionText({type: 'object', properties: {o: {type: 'object', required: ['type']}}}),
      'only the root keeps type out of its required names',
    ).not.toBe(schemaDefinitionText({type: 'object', properties: {o: {type: 'object'}}}));
  });
});

describe('type value and processor endpoint', () => {
  it('accepts printable type values without spaces', () => {
    expect(isValidTypeValue('payment_initiation')).toBe(true);
    expect(isValidTypeValue('https://scheme.example.com/payment/v1')).toBe(true);
    expect(isValidTypeValue('has space')).toBe(false);
    expect(isValidTypeValue('')).toBe(false);
    expect(isValidTypeValue('x'.repeat(513))).toBe(false);
  });

  it('accepts HTTPS, and HTTP only on loopback', () => {
    expect(isValidProcessorEndpoint('https://bank.example.com/process')).toBe(true);
    expect(isValidProcessorEndpoint('http://127.0.0.1:9099/process')).toBe(true);
    expect(isValidProcessorEndpoint('http://localhost/process')).toBe(true);
    expect(isValidProcessorEndpoint('http://bank.example.com/process')).toBe(false);
    expect(isValidProcessorEndpoint('not a url')).toBe(false);
    expect(isValidProcessorEndpoint('http://LOCALHOST/process')).toBe(true);
    expect(isValidProcessorEndpoint('http://127.0.0.2:9099/process')).toBe(true);
    expect(isValidProcessorEndpoint('http://[::1]/process')).toBe(true);
    for (const rewritten of ['http://127.1/process', 'https:bank.example.com', 'http://0x7f.0.0.1/process']) {
      expect(isValidProcessorEndpoint(rewritten)).toBe(false);
    }
  });

  it('accepts what the server accepts as written', () => {
    for (const endpoint of [
      'http://[0:0:0:0:0:0:0:1]:9099/process',
      'http://[::ffff:127.0.0.1]:9099/process',
      'http://[::FFFF:7F00:1]/process',
      'https://例え.jp/process',
      'HTTPS://bank.example.com/process',
      'http://127.255.255.254:9099/process',
    ]) {
      expect(isValidProcessorEndpoint(endpoint), endpoint).toBe(true);
    }
    for (const endpoint of [
      'http://example.com/process',
      'http://[::2]/process',
      'http://[::ffff:10.0.0.1]/process',
      'http://127.0.0.256/process',
      'http://127.0.0.01/process',
      'http://127.0.0/process',
      'http://localhost.example.com/process',
      'http://localhost@example.com/process',
      'https:/bank.example.com/process',
      'https:///process',
      'ftp://localhost/process',
    ]) {
      expect(isValidProcessorEndpoint(endpoint), endpoint).toBe(false);
    }
  });

  it('refuses credentials in the URL and a port outside 1 to 65535, as the server does', () => {
    for (const endpoint of [
      'http://u:p@127.0.0.1:9099/process',
      'https://user@bank.example.com/process',
      'https://@bank.example.com/process',
      'https://example.com:99999/p',
      'https://example.com:65536/p',
      'https://example.com:0/p',
    ]) {
      expect(isValidProcessorEndpoint(endpoint), endpoint).toBe(false);
    }
    for (const endpoint of ['https://example.com:65535/p', 'https://example.com:1/p', 'https://bank.example.com/a@b']) {
      expect(isValidProcessorEndpoint(endpoint), endpoint).toBe(true);
    }
  });
});

describe('missingStandardFields', () => {
  it('offers the standard lists a type does not declare yet', () => {
    expect(missingStandardFields([]).map((f) => [f.name, f.kind, f.itemKind])).toEqual([
      ['locations', 'array', 'string'],
      ['actions', 'array', 'string'],
    ]);
    expect(missingStandardFields([field({name: 'actions'})]).map((f) => f.name)).toEqual(['locations']);
    expect(missingStandardFields([field({name: 'locations'}), field({name: 'actions'})])).toEqual([]);
  });
});

describe('consent summary placeholders', () => {
  it('lists the paths a summary names, each once', () => {
    expect(templatePaths('Pay {{amount}} {{ currency }} to {{payee.name}}, {{amount}}')).toEqual([
      'amount',
      'currency',
      'payee.name',
    ]);
  });

  it('finds the placeholders that name a field or a field inside it', () => {
    const template = 'Pay {{money.amount}} {{money.currency}} for {{moneyNote}}';
    expect(pathsReferencing(template, 'money')).toEqual(['money.amount', 'money.currency']);
    expect(pathsReferencing(template, '')).toEqual([]);
  });

  it('renames the placeholders of a renamed field and of the fields inside it', () => {
    expect(renameTemplatePaths('Pay {{money.amount}} {{ money }} {{moneyNote}}', 'money', 'total')).toBe(
      'Pay {{total.amount}} {{total}} {{moneyNote}}',
    );
    expect(renameTemplatePaths('Pay {{amount}}', 'amount', '')).toBe('Pay {{amount}}');
  });
});

describe('composer fixes', () => {
  const field = (kind: SchemaField['kind'], name: string, extra: Partial<SchemaField> = {}): SchemaField => ({
    ...newField(kind),
    name,
    ...extra,
  });
  const keys = (fields: SchemaField[]): string[] => validateFields(fields).map((issue) => issue.messageKey);

  it('refuses a change rule inside an object the processor supplies, as the server does', () => {
    const limit = field('number', 'limit', {policy: {modification: 'decrease', modifiableBy: ['user']}});
    const grp = field('object', 'grp', {policy: {suppliedBy: 'processor'}, children: [limit]});
    expect(keys([grp])).toEqual(['fields.errors.modifiableInSupplied']);
    expect(keys([{...grp, policy: {}}])).toEqual([]);
  });

  it('refuses a space inside a field name, which a summary placeholder cannot name', () => {
    expect(keys([field('string', 'z z')])).toEqual(['fields.errors.nameInnerSpace']);
  });

  it('caps a maximum length and a fewest number of items at what the server holds', () => {
    expect(keys([field('string', 's', {maxLength: '2147483647'})])).toEqual([]);
    expect(keys([field('string', 's', {maxLength: '9223372036854775808'})])).toEqual([
      'fields.errors.maxLengthTooLarge',
    ]);
    expect(keys([field('array', 'l', {minItems: '2147483648'})])).toEqual(['fields.errors.minItemsTooLarge']);
  });

  it('fills {{type}} with the type identifier, as the server does, even when a type field is declared', () => {
    expect(previewConsentTemplate('Pay {{type}}', [], 'payment').text).toBe('Pay payment');
    expect(
      previewConsentTemplate('Pay {{type}}', [field('string', 'type', {title: 'Request type'})], 'payment').text,
    ).toBe('Pay payment');
  });

  it('finds text in braces that is not a placeholder', () => {
    expect(malformedPlaceholders('Pay {{z z}} {{amount}} {{ }} {{z z}}')).toEqual(['{{z z}}', '{{ }}']);
    expect(malformedPlaceholders('Pay {{ amount }}')).toEqual([]);
  });

  it('finds the placeholders that name a field a request can leave out', () => {
    const fields = [
      field('number', 'amount', {required: true}),
      field('string', 'note'),
      field('object', 'payee', {required: true, children: [field('string', 'iban', {required: true})]}),
      field('object', 'extra', {children: [field('string', 'code', {required: true})]}),
    ];
    expect(optionalPaths('{{amount}} {{note}} {{payee.iban}} {{extra.code}} {{type}}', fields)).toEqual([
      'note',
      'extra.code',
    ]);
  });

  it('samples at least the fewest items of a list, up to a few', () => {
    expect(sampleData(field('array', 'l', {minItems: '5'}))).toEqual(['first', 'second', 'third', 'fourth', 'fifth']);
    expect(sampleData(field('array', 'l', {minItems: '3', allowedValues: ['a', 'b']}))).toEqual(['a', 'b', 'a']);
    expect(sampleData(field('array', 'l', {minItems: '999'}))).toHaveLength(10);
    expect(sampleData(field('array', 'l', {allowedValues: ['a']}))).toEqual(['a']);
  });

  it('samples a nested field named __proto__ as its own value', () => {
    const sample = sampleData(field('object', 'o', {children: [field('string', '__proto__')]})) as object;
    expect(Object.keys(sample)).toEqual(['__proto__']);
  });
});

describe('schemaKeywordIssues', () => {
  const issues = (schema: Record<string, unknown>): string[] =>
    schemaKeywordIssues(schema).map(
      (issue) => `${issue.path}|${issue.messageKey}${issue.values ? `|${Object.values(issue.values).join()}` : ''}`,
    );

  it('accepts what the server accepts', () => {
    expect(schemaKeywordIssues(flightBookingSchema)).toEqual([]);
    expect(
      issues({
        $schema: 'https://json-schema.org/draft/2020-12/schema',
        type: 'object',
        required: ['type', 'n'],
        properties: {
          n: {type: 'number', minimum: 0, multipleOf: 1, title: 'N', policy: {suppliedBy: 'processor'}},
          s: {type: 'string', pattern: '^a', format: 'date', minLength: 1, maxLength: 3},
          e: {enum: ['a', 'b']},
          l: {type: 'array', items: {type: 'string'}, minItems: 1, maxItems: 2, uniqueItems: true},
        },
      }),
    ).toEqual([]);
  });

  it('names each keyword and value the server refuses, with where it is', () => {
    expect(
      issues({
        type: 'object',
        additionalProperties: false,
        required: ['missing'],
        properties: {
          o: {type: 'object', properties: {q: {oneOf: [{type: 'string'}]}, r: {$ref: '#/x'}}},
          s: {type: 'string', format: 'email', maxLength: 9223372036854775808},
          l: {type: 'array'},
          items: {type: 'array', items: {type: 'object', properties: {p: {type: 'string', policy: {}}}}},
          e: {type: 'string', enum: ['a', 'a', ' b']},
          t: {type: 'string', title: ''},
          u: {anyOf: []},
          w: {type: 'text'},
          x: {type: 'integer', minimum: 0},
          y: {type: 'null'},
        },
      }),
    ).toEqual([
      '|schema.errors.keyword|additionalProperties',
      '|schema.errors.requiredUndeclared|missing',
      'e|schema.errors.enum',
      'items.p|schema.errors.keyword|policy',
      'l|schema.errors.items',
      'o.q|schema.errors.keyword|oneOf',
      'o.q|schema.errors.unconstrained',
      'o.r|schema.errors.keyword|$ref',
      'o.r|schema.errors.unconstrained',
      's|schema.errors.count|maxLength',
      's|schema.errors.format',
      't|schema.errors.title',
      'u|schema.errors.keyword|anyOf',
      'u|schema.errors.unconstrained',
      'w|schema.errors.type',
      'x|schema.errors.integerType',
      'y|schema.errors.nullType',
    ]);
  });

  it('requires an object, the 2020-12 dialect, and a plain text type member', () => {
    expect(issues({type: 'array', $schema: 'http://json-schema.org/draft-07/schema', items: {type: 'string'}})).toEqual(
      ['|schema.errors.object', '|schema.errors.dialect'],
    );
    expect(issues({type: 'object', properties: {type: {type: 'number'}}})).toEqual(['type|fields.errors.typeKind']);
  });
});
