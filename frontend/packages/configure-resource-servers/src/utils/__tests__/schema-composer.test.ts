// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import type {SchemaField} from '../../models/authorization-detail-type';
import {fieldsToSchema, newField, schemaToFields} from '../authorization-detail-schema';
import {
  applyDrop,
  applyMove,
  changeKind,
  createPaletteField,
  droppedSettings,
  duplicateField,
  duplicateLabels,
  exampleDetail,
  findField,
  findPosition,
  flattenFields,
  insertField,
  moveAbilities,
  moveField,
  moveTarget,
  previewRow,
  recordPaths,
  removeField,
  setSuppliedBy,
  summaryRenames,
  toggleParty,
  updateField,
  type FieldMove,
} from '../schema-composer';

const named = (kind: SchemaField['kind'], name: string, extra: Partial<SchemaField> = {}): SchemaField => ({
  ...newField(kind),
  name,
  ...extra,
});

/** A tree of fields: amount, payee { name, iban }, until. */
function sampleTree(): SchemaField[] {
  return [
    named('number', 'amount'),
    named('object', 'payee', {children: [named('string', 'name'), named('string', 'iban')]}),
    named('date', 'until'),
  ];
}

const names = (fields: SchemaField[]): unknown =>
  fields.map((field) => (field.kind === 'object' ? {[field.name]: names(field.children)} : field.name));

/** Reports whether every key in the tree is unique, as rendering relies on. */
function keysUnique(fields: SchemaField[]): boolean {
  const keys = flattenFields(fields).map((row) => row.field.key);
  return new Set(keys).size === keys.length;
}

