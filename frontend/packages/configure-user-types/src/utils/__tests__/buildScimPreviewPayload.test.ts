// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import buildScimPreviewPayload from '../buildScimPreviewPayload';

describe('buildScimPreviewPayload', () => {
  it('places a top-level target directly on the payload', () => {
    expect(buildScimPreviewPayload({loginId: 'userName'}, {}, ['loginId'], 'Employee')).toEqual({
      userName: '<loginId>',
    });
  });

  it('nests name sub-attributes under a single name object', () => {
    expect(
      buildScimPreviewPayload(
        {firstName: 'name.givenName', lastName: 'name.familyName'},
        {},
        ['firstName', 'lastName'],
        'Employee',
      ),
    ).toEqual({
      name: {givenName: '<firstName>', familyName: '<lastName>'},
    });
  });

  it('wraps a single multi-valued target as a one-element array', () => {
    expect(buildScimPreviewPayload({email: 'emails'}, {}, ['email'], 'Employee')).toEqual({
      emails: [{value: '<email>', primary: true}],
    });
  });

  it('collects multiple properties mapped to the same multi-valued target into one array', () => {
    const meta = {
      workEmail: {type: 'work', primary: true},
      personalEmail: {type: 'home', primary: false},
    };
    expect(
      buildScimPreviewPayload(
        {workEmail: 'emails', personalEmail: 'emails'},
        meta,
        ['workEmail', 'personalEmail'],
        'Employee',
      ),
    ).toEqual({
      emails: [
        {value: '<workEmail>', type: 'work', primary: true},
        {value: '<personalEmail>', type: 'home'},
      ],
    });
  });

  it('marks a lone entry of a multi-valued target as primary', () => {
    expect(buildScimPreviewPayload({email: 'emails'}, {}, ['email'], 'Employee')).toEqual({
      emails: [{value: '<email>', primary: true}],
    });
  });

  it('omits type/primary from entries sharing a target when their meta is not set', () => {
    expect(buildScimPreviewPayload({a: 'emails', b: 'emails'}, {}, ['a', 'b'], 'Employee')).toEqual({
      emails: [{value: '<a>'}, {value: '<b>'}],
    });
  });

  it('nests address sub-attributes under a single addresses array entry', () => {
    expect(
      buildScimPreviewPayload(
        {city: 'addresses.locality', zip: 'addresses.postalCode'},
        {},
        ['city', 'zip'],
        'Employee',
      ),
    ).toEqual({
      addresses: [{locality: '<city>', postalCode: '<zip>'}],
    });
  });

  it('nests enterprise fields under the enterprise extension URN', () => {
    const payload = buildScimPreviewPayload(
      {empNo: 'employeeNumber', mgr: 'manager'},
      {},
      ['empNo', 'mgr'],
      'Employee',
    );
    expect(payload['urn:ietf:params:scim:schemas:extension:enterprise:2.0:User']).toEqual({
      employeeNumber: '<empNo>',
      manager: {value: '<mgr>'},
    });
  });

  it('ignores unmapped (empty string) entries in the mapping itself', () => {
    expect(buildScimPreviewPayload({unused: ''}, {}, [], 'Employee')).toEqual({});
  });

  it('places unmapped properties under the user type custom extension URN', () => {
    const payload = buildScimPreviewPayload(
      {loginId: 'userName'},
      {},
      ['loginId', 'department', 'shirtSize'],
      'Employee',
    );
    expect(payload).toEqual({
      userName: '<loginId>',
      'urn:thunderid:params:scim:schemas:employee:2.0:User': {
        department: '<department>',
        shirtSize: '<shirtSize>',
      },
    });
  });

  it('lowercases the user type name in the custom extension URN', () => {
    const payload = buildScimPreviewPayload({}, {}, ['shirtSize'], 'Contractor');
    expect(Object.keys(payload)).toEqual(['urn:thunderid:params:scim:schemas:contractor:2.0:User']);
  });

  it('omits the custom extension entirely when every property is mapped', () => {
    expect(buildScimPreviewPayload({loginId: 'userName'}, {}, ['loginId'], 'Employee')).not.toHaveProperty(
      'urn:thunderid:params:scim:schemas:employee:2.0:User',
    );
  });
});
