// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import type {FlowNode} from '../../models/responses';
import {ExecutionTypes} from '../../models/steps';
import resolveConnectionPlaceholders, {resolveConnectionProperty} from '../resolveConnectionPlaceholders';

describe('resolveConnectionProperty', () => {
  const google = {id: 'google-1', type: 'GOOGLE'};

  it('assigns the only connection of the executor type to an IDP placeholder', () => {
    expect(
      resolveConnectionProperty(ExecutionTypes.GoogleFederation, {idpId: '{{IDP_ID}}'}, {identityProviders: [google]}),
    ).toEqual({idpId: 'google-1'});
  });

  it('assigns a connection when the IDP property is empty or missing', () => {
    expect(
      resolveConnectionProperty(ExecutionTypes.GoogleFederation, {idpId: ''}, {identityProviders: [google]}),
    ).toEqual({idpId: 'google-1'});
    expect(
      resolveConnectionProperty(ExecutionTypes.GoogleFederation, undefined, {identityProviders: [google]}),
    ).toEqual({idpId: 'google-1'});
  });

  it('ignores connections of other types', () => {
    expect(
      resolveConnectionProperty(
        ExecutionTypes.GoogleFederation,
        {idpId: '{{IDP_ID}}'},
        {identityProviders: [google, {id: 'github-1', type: 'GITHUB'}]},
      ),
    ).toEqual({idpId: 'google-1'});
  });

  it('assigns nothing when several connections of the executor type exist', () => {
    expect(
      resolveConnectionProperty(
        ExecutionTypes.GoogleFederation,
        {idpId: '{{IDP_ID}}'},
        {identityProviders: [google, {id: 'google-2', type: 'GOOGLE'}]},
      ),
    ).toBeNull();
  });

  it('assigns nothing when no connection of the executor type exists or the list could not be read', () => {
    expect(
      resolveConnectionProperty(ExecutionTypes.GoogleFederation, {idpId: '{{IDP_ID}}'}, {identityProviders: []}),
    ).toBeNull();
    expect(resolveConnectionProperty(ExecutionTypes.GoogleFederation, {idpId: '{{IDP_ID}}'}, {})).toBeNull();
  });

  it('keeps an IDP that is already set', () => {
    expect(
      resolveConnectionProperty(ExecutionTypes.GoogleFederation, {idpId: 'chosen'}, {identityProviders: [google]}),
    ).toBeNull();
  });

  it('assigns the only SMS provider to a sender placeholder, and nothing when there are several', () => {
    expect(
      resolveConnectionProperty(
        ExecutionTypes.SMSExecutor,
        {senderId: '{{SENDER_ID}}'},
        {smsProviders: [{id: 'sms-1'}]},
      ),
    ).toEqual({senderId: 'sms-1'});
    expect(
      resolveConnectionProperty(
        ExecutionTypes.SMSExecutor,
        {senderId: '{{SENDER_ID}}'},
        {smsProviders: [{id: 'sms-1'}, {id: 'sms-2'}]},
      ),
    ).toBeNull();
  });

  it('assigns the first email provider even when there are several', () => {
    expect(
      resolveConnectionProperty(
        ExecutionTypes.EmailExecutor,
        {senderId: '{{SENDER_ID}}'},
        {emailProviders: [{id: 'email-1'}, {id: 'email-2'}]},
      ),
    ).toEqual({senderId: 'email-1'});
  });

  it('keeps a sender that is already set, and assigns nothing to an email step when no provider exists', () => {
    expect(
      resolveConnectionProperty(
        ExecutionTypes.EmailExecutor,
        {senderId: 'chosen'},
        {emailProviders: [{id: 'email-1'}]},
      ),
    ).toBeNull();
    expect(
      resolveConnectionProperty(ExecutionTypes.SMSExecutor, {senderId: 'chosen'}, {smsProviders: [{id: 'sms-1'}]}),
    ).toBeNull();
    expect(
      resolveConnectionProperty(ExecutionTypes.EmailExecutor, {senderId: '{{SENDER_ID}}'}, {emailProviders: []}),
    ).toBeNull();
    expect(resolveConnectionProperty(ExecutionTypes.EmailExecutor, undefined, {})).toBeNull();
  });

  it('assigns nothing to an executor that takes no connection', () => {
    expect(resolveConnectionProperty('ProvisioningExecutor', undefined, {identityProviders: [google]})).toBeNull();
  });
});

describe('resolveConnectionPlaceholders', () => {
  it('sets the connection of each resolvable node and leaves the rest unchanged', () => {
    const nodes: FlowNode[] = [
      {
        id: 'google_auth',
        type: 'TASK_EXECUTION',
        properties: {idpId: '{{IDP_ID}}', allowAuthenticationWithoutLocalUser: true},
        executor: {name: ExecutionTypes.GoogleFederation},
      },
      {
        id: 'github_auth',
        type: 'TASK_EXECUTION',
        properties: {idpId: '{{IDP_ID}}'},
        executor: {name: ExecutionTypes.GithubFederation},
      },
      {id: 'provisioning', type: 'TASK_EXECUTION', executor: {name: 'ProvisioningExecutor'}},
    ] as FlowNode[];

    const resolved = resolveConnectionPlaceholders(nodes, {identityProviders: [{id: 'google-1', type: 'GOOGLE'}]});

    expect(resolved[0].properties).toEqual({idpId: 'google-1', allowAuthenticationWithoutLocalUser: true});
    expect(resolved[1]).toBe(nodes[1]);
    expect(resolved[2]).toBe(nodes[2]);
    expect(nodes[0].properties?.['idpId']).toBe('{{IDP_ID}}');
  });
});
