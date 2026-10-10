// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import {fieldLabel} from '../labels';
import {descriptionOf, layoutOf, layoutOfType, nameOf} from '../layouts';
import {rootOf, valueAt} from '../paths';
import {fromListText, isEmptyValue, parseListValue, parseReference, toListText} from '../values';

describe('fieldLabel', () => {
  it.each([
    ['redirectUris', 'Redirect URIs'],
    ['token_endpoint_auth_method', 'Token endpoint auth method'],
    ['clientId', 'Client ID'],
    ['isRegistrationFlowEnabled', 'Is registration flow enabled'],
    ['oauth2', 'OAuth2'],
  ])('names %s as %s', (field, label) => {
    expect(fieldLabel(field)).toBe(label);
  });
});

describe('values', () => {
  it('reads variable and secret references', () => {
    expect(parseReference('var:A_B')).toEqual({kind: 'variable', name: 'A_B'});
    expect(parseReference('sec:S')).toEqual({kind: 'secret', name: 'S'});
    expect(parseReference('var:')).toBeUndefined();
    expect(parseReference('plain')).toBeUndefined();
    expect(parseReference(3)).toBeUndefined();
  });

  it('reads and writes list values', () => {
    expect(parseListValue('["a","b"]')).toEqual(['a', 'b']);
    expect(parseListValue('[1]')).toBeUndefined();
    expect(parseListValue('[oops')).toBeUndefined();
    expect(parseListValue('a')).toBeUndefined();
    expect(toListText('["a","b"]')).toBe('a\nb');
    expect(toListText('a')).toBe('a');
    expect(fromListText(' a \n\nb\n')).toBe('["a","b"]');
  });

  it('says which values hold nothing', () => {
    expect([null, undefined, ' ', [], {a: '', b: []}].every(isEmptyValue)).toBe(true);
    expect([0, false, 'x', ['x'], {a: 'x'}].some(isEmptyValue)).toBe(false);
  });
});

describe('layouts', () => {
  it('finds a section by its path and by the types it shows', () => {
    expect(layoutOf('design')?.types).toEqual(['theme', 'layout']);
    expect(layoutOf('nowhere')).toBeUndefined();
    expect(layoutOfType('organization_unit')?.segment).toBe('organization-units');
    expect(layoutOfType('nothing')).toBeUndefined();
  });

  it('names and describes a resource', () => {
    expect(nameOf({name: 'Orders'}, 'id-1')).toBe('Orders');
    expect(nameOf({displayName: 'Dark'}, 'id-1')).toBe('Dark');
    expect(nameOf({}, 'id-1')).toBe('id-1');
    expect(nameOf('not a resource', 'id-1')).toBe('id-1');
    expect(nameOf({attributes: {email: 'a@b.test'}}, 'u-1', layoutOfType('user'))).toBe('a@b.test');
    expect(nameOf({attributes: {}}, 'u-1', layoutOfType('user'))).toBe('u-1');
    expect(descriptionOf({description: 'Order service'})).toBe('Order service');
    expect(descriptionOf({description: ''})).toBeUndefined();
  });
});

describe('paths', () => {
  const application = {
    url: 'https://orders.test',
    theme: {shape: {borderRadius: 8}},
    inboundAuthConfig: [
      {type: 'saml', config: {entityId: 'x'}},
      {type: 'oauth2', config: {clientId: 'orders', token: {idToken: {validityPeriod: 300}}}},
    ],
  };

  it('reads a field, a nested field and the OAuth 2 settings', () => {
    expect(valueAt(application, 'url')).toBe('https://orders.test');
    expect(valueAt(application, 'theme.shape.borderRadius')).toBe(8);
    expect(valueAt(application, 'oauth.clientId')).toBe('orders');
    expect(valueAt(application, 'oauth.token.idToken.validityPeriod')).toBe(300);
  });

  it('reads nothing where there is nothing', () => {
    expect(valueAt(application, 'theme.colors.primary')).toBeUndefined();
    expect(valueAt({}, 'oauth.clientId')).toBeUndefined();
    expect(valueAt('not a resource', 'url')).toBeUndefined();
  });

  it('names the field a path starts in', () => {
    expect(rootOf('oauth.redirectUris')).toBe('inboundAuthConfig');
    expect(rootOf('theme.shape')).toBe('theme');
  });
});
