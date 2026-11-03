// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {act, renderHook} from '@thunderid/test-utils';
import {describe, it, expect} from 'vitest';
import type {AuthorizationDetailType, SchemaField} from '../../models/authorization-detail-type';
import useAuthorizationDetailTypeForm, {errorLogDetails} from '../useAuthorizationDetailTypeForm';

function field(key: string, name: string): SchemaField {
  return {key, name, title: '', kind: 'string', required: false, itemKind: 'string', policy: {}, children: []};
}

const paymentType: AuthorizationDetailType = {
  id: 'type-1',
  type: 'payment_initiation',
  name: 'Payment',
  description: 'Pays once',
  version: 1,
  schema: {type: 'object', properties: {amount: {type: 'number', title: 'Amount'}}},
  processor: {endpoint: 'https://bank.example.com/process'},
  reuse: false,
};

describe('useAuthorizationDetailTypeForm', () => {
  it('derives the type identifier from the base letters of an accented name', () => {
    const {result} = renderHook(() => useAuthorizationDetailTypeForm());

    act(() => result.current.setName("Paiement à l'étranger"));
    expect(result.current.typeValue).toBe('paiement_a_l_etranger');
    act(() => result.current.setName('Größe'));
    expect(result.current.typeValue, 'a sharp s becomes ss').toBe('grosse');
  });

  it('reports a missing type identifier once a name leaves it empty, before anything is revealed', () => {
    const {result} = renderHook(() => useAuthorizationDetailTypeForm());
    expect(result.current.typeMissing, 'not before anything is entered').toBe(false);

    act(() => result.current.setName('支払い'));
    expect(result.current.typeValue).toBe('');
    expect(result.current.typeMissing).toBe(true);
    expect(result.current.issuesRevealed).toBe(false);
  });

  it('reports a cleared name of a type being edited at once', () => {
    const {result} = renderHook(() => useAuthorizationDetailTypeForm(paymentType));

    act(() => result.current.setName(''));
    expect(result.current.nameMissing).toBe(true);
  });

  it('does not count spaces around a value as a change', () => {
    const {result} = renderHook(() => useAuthorizationDetailTypeForm(paymentType));

    act(() => {
      result.current.setName('Payment ');
      result.current.setDescription(' Pays once');
      result.current.setEndpoint('https://bank.example.com/process ');
      result.current.setTemplate(' ');
    });
    expect(result.current.hasChanges).toBe(false);

    act(() => result.current.setName('Payment v2'));
    expect(result.current.hasChanges).toBe(true);
  });

  it('does not count keywords in another order as a change, but counts fields in another order', () => {
    const schema = {
      type: 'object',
      properties: {
        amount: {type: 'number', title: 'Amount', minimum: 1, maximum: 5},
        payee: {type: 'string', title: 'Payee'},
      },
    };
    // A schema with a keyword the composer does not know opens as JSON.
    const {result} = renderHook(() =>
      useAuthorizationDetailTypeForm({...paymentType, schema: {...schema, $comment: 'kept'}}),
    );
    expect(result.current.schemaMode).toBe('json');

    act(() =>
      result.current.setJsonText(
        JSON.stringify({
          $comment: 'kept',
          properties: {
            amount: {maximum: 5, minimum: 1, title: 'Amount', type: 'number'},
            payee: {title: 'Payee', type: 'string'},
          },
          type: 'object',
        }),
      ),
    );
    expect(result.current.hasChanges).toBe(false);

    act(() =>
      result.current.setJsonText(
        JSON.stringify({
          $comment: 'kept',
          type: 'object',
          properties: {payee: schema.properties.payee, amount: schema.properties.amount},
        }),
      ),
    );
    expect(result.current.hasChanges, 'the consent screen shows the fields in their order').toBe(true);
  });

  it.each([
    ['fields with the same name', [field('a', 'amount'), field('b', 'amount')], 'fields.errors.json.nameDuplicate'],
    ['a field without a name', [field('a', 'amount'), field('b', '')], 'fields.errors.json.nameRequired'],
  ])('keeps the composer for %s, which JSON would lose, and says why', (_case, fields, messageKey) => {
    const {result} = renderHook(() => useAuthorizationDetailTypeForm());

    act(() => result.current.setFields(fields));
    act(() => result.current.setSchemaMode('json'));
    expect(result.current.schemaMode).toBe('builder');
    expect(result.current.revealCount, 'the first problem is selected').toBe(1);
    expect(result.current.fieldIssues.map((issue) => issue.messageKey)).toContain(messageKey);

    act(() => result.current.setFields([field('a', 'amount'), field('b', 'payee')]));
    expect(result.current.fieldIssues).toEqual([]);
    act(() => result.current.setSchemaMode('json'));
    expect(result.current.schemaMode).toBe('json');
  });

  it('says a problem is about switching to JSON only until the fields change', () => {
    const {result} = renderHook(() => useAuthorizationDetailTypeForm());

    act(() => result.current.setFields([field('a', 'amount'), field('b', 'amount')]));
    act(() => result.current.setSchemaMode('json'));
    act(() => result.current.setFields([field('a', 'amount'), field('b', 'amount'), field('c', 'payee')]));
    expect(result.current.fieldIssues.map((issue) => issue.messageKey)).toContain('fields.errors.nameDuplicate');
  });

  it('logs only the status and error code of a failed request', () => {
    const err = Object.assign(new Error('bad'), {
      config: {data: '{"token":"secret"}', headers: {Authorization: 'Bearer admin'}},
      response: {status: 400, data: {code: 'RAR-1008', message: 'raw'}},
    });

    expect(errorLogDetails(err)).toEqual({status: 400, code: 'RAR-1008'});
    expect(errorLogDetails(new Error('Network Error'))).toEqual({status: undefined, code: undefined});
  });
});