describe('schema composer model', () => {
  it('creates palette fields with their defaults', () => {
    expect(createPaletteField('string')).toMatchObject({kind: 'string', maxLength: '100', name: ''});
    expect(createPaletteField('dateTime')).toMatchObject({kind: 'dateTime'});
    const money = createPaletteField('money');
    expect(money.kind).toBe('object');
    expect(fieldsToSchema([{...money, name: 'instructedAmount'}])).toEqual({
      type: 'object',
      properties: {
        instructedAmount: {
          type: 'object',
          required: ['amount', 'currency'],
          properties: {
            amount: {type: 'number', title: 'Amount'},
            currency: {type: 'string', title: 'Currency', maxLength: 3},
          },
        },
      },
    });
  });

  it('flattens the fields depth first with paths, in declared order', () => {
    const fields = sampleTree();
    expect(flattenFields(fields).map((row) => [row.path, row.depth, row.parentKey, row.index])).toEqual([
      ['amount', 0, null, 0],
      ['payee', 0, null, 1],
      ['payee.name', 1, fields[1].key, 0],
      ['payee.iban', 1, fields[1].key, 1],
      ['until', 0, null, 2],
    ]);
  });

  it('finds, updates, and removes nested fields', () => {
    const fields = sampleTree();
    const iban = fields[1].children[1];
    expect(findField(fields, iban.key)).toBe(iban);
    expect(findPosition(fields, iban.key)).toEqual({parentKey: fields[1].key, index: 1});
    expect(findPosition(fields, 'missing')).toBeUndefined();

    const updated = updateField(fields, iban.key, (field) => ({...field, title: 'IBAN'}));
    expect(findField(updated, iban.key)?.title).toBe('IBAN');
    expect(fields[1].children[1].title, 'the update does not mutate').toBe('');

    expect(names(removeField(fields, iban.key))).toEqual(['amount', {payee: ['name']}, 'until']);
    expect(names(removeField(fields, fields[1].key))).toEqual(['amount', 'until']);
  });

  it('inserts at a position, and only into objects', () => {
    const fields = sampleTree();
    const note = named('string', 'note');
    expect(names(insertField(fields, note, null, 1))).toEqual(['amount', 'note', {payee: ['name', 'iban']}, 'until']);
    expect(names(insertField(fields, note, fields[1].key, 0))).toEqual([
      'amount',
      {payee: ['note', 'name', 'iban']},
      'until',
    ]);
    expect(insertField(fields, note, fields[0].key, 0), 'a number holds no fields').toBe(fields);
  });

  it('moves fields among siblings, into objects, and out of them', () => {
    const fields = sampleTree();
    const [amount, payee, until] = fields;
    expect(names(moveField(fields, amount.key, null, 3))).toEqual([{payee: ['name', 'iban']}, 'until', 'amount']);
    expect(names(moveField(fields, until.key, null, 0))).toEqual(['until', 'amount', {payee: ['name', 'iban']}]);
    expect(names(moveField(fields, amount.key, payee.key, 1))).toEqual([{payee: ['name', 'amount', 'iban']}, 'until']);
    expect(names(moveField(fields, payee.children[0].key, null, 0))).toEqual([
      'name',
      'amount',
      {payee: ['iban']},
      'until',
    ]);
    expect(moveField(fields, amount.key, null, 1), 'a drop next to itself changes nothing').toBe(fields);
    expect(moveField(fields, payee.key, payee.key, 0), 'an object cannot go inside itself').toBe(fields);
  });

  it('never moves an object into a field inside it', () => {
    const inner = named('object', 'inner');
    const outer = named('object', 'outer', {children: [inner]});
    expect(moveField([outer], outer.key, inner.key, 0)).toEqual([outer]);
  });

  it('offers the moves a field can make without dragging', () => {
    const fields = sampleTree();
    const [amount, payee, until] = fields;
    expect(moveTarget(fields, amount.key, 'up')).toBeUndefined();
    expect(moveTarget(fields, amount.key, 'indent'), 'nothing above it').toBeUndefined();
    expect(moveTarget(fields, amount.key, 'outdent'), 'already at the top level').toBeUndefined();
    expect(moveTarget(fields, until.key, 'down')).toBeUndefined();
    expect(moveTarget(fields, until.key, 'indent')).toEqual({parentKey: payee.key, index: 2});

    expect(names(applyMove(fields, amount.key, 'down'))).toEqual([{payee: ['name', 'iban']}, 'amount', 'until']);
    expect(names(applyMove(fields, until.key, 'up'))).toEqual(['amount', 'until', {payee: ['name', 'iban']}]);
    expect(names(applyMove(fields, until.key, 'indent'))).toEqual(['amount', {payee: ['name', 'iban', 'until']}]);
    expect(names(applyMove(fields, payee.children[1].key, 'outdent'))).toEqual([
      'amount',
      {payee: ['name']},
      'iban',
      'until',
    ]);
    expect(applyMove(fields, amount.key, 'up')).toBe(fields);
  });

  it('duplicates a field and the fields inside it under new keys and a free name', () => {
    const fields = sampleTree();
    const first = duplicateField(fields, fields[1].key);
    expect(names(first?.fields ?? [])).toEqual([
      'amount',
      {payee: ['name', 'iban']},
      {payee_copy: ['name', 'iban']},
      'until',
    ]);
    expect(keysUnique(first?.fields ?? [])).toBe(true);
    const second = duplicateField(first?.fields ?? [], fields[1].key);
    expect(findField(second?.fields ?? [], second?.key ?? '')?.name).toBe('payee_copy2');
    expect(duplicateField(fields, 'missing')).toBeUndefined();
  });

  it('applies drops of fields of the tree', () => {
    const fields = sampleTree();
    const moved = applyDrop(fields, {key: fields[0].key}, {parentKey: fields[1].key, index: 0});
    expect(names(moved ?? [])).toEqual([{payee: ['amount', 'name', 'iban']}, 'until']);
    expect(applyDrop(fields, {key: fields[0].key}, {parentKey: null, index: 0})).toBeUndefined();
    expect(
      applyDrop(fields, {key: fields[1].key}, {parentKey: fields[0].key, index: 0}),
      'a number holds no fields',
    ).toBeUndefined();
  });

  it('keeps every edit a schema the conversion writes and reads back the same way', () => {
    let fields = sampleTree();
    fields = insertField(fields, createPaletteField('money'), null, 1);
    fields = applyMove(fields, fields[3].key, 'indent');
    fields = duplicateField(fields, fields[0].key)?.fields ?? [];
    fields = updateField(fields, fields[2].key, (field) => ({...field, name: 'price'}));
    const schema = fieldsToSchema(fields);
    const read = schemaToFields(schema);
    expect(read).not.toBeNull();
    expect(fieldsToSchema(read ?? [])).toEqual(schema);
  });

  it('drops what a new kind cannot carry', () => {
    const text = {...named('string', 'note'), maxLength: '40'};
    expect(changeKind(text, 'number').maxLength).toBeUndefined();
    const list = named('array', 'actions', {
      allowedValues: ['read'],
      policy: {modification: 'removeItems', modifiableBy: ['user']},
    });
    const asDate = changeKind(list, 'date');
    expect(asDate.allowedValues).toBeUndefined();
    expect(asDate.policy.modification, 'a date cannot lose items').toBeUndefined();
    const amount = named('number', 'amount', {policy: {modification: 'decrease', minimum: '1'}});
    expect(changeKind(amount, 'date').policy).toMatchObject({modification: 'decrease', minimum: '1'});
    expect(changeKind(amount, 'string').policy.minimum).toBeUndefined();
    expect(changeKind(named('object', 'g', {children: [named('string', 'a')]}), 'string').children).toEqual([]);
  });

  it('clears the change rule of a value the processor supplies', () => {
    const amount = named('number', 'amount', {policy: {modification: 'decrease', modifiableBy: ['client']}});
    const supplied = setSuppliedBy(amount, true);
    expect(supplied.policy).toEqual({suppliedBy: 'processor', modification: undefined, modifiableBy: undefined});
    expect(setSuppliedBy(supplied, false).policy.suppliedBy).toBeUndefined();
  });

  it('keeps the parties in order without repeats', () => {
    let field = named('number', 'amount');
    field = toggleParty(field, 'client');
    field = toggleParty(field, 'processor');
    expect(field.policy.modifiableBy).toEqual(['processor', 'client']);
    expect(toggleParty(field, 'client').policy.modifiableBy).toEqual(['processor']);
  });

  it('builds the consent row of each field as the server does', () => {
    const fields = [
      named('number', 'amount', {
        title: 'Amount',
        policy: {modification: 'decrease', modifiableBy: ['user', 'client']},
      }),
      named('date', 'until', {policy: {modification: 'increase', modifiableBy: ['client']}}),
      named('array', 'excluded', {
        allowedValues: ['games', 'travel'],
        policy: {modification: 'addItems', modifiableBy: ['user']},
      }),
      named('array', 'counts', {
        itemKind: 'number',
        allowedValues: ['1', '2', '3'],
        policy: {modification: 'addItems', modifiableBy: ['user']},
      }),
      named('array', 'tags', {policy: {modification: 'addItems', modifiableBy: ['user']}}),
      named('object', 'account', {policy: {suppliedBy: 'processor'}, children: [named('string', 'iban')]}),
    ];
    expect(
      flattenFields(fields)
        .filter((row) => row.field.kind !== 'object')
        .map(previewRow),
    ).toEqual([
      {path: 'amount', label: 'Amount', value: 100, modification: 'decrease', as: 'number'},
      {path: 'until', label: 'until', value: '2026-01-31'},
      {
        path: 'excluded',
        label: 'excluded',
        // One allowed value is requested, so the user is offered the other.
        value: ['games'],
        modification: 'addItems',
        options: ['games', 'travel'],
      },
      {path: 'counts', label: 'counts', value: [1], modification: 'addItems', options: [1, 2, 3]},
      {path: 'tags', label: 'tags', value: ['first', 'second'], modification: 'addItems'},
      {path: 'account.iban', label: 'account.iban', value: '‹iban›', origin: 'server'},
    ]);
  });

  it('generates an example detail with sample values of each type', () => {
    const fields = [
      named('number', 'amount'),
      named('string', 'creditorName', {title: 'Pay to'}),
      named('date', 'until'),
      named('dateTime', 'at'),
      named('boolean', 'recurring'),
      named('array', 'actions', {allowedValues: ['read', 'write', 'delete']}),
      named('array', 'counts', {itemKind: 'number'}),
      named('object', 'payee', {children: [named('string', 'iban'), named('string', '')]}),
      named('string', 'type'),
    ];
    expect(exampleDetail(fields, 'payment_initiation')).toEqual({
      type: 'payment_initiation',
      amount: 100,
      creditorName: '‹Pay to›',
      until: '2026-01-31',
      at: '2026-01-31T09:00:00Z',
      recurring: true,
      actions: ['read', 'write'],
      counts: [1, 2],
      payee: {iban: '‹iban›'},
    });
    expect(exampleDetail([], ' ')).toEqual({type: 'type'});
  });
});

