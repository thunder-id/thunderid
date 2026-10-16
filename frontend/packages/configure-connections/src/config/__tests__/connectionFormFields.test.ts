// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import {ConnectionTypes} from '../../models/connection';
import {fieldsForMode} from '../connectionFormFields';

function fieldNames(type: (typeof ConnectionTypes)[keyof typeof ConnectionTypes], mode: 'create' | 'edit'): string[] {
  return fieldsForMode(type, mode).map((field) => field.name);
}

describe('fieldsForMode', () => {
  it('hides the redirect URI and scopes on create for Google/GitHub, showing them on edit', () => {
    expect(fieldNames(ConnectionTypes.GOOGLE, 'create')).toEqual(['name', 'clientId', 'clientSecret']);
    expect(fieldNames(ConnectionTypes.GOOGLE, 'edit')).toEqual([
      'name',
      'clientId',
      'clientSecret',
      'redirectUri',
      'scopes',
      'prompt',
    ]);
  });

  it('shows only the required fields for OIDC on create, all fields on edit', () => {
    expect(fieldNames(ConnectionTypes.OIDC, 'create')).toEqual([
      'name',
      'clientId',
      'clientSecret',
      'authorizationEndpoint',
      'tokenEndpoint',
    ]);
    expect(fieldNames(ConnectionTypes.OIDC, 'edit')).toEqual([
      'name',
      'clientId',
      'clientSecret',
      'authorizationEndpoint',
      'tokenEndpoint',
      'issuer',
      'userInfoEndpoint',
      'jwksEndpoint',
      'redirectUri',
      'scopes',
      'prompt',
      'tokenExchangeEnabled',
      'trustedTokenAudience',
    ]);
  });

  it('shows only the required fields for OAuth 2 on create, all fields on edit', () => {
    expect(fieldNames(ConnectionTypes.OAUTH, 'create')).toEqual([
      'name',
      'clientId',
      'clientSecret',
      'authorizationEndpoint',
      'tokenEndpoint',
      'userInfoEndpoint',
    ]);
    expect(fieldNames(ConnectionTypes.OAUTH, 'edit')).toEqual([
      'name',
      'clientId',
      'clientSecret',
      'authorizationEndpoint',
      'tokenEndpoint',
      'userInfoEndpoint',
      'redirectUri',
      'scopes',
      'prompt',
    ]);
  });

  it('shows SMS gateway authentication fields only on edit', () => {
    expect(fieldNames(ConnectionTypes.TWILIO, 'create')).toEqual(fieldNames(ConnectionTypes.TWILIO, 'edit'));
    expect(fieldNames(ConnectionTypes.VONAGE, 'create')).toEqual(fieldNames(ConnectionTypes.VONAGE, 'edit'));
    expect(fieldNames(ConnectionTypes.SMS_GATEWAY, 'create')).toEqual([
      'name',
      'url',
      'httpMethod',
      'contentType',
      'smsHttpHeaders',
    ]);
    expect(fieldNames(ConnectionTypes.SMS_GATEWAY, 'edit')).toEqual([
      ...fieldNames(ConnectionTypes.SMS_GATEWAY, 'create'),
      'authenticationScheme',
      'bearerToken',
      'httpHeaders',
      'basicUsername',
      'basicPassword',
    ]);
  });

  it('offers the SMS gateway method and content type as selects with the API-accepted values', () => {
    const fields = fieldsForMode(ConnectionTypes.SMS_GATEWAY, 'create');
    const httpMethod = fields.find((field) => field.name === 'httpMethod');
    const contentType = fields.find((field) => field.name === 'contentType');

    expect(httpMethod).toMatchObject({kind: 'select', defaultValue: 'POST'});
    expect(httpMethod?.options?.map((option) => option.value)).toEqual(['POST', 'GET']);
    expect(contentType).toMatchObject({kind: 'select', defaultValue: 'JSON'});
    expect(contentType?.options?.map((option) => option.value)).toEqual(['JSON', 'FORM']);
  });

  it('marks only name and the gateway URL required, matching the API contract', () => {
    const required: string[] = fieldsForMode(ConnectionTypes.SMS_GATEWAY, 'create')
      .filter((field) => field.required)
      .map((field) => field.name);

    expect(required).toEqual(['name', 'url']);
  });

  it('keeps general headers separate from authentication header fields', () => {
    const headers = fieldsForMode(ConnectionTypes.SMS_GATEWAY, 'edit').find((field) => field.name === 'httpHeaders');

    expect(headers).toMatchObject({kind: 'key-value', addLabelKey: 'connections:form.fields.httpHeaders.add'});
    expect(headers?.required).toBeUndefined();
  });

  it('requires only the AuthZEN PDP single-evaluation endpoint on creation', () => {
    expect(fieldNames(ConnectionTypes.AUTHZEN_PDP, 'create')).toEqual(['name', 'endpoint', 'batchEndpoint']);
    expect(fieldNames(ConnectionTypes.AUTHZEN_PDP, 'edit')).toEqual([
      'name',
      'endpoint',
      'batchEndpoint',
      'timeoutMs',
      'retryCount',
      'authenticationScheme',
      'bearerToken',
      'httpHeaders',
      'basicUsername',
      'basicPassword',
    ]);
    const requiredFields = fieldsForMode(ConnectionTypes.AUTHZEN_PDP, 'create')
      .filter((field) => field.required)
      .map((field) => field.name);
    const timeoutField = fieldsForMode(ConnectionTypes.AUTHZEN_PDP, 'edit').find((field) => field.name === 'timeoutMs');

    expect(requiredFields).toEqual(['name', 'endpoint']);
    expect(timeoutField).toMatchObject({defaultValue: '1000', placeholder: '1000'});
    expect(
      fieldsForMode(ConnectionTypes.AUTHZEN_PDP, 'edit').filter((field) => field.requiresHttpsWhenAuthenticated),
    ).toEqual(
      expect.arrayContaining([
        expect.objectContaining({name: 'endpoint', showErrorImmediately: true}),
        expect.objectContaining({name: 'batchEndpoint', showErrorImmediately: true}),
      ]),
    );
  });

  it('edits AuthZEN PDP additional headers as key-value rows', () => {
    const headers = fieldsForMode(ConnectionTypes.AUTHZEN_PDP, 'edit').find((field) => field.name === 'httpHeaders');

    expect(headers).toMatchObject({kind: 'key-value', addLabelKey: 'connections:form.fields.httpHeaders.add'});
  });

  it('asks only for the name and URL when creating an HTTP endpoint, and edits the rest', () => {
    expect(fieldNames(ConnectionTypes.HTTP, 'create')).toEqual(['name', 'url']);
    expect(fieldNames(ConnectionTypes.HTTP, 'edit')).toEqual(['name', 'description', 'url', 'timeoutMs', 'headers']);
    expect(fieldsForMode(ConnectionTypes.HTTP, 'create').find((field) => field.name === 'url')).toMatchObject({
      kind: 'url',
      required: true,
      requiresHttpsWhenAuthenticated: true,
      rejectsLoopbackHttp: true,
      rejectsUserInfo: true,
    });
    expect(fieldsForMode(ConnectionTypes.HTTP, 'edit').find((field) => field.name === 'headers')).toMatchObject({
      kind: 'key-value',
      keyValueFormat: 'pairs',
      uniqueNames: true,
    });
  });

  it('bounds the HTTP endpoint timeout to 1 to 20000 with a 10000 default', () => {
    const timeout = fieldsForMode(ConnectionTypes.HTTP, 'edit').find((field) => field.name === 'timeoutMs');

    expect(timeout).toMatchObject({kind: 'number', defaultValue: '10000'});
    for (const valid of ['1', '999', '10000', '19999', '20000']) {
      expect(timeout?.pattern?.test(valid)).toBe(true);
    }
    for (const invalid of ['0', '20001', '30000', '-1', '1.5', '010']) {
      expect(timeout?.pattern?.test(invalid)).toBe(false);
    }
  });
});
