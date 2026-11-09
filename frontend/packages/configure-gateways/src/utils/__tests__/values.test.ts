// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import {gatewayColor, GATEWAY_COLORS} from '../../constants/gateway-colors';
import type {Gateway} from '../../models/gateway';
import {errorReasonOf} from '../apiError';
import {aboutOwner, aboutValues, type MissingField} from '../missingValues';
import runnersOf from '../runners';
import {parseValueName} from '../valueLabels';
import valueRowsOf, {withoutHiddenResources} from '../valueRows';

describe('valueRowsOf', () => {
  it('lists every name of a kind the target holds or the version uses, unset first, then by name', () => {
    const rows = valueRowsOf(
      'variable',
      [
        {name: 'B_SET', value: 'b'},
        {name: 'A_SET', value: 'a'},
      ] as never,
      [
        {name: 'Z_UNSET', kind: 'variable', resourceType: 'application', resourceId: 'a1'},
        {name: 'B_SET', kind: 'variable', resourceType: 'application', resourceId: 'a1'},
        {name: 'SECRET', kind: 'secret', resourceType: 'application', resourceId: 'a1'},
      ],
    );

    expect(rows.map((row) => row.name)).toEqual(['Z_UNSET', 'A_SET', 'B_SET']);
    expect(rows[2].references).toHaveLength(1);
    expect(rows[0].stored).toBeUndefined();
  });
});

describe('valueRowsOf with the current configuration', () => {
  const stored = [{name: 'APPLICATION__AB__CLIENT_ID', value: 'x'}] as never;
  const current = [
    {
      name: 'APPLICATION__AB__CLIENT_ID',
      kind: 'variable' as const,
      resourceType: 'application',
      resourceName: 'Orders',
    },
    {name: 'APPLICATION__AB__CLIENT_ID', kind: 'secret' as const, resourceType: 'application', resourceName: 'Other'},
  ];

  it('says which resource a value belongs to when the running version does not refer to it', () => {
    const [row] = valueRowsOf('variable', stored, [], current);

    expect(row.references).toEqual([]);
    expect(row.current.map((reference) => reference.resourceName)).toEqual(['Orders']);
  });

  it('leaves a value the running version refers to with the places it refers to it', () => {
    const running = [
      {
        name: 'APPLICATION__AB__CLIENT_ID',
        kind: 'variable' as const,
        resourceType: 'application',
        resourceName: 'Live',
      },
    ];
    const [row] = valueRowsOf('variable', stored, running, current);

    expect(row.references.map((reference) => reference.resourceName)).toEqual(['Live']);
    expect(row.current).toEqual([]);
  });

  it('does not add a row for a name only the current configuration refers to', () => {
    expect(valueRowsOf('variable', [], [], current)).toEqual([]);
  });
});

describe('parseValueName', () => {
  it('reads the resource type and field back from a name the server derived', () => {
    expect(parseValueName('APPLICATION__019000000000700080000000000000F1__CLIENT_ID')).toEqual({
      resourceType: 'application',
      field: 'CLIENT_ID',
    });
    expect(parseValueName('RESOURCE_SERVER__01900000000070008000000000000020__IDENTIFIER')).toEqual({
      resourceType: 'resource_server',
      field: 'IDENTIFIER',
    });
    expect(parseValueName('USER__H0123456789ABCDEF0123456789ABCDEF__PASSWORD')).toEqual({
      resourceType: 'user',
      field: 'PASSWORD',
    });
  });

  it('does not read a name someone chose', () => {
    expect(parseValueName('APPLICATION_MY_APP_CLIENT_ID')).toBeNull();
    expect(parseValueName('DB_PASSWORD')).toBeNull();
    expect(parseValueName('APPLICATION__NOT_AN_ID__CLIENT_ID')).toBeNull();
  });
});

describe('runnersOf', () => {
  const dev: Gateway = {id: 'gw-dev', name: 'dev', baseUrl: ''};
  const prod: Gateway = {id: 'gw-prod', name: 'prod', baseUrl: ''};
  const idle: Gateway = {id: 'gw-idle', name: 'idle', baseUrl: ''};

  it('names the targets that run each version, and those that hold it in part', () => {
    const {running, partly} = runnersOf([dev, prod, idle], {
      'gw-dev': {gatewayId: 'gw-dev', appliedVersion: 'h2'},
      'gw-prod': {gatewayId: 'gw-prod', appliedVersion: 'h2', complete: false},
    });

    expect(running).toEqual({h2: ['dev', 'prod']});
    expect(partly).toEqual({h2: ['prod']});
  });

  it('names a target as asked', () => {
    expect(
      runnersOf([dev], {'gw-dev': {gatewayId: 'gw-dev', appliedVersion: 'h1'}}, () => 'Development').running,
    ).toEqual({h1: ['Development']});
  });
});