describe('exampleDetail', () => {
  it('keeps a field named __proto__', () => {
    const field = {...newField('string'), name: '__proto__'};
    expect(Object.keys(exampleDetail([field], 'payment'))).toEqual(['type', '__proto__']);
  });
});

describe('droppedSettings', () => {
  it('names the settings with a value that a change of kind removed', () => {
    const list: SchemaField = {
      ...newField('array'),
      name: 'actions',
      allowedValues: ['read'],
      minItems: '1',
      policy: {modification: 'removeItems', modifiableBy: ['user']},
    };
    expect(droppedSettings(list, changeKind(list, 'string'))).toEqual(['allowedValues', 'minItems', 'change']);

    const object: SchemaField = {...newField('object'), name: 'money', children: [newField('number')]};
    expect(droppedSettings(object, changeKind(object, 'number'))).toEqual(['fields']);

    const amount: SchemaField = {...newField('number'), policy: {minimum: '1', maximum: ''}};
    expect(droppedSettings(amount, changeKind(amount, 'date'))).toEqual([]);
    expect(droppedSettings(amount, changeKind(amount, 'boolean'))).toEqual(['limits']);
  });
});

describe('nested processor-supplied fields', () => {
  it('clears the change rules of the fields inside an object the processor supplies', () => {
    const group = named('object', 'grp', {
      children: [
        named('number', 'limit', {policy: {modification: 'decrease', modifiableBy: ['user']}}),
        named('object', 'inner', {
          children: [named('array', 'tags', {policy: {modification: 'removeItems', modifiableBy: ['client']}})],
        }),
      ],
    });
    const supplied = setSuppliedBy(group, true);
    expect(fieldsToSchema([supplied])).toEqual({
      type: 'object',
      properties: {
        grp: {
          type: 'object',
          policy: {suppliedBy: 'processor'},
          properties: {
            limit: {type: 'number'},
            inner: {type: 'object', properties: {tags: {type: 'array', items: {type: 'string'}}}},
          },
        },
      },
    });
  });

  it('shows a field inside an object the processor supplies as one nobody can change', () => {
    const limit = named('number', 'limit', {policy: {modification: 'decrease', modifiableBy: ['user']}});
    expect(previewRow({field: limit, path: 'grp.limit', processorSupplied: true})).toEqual({
      path: 'grp.limit',
      label: 'grp.limit',
      value: 100,
      origin: 'server',
    });
  });

  it('leaves out of the example request what the processor supplies, and objects left with no value', () => {
    const fields = [
      named('number', 'amount'),
      named('string', 'iban', {policy: {suppliedBy: 'processor'}}),
      named('object', 'fx', {children: [named('number', 'rate', {policy: {suppliedBy: 'processor'}})]}),
      named('object', 'account', {policy: {suppliedBy: 'processor'}, children: [named('string', 'id')]}),
      named('object', 'empty'),
      named('object', 'marker', {required: true}),
    ];
    expect(exampleDetail(fields, 'payment')).toEqual({type: 'payment', amount: 100, marker: {}});
  });
});