describe('gatewayColor', () => {
  it('marks gateways by their place, repeating after the last, for any place', () => {
    expect(gatewayColor(0)).toBe(GATEWAY_COLORS[0]);
    expect(gatewayColor(GATEWAY_COLORS.length)).toBe(GATEWAY_COLORS[0]);
    expect(gatewayColor(-1)).toBe(GATEWAY_COLORS[GATEWAY_COLORS.length - 1]);
  });
});

describe('errorReasonOf', () => {
  it("reads why a call failed from the answer's text, description, message, error or code, else the error", () => {
    const failed = (data: unknown): Error => Object.assign(new Error('Request failed'), {response: {data}});
    expect(errorReasonOf(failed(' plain text '))).toBe('plain text');
    expect(errorReasonOf(failed({description: 'Described', message: 'Message'}))).toBe('Described');
    expect(errorReasonOf(failed({message: 'Message'}))).toBe('Message');
    expect(errorReasonOf(failed({error: 'invalid_request'}))).toBe('invalid_request');
    expect(errorReasonOf(failed({code: 'ORG-4001'}))).toBe('ORG-4001');
    expect(errorReasonOf(failed({}))).toBe('Request failed');
    expect(errorReasonOf('not an error')).toBeUndefined();
  });
});

describe('aboutValues and aboutOwner', () => {
  const secret: MissingField = {
    kind: 'secret',
    name: 'AGENT_SECRET',
    list: false,
    references: [
      {name: 'AGENT_SECRET', kind: 'secret', resourceType: 'agent', resourceId: 'ag', field: 'timeoutSeconds'},
    ],
  };

  it('says a refusal is about values when it names one, the field it is set in, or speaks of values', () => {
    expect(aboutValues({message: 'AGENT_SECRET is empty'}, [secret])).toBe(true);
    expect(aboutValues({description: 'timeoutSeconds is too low'}, [])).toBe(false);
    expect(aboutValues({description: 'timeoutSeconds is too low'}, [secret])).toBe(true);
    expect(aboutValues({message: 'The redirect URL is not allowed'}, [])).toBe(true);
    expect(aboutValues({code: 'AGT-1039', message: 'Owner not found'}, [secret])).toBe(false);
    expect(aboutValues({}, [secret])).toBe(false);
    expect(aboutValues(undefined, [secret])).toBe(false);
  });

  it("says a refusal is about the resource's owner when it names the owner", () => {
    expect(aboutOwner({code: 'AGT-1039', message: 'Owner not found'})).toBe(true);
    expect(aboutOwner({message: 'Missing executor'})).toBe(false);
    expect(aboutOwner(undefined)).toBe(false);
  });
});

describe('withoutHiddenResources', () => {
  const ref = (name: string, resourceId: string) => ({
    name,
    kind: 'variable' as const,
    resourceType: 'application',
    resourceId,
  });
  const isHidden = (resourceType: string, id: string | undefined): boolean =>
    resourceType === 'application' && id === 'app-hidden';

  it('drops a value only hidden resources use, and keeps the others without the hidden uses', () => {
    const rows = valueRowsOf(
      'variable',
      [
        {name: 'HIDDEN_ONLY', value: 'h'},
        {name: 'SHARED', value: 's'},
        {name: 'UNUSED', value: 'u'},
      ],
      [ref('HIDDEN_ONLY', 'app-hidden'), ref('SHARED', 'app-hidden'), ref('SHARED', 'app-shown')],
    );

    const shown = withoutHiddenResources(rows, isHidden);

    expect(shown.map((row) => row.name)).toEqual(['SHARED', 'UNUSED']);
    expect(shown[0].references.map((reference) => reference.resourceId)).toEqual(['app-shown']);
    expect(shown[1].references).toEqual([]);
  });

  it('drops a value only a hidden resource of the current configuration uses', () => {
    const rows = valueRowsOf('variable', [{name: 'HIDDEN_NOW', value: 'h'}], [], [ref('HIDDEN_NOW', 'app-hidden')]);

    expect(withoutHiddenResources(rows, isHidden)).toEqual([]);
  });

  it('keeps every row when nothing is hidden', () => {
    const rows = valueRowsOf('variable', [{name: 'A', value: 'a'}], [ref('A', 'app-hidden')]);

    expect(withoutHiddenResources(rows, () => false)).toEqual(rows);
  });
});