describe('card performance helpers', () => {
  it('keeps the fields that do not hold a changed field as they are', () => {
    const fields = sampleTree();
    const next = updateField(fields, fields[1].children[0].key, (field) => ({...field, title: 'Name'}));
    expect(next[0]).toBe(fields[0]);
    expect(next[2]).toBe(fields[2]);
    expect(next[1]).not.toBe(fields[1]);
    expect(next[1].children[1]).toBe(fields[1].children[1]);
    expect(updateField(fields, 'missing', (field) => field)).toBe(fields);
  });

  it('finds the moves of every field in one pass, as moveTarget finds them one at a time', () => {
    const fields = [
      named('string', 'a'),
      named('object', 'obj', {children: [named('string', 'x'), named('object', 'inner'), named('string', 'y')]}),
      named('string', 'b'),
    ];
    const abilities = moveAbilities(fields);
    for (const row of flattenFields(fields)) {
      for (const move of ['up', 'down', 'indent', 'outdent'] as FieldMove[]) {
        expect(abilities.get(row.field.key)?.[move], `${row.path} ${move}`).toBe(
          moveTarget(fields, row.field.key, move) !== undefined,
        );
      }
    }
  });
});

describe('duplicateLabels', () => {
  it('names the labels the consent screen would show on more than one row', () => {
    const money = createPaletteField('money');
    const fields = [named('number', 'amount', {title: 'Amount'}), {...money, name: 'fee'}, named('string', 'note')];
    expect(duplicateLabels(fields)).toEqual(['Amount']);
    expect(
      duplicateLabels([named('string', 'type', {title: 'Kind'}), named('string', 'kind', {title: 'Kind'})]),
    ).toEqual([]);
  });
});

describe('summaryRenames', () => {
  it('finds where a field the summary names went after a rename or a move', () => {
    const fields = sampleTree();
    const history = recordPaths({}, flattenFields(fields));
    expect(recordPaths(history, flattenFields(fields)), 'nothing new to record').toBe(history);

    const renamed = updateField(fields, fields[1].key, (field) => ({...field, name: 'creditor'}));
    expect(summaryRenames(['payee.iban', 'amount'], flattenFields(renamed), history)).toEqual([
      {from: 'payee', to: 'creditor', keys: [fields[1].key, fields[1].children[1].key]},
    ]);

    const moved = applyMove(fields, fields[1].children[0].key, 'outdent');
    expect(summaryRenames(['payee.name'], flattenFields(moved), history)).toEqual([
      {from: 'payee.name', to: 'name', keys: [fields[1].children[0].key]},
    ]);

    const unnamed = updateField(fields, fields[0].key, (field) => ({...field, name: ''}));
    expect(summaryRenames(['amount'], flattenFields(unnamed), history), 'not to a field without a name').toEqual([]);
    const spaced = updateField(fields, fields[0].key, (field) => ({...field, name: 'z z'}));
    expect(summaryRenames(['amount'], flattenFields(spaced), history), 'nor to one no placeholder can name').toEqual(
      [],
    );
  });
});
